package proxyfailover

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"sort"
	"sync"
	"time"
)

const interval = 30 * time.Second
const freshness = 45 * time.Second

var ErrConflict = errors.New("proxy or policy changed; refresh and try again")
var ErrInvalid = errors.New("select an Adobe OAuth account with an active proxy and 1–8 distinct active backups")

type Policy struct {
	Enabled        bool    `json:"enabled"`
	PrimaryProxyID int64   `json:"primary_proxy_id"`
	BackupProxyIDs []int64 `json:"backup_proxy_ids"`
	Revision       int64   `json:"revision"`
	CurrentProxyID int64   `json:"current_proxy_id"`
}
type health struct {
	ProxyID     int64     `json:"proxy_id"`
	Status      string    `json:"status"`
	Failures    int       `json:"failures"`
	LatencyMS   int64     `json:"latency_ms"`
	CheckedAt   time.Time `json:"checked_at"`
	Message     string    `json:"message"`
	fingerprint string
}
type Event struct {
	FromProxyID int64     `json:"from_proxy_id"`
	ToProxyID   int64     `json:"to_proxy_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type View struct {
	Policy
	State  string   `json:"state"`
	Health []health `json:"health"`
	Events []Event  `json:"events"`
}
type proxy struct {
	ID                                         int64
	Protocol, Host, Username, Password, Status string
	Port                                       int
	ExpiresAt                                  sql.NullTime
	UpdatedAt                                  time.Time
	Deleted                                    bool
}

func (p proxy) available(now time.Time) bool {
	return !p.Deleted && p.Status == "active" && (!p.ExpiresAt.Valid || p.ExpiresAt.Time.After(now))
}
func (p proxy) fingerprint() string {
	// Persist only a digest, never proxy credentials, in health records.
	raw, _ := json.Marshal(p)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

type Manager struct {
	db    *sql.DB
	probe func(context.Context, proxy) health
}

func NewManager(db *sql.DB) *Manager { return &Manager{db: db, probe: probeAdobe} }

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadProxies(ctx context.Context, q queryer) (map[int64]proxy, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, protocol, host, port, COALESCE(username,''), COALESCE(password,''), status, expires_at, updated_at, deleted_at IS NOT NULL FROM proxies
 WHERE id IN (SELECT primary_proxy_id FROM account_proxy_failover UNION SELECT jsonb_array_elements_text(backup_proxy_ids)::bigint FROM account_proxy_failover)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]proxy)
	for rows.Next() {
		var p proxy
		if err := rows.Scan(&p.ID, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.ExpiresAt, &p.UpdatedAt, &p.Deleted); err != nil {
			return nil, err
		}
		result[p.ID] = p
	}
	return result, rows.Err()
}
func loadHealth(ctx context.Context, q queryer) (map[int64]health, error) {
	rows, err := q.QueryContext(ctx, `SELECT proxy_id,fingerprint,status,failures,latency_ms,checked_at,message FROM proxy_failover_health`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]health)
	for rows.Next() {
		var h health
		if err := rows.Scan(&h.ProxyID, &h.fingerprint, &h.Status, &h.Failures, &h.LatencyMS, &h.CheckedAt, &h.Message); err != nil {
			return nil, err
		}
		result[h.ProxyID] = h
	}
	return result, rows.Err()
}
func (m *Manager) Get(ctx context.Context, id int64) (View, error) {
	v := View{Policy: Policy{BackupProxyIDs: []int64{}}, Health: []health{}, Events: []Event{}, State: "disabled"}
	var raw []byte
	var active bool
	err := m.db.QueryRowContext(ctx, `SELECT COALESCE(a.proxy_id,0),COALESCE(f.enabled,false),COALESCE(f.primary_proxy_id,a.proxy_id,0),COALESCE(f.backup_proxy_ids,'[]'::jsonb),COALESCE(f.revision,0),a.status='active' AND a.schedulable AND (a.expires_at IS NULL OR a.expires_at>NOW())
 FROM accounts a LEFT JOIN account_proxy_failover f ON f.account_id=a.id WHERE a.id=$1 AND a.deleted_at IS NULL AND a.platform='adobe' AND a.type='oauth'`, id).Scan(&v.CurrentProxyID, &v.Enabled, &v.PrimaryProxyID, &raw, &v.Revision, &active)
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal(raw, &v.BackupProxyIDs); err != nil {
		return v, err
	}
	hs, err := loadHealth(ctx, m.db)
	if err != nil {
		return v, err
	}
	ps, err := loadProxies(ctx, m.db)
	if err != nil {
		return v, err
	}
	now := time.Now()
	for _, pid := range append([]int64{v.PrimaryProxyID}, v.BackupProxyIDs...) {
		h, ok := hs[pid]
		p, exists := ps[pid]
		if !ok || !exists || h.fingerprint != p.fingerprint() || now.Sub(h.CheckedAt) > freshness {
			h = health{ProxyID: pid, Status: "unknown", Message: "awaiting_check"}
		}
		if pid > 0 {
			v.Health = append(v.Health, h)
		}
	}
	if v.Enabled {
		v.State = "checking"
		if !active || !slices.Contains(append([]int64{v.PrimaryProxyID}, v.BackupProxyIDs...), v.CurrentProxyID) {
			v.State = "paused"
		} else {
			for _, h := range v.Health {
				if h.ProxyID == v.CurrentProxyID {
					if h.Status == "healthy" {
						v.State = "healthy"
					} else if h.Failures >= 2 {
						v.State = "no_backup"
					}
				}
			}
		}
	}
	rows, err := m.db.QueryContext(ctx, `SELECT from_proxy_id,to_proxy_id,created_at FROM proxy_failover_events WHERE account_id=$1 ORDER BY id DESC LIMIT 10`, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.FromProxyID, &e.ToProxyID, &e.CreatedAt); err != nil {
			return v, err
		}
		v.Events = append(v.Events, e)
	}
	return v, rows.Err()
}

func (m *Manager) Save(ctx context.Context, id int64, p Policy) (View, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return View{}, err
	}
	defer tx.Rollback()
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(proxy_id,0) FROM accounts WHERE id=$1 AND deleted_at IS NULL AND platform='adobe' AND type='oauth' FOR UPDATE`, id).Scan(&current)
	if err != nil {
		return View{}, err
	}
	if current != p.CurrentProxyID {
		return View{}, ErrConflict
	}
	var rev int64
	err = tx.QueryRowContext(ctx, `SELECT revision FROM account_proxy_failover WHERE account_id=$1 FOR UPDATE`, id).Scan(&rev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return View{}, err
	}
	if rev != p.Revision {
		return View{}, ErrConflict
	}
	if p.Enabled {
		ids := append([]int64{p.PrimaryProxyID}, p.BackupProxyIDs...)
		if len(p.BackupProxyIDs) < 1 || len(p.BackupProxyIDs) > 8 || !slices.Contains(ids, current) || (rev == 0 && p.PrimaryProxyID != current) {
			return View{}, ErrInvalid
		}
		seen := map[int64]bool{}
		for _, pid := range ids {
			if pid <= 0 || seen[pid] {
				return View{}, ErrInvalid
			}
			seen[pid] = true
			var valid bool
			err = tx.QueryRowContext(ctx, `SELECT status='active' AND (expires_at IS NULL OR expires_at>NOW()) FROM proxies WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, pid).Scan(&valid)
			if errors.Is(err, sql.ErrNoRows) || !valid {
				return View{}, ErrInvalid
			}
			if err != nil {
				return View{}, err
			}
		}
	}
	if p.BackupProxyIDs == nil {
		p.BackupProxyIDs = []int64{}
	}
	raw, _ := json.Marshal(p.BackupProxyIDs)
	_, err = tx.ExecContext(ctx, `INSERT INTO account_proxy_failover(account_id,enabled,primary_proxy_id,backup_proxy_ids) VALUES($1,$2,$3,$4)
 ON CONFLICT(account_id) DO UPDATE SET enabled=EXCLUDED.enabled,primary_proxy_id=EXCLUDED.primary_proxy_id,backup_proxy_ids=EXCLUDED.backup_proxy_ids,revision=account_proxy_failover.revision+1,updated_at=NOW()`, id, p.Enabled, p.PrimaryProxyID, string(raw))
	if err != nil {
		return View{}, err
	}
	if err = tx.Commit(); err != nil {
		return View{}, err
	}
	return m.Get(ctx, id)
}

func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		round, cancel := context.WithTimeout(ctx, 25*time.Second)
		err := m.check(round)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("[proxy-failover] check failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type accountPolicy struct {
	Policy
	ID int64
}

func (m *Manager) check(ctx context.Context) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked bool
	// Transaction-scoped leadership prevents duplicate checks/switches across replicas.
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(241092601)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT f.account_id,f.primary_proxy_id,f.backup_proxy_ids,f.revision,COALESCE(a.proxy_id,0) FROM account_proxy_failover f JOIN accounts a ON a.id=f.account_id
 WHERE f.enabled AND a.deleted_at IS NULL AND a.platform='adobe' AND a.type='oauth' AND a.status='active' AND a.schedulable AND (a.expires_at IS NULL OR a.expires_at>NOW()) ORDER BY f.account_id`)
	if err != nil {
		return err
	}
	policies := []accountPolicy{}
	needed := map[int64]bool{}
	for rows.Next() {
		var a accountPolicy
		var raw []byte
		if err = rows.Scan(&a.ID, &a.PrimaryProxyID, &raw, &a.Revision, &a.CurrentProxyID); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(raw, &a.BackupProxyIDs); err != nil {
			rows.Close()
			return err
		}
		ids := append([]int64{a.PrimaryProxyID}, a.BackupProxyIDs...)
		// Manual changes outside the configured pool pause automation.
		if !slices.Contains(ids, a.CurrentProxyID) {
			continue
		}
		policies = append(policies, a)
		for _, id := range ids {
			needed[id] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(policies) == 0 {
		return tx.Commit()
	}
	ps, err := loadProxies(ctx, tx)
	if err != nil {
		return err
	}
	hs, err := loadHealth(ctx, tx)
	if err != nil {
		return err
	}
	pending := []proxy{}
	for id := range needed {
		p, exists := ps[id]
		if !exists {
			continue
		}
		h := hs[id]
		if h.fingerprint != p.fingerprint() || time.Since(h.CheckedAt) >= 20*time.Second {
			pending = append(pending, p)
		}
	}
	sort.Slice(pending, func(i, j int) bool { return hs[pending[i].ID].CheckedAt.Before(hs[pending[j].ID].CheckedAt) })
	// Bound load and leave time for the final transaction even with many slow proxies.
	probeCtx, cancel := context.WithTimeout(ctx, 19*time.Second)
	defer cancel()
	results := make(chan health, len(pending))
	jobs := make(chan proxy)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if probeCtx.Err() != nil {
					return
				}
				h := m.probe(probeCtx, p)
				if probeCtx.Err() != nil {
					return
				}
				h.ProxyID = p.ID
				h.fingerprint = p.fingerprint()
				h.CheckedAt = time.Now()
				results <- h
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, p := range pending {
			select {
			case jobs <- p:
			case <-probeCtx.Done():
				return
			}
		}
	}()
	wg.Wait()
	close(results)
	for h := range results {
		old := hs[h.ProxyID]
		if h.Status == "unhealthy" {
			h.Failures = 1
			if old.Status == "unhealthy" && old.fingerprint == h.fingerprint && h.CheckedAt.Sub(old.CheckedAt) < 3*time.Minute {
				h.Failures = min(old.Failures+1, 100)
			}
		}
		hs[h.ProxyID] = h
		_, err = tx.ExecContext(ctx, `INSERT INTO proxy_failover_health(proxy_id,fingerprint,status,failures,latency_ms,checked_at,message) VALUES($1,$2,$3,$4,$5,$6,$7)
  ON CONFLICT(proxy_id) DO UPDATE SET fingerprint=EXCLUDED.fingerprint,status=EXCLUDED.status,failures=EXCLUDED.failures,latency_ms=EXCLUDED.latency_ms,checked_at=EXCLUDED.checked_at,message=EXCLUDED.message`, h.ProxyID, h.fingerprint, h.Status, h.Failures, h.LatencyMS, h.CheckedAt, h.Message)
		if err != nil {
			return err
		}
	}
	for _, a := range policies {
		next := choose(a.Policy, ps, hs, time.Now())
		if next == 0 {
			continue
		}
		if err = m.switchProxy(ctx, tx, a, next, ps, hs); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM proxy_failover_events WHERE created_at < NOW()-INTERVAL '30 days'`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func choose(p Policy, ps map[int64]proxy, hs map[int64]health, now time.Time) int64 {
	current := hs[p.CurrentProxyID]
	if current.Status != "unhealthy" || current.Failures < 2 || now.Sub(current.CheckedAt) > freshness || current.fingerprint != ps[p.CurrentProxyID].fingerprint() {
		return 0
	}
	for _, id := range append([]int64{p.PrimaryProxyID}, p.BackupProxyIDs...) {
		h := hs[id]
		candidate, ok := ps[id]
		if id != p.CurrentProxyID && ok && candidate.available(now) && h.Status == "healthy" && h.fingerprint == candidate.fingerprint() && now.Sub(h.CheckedAt) <= freshness {
			return id
		}
	}
	return 0
}
func (m *Manager) switchProxy(ctx context.Context, tx *sql.Tx, a accountPolicy, next int64, ps map[int64]proxy, hs map[int64]health) error {
	var current int64
	var eligible bool
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(proxy_id,0),deleted_at IS NULL AND platform='adobe' AND type='oauth' AND status='active' AND schedulable AND (expires_at IS NULL OR expires_at>NOW()) FROM accounts WHERE id=$1 FOR UPDATE`, a.ID).Scan(&current, &eligible)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if current != a.CurrentProxyID || !eligible {
		return nil
	}
	var rev int64
	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT revision,enabled FROM account_proxy_failover WHERE account_id=$1 FOR UPDATE`, a.ID).Scan(&rev, &enabled); err != nil {
		return err
	}
	if !enabled || rev != a.Revision {
		return nil
	}
	// Recheck both definitions under row locks; never apply a probe to edited nodes.
	ids := []int64{current, next}
	slices.Sort(ids)
	for _, id := range ids {
		var updated time.Time
		var valid bool
		err = tx.QueryRowContext(ctx, `SELECT updated_at,deleted_at IS NULL AND status='active' AND (expires_at IS NULL OR expires_at>NOW()) FROM proxies WHERE id=$1 FOR SHARE`, id).Scan(&updated, &valid)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if !updated.Equal(ps[id].UpdatedAt) || (id == next && !valid) {
			return nil
		}
	}
	if time.Since(hs[next].CheckedAt) > freshness {
		return nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=$2,proxy_fallback_origin_id=COALESCE(proxy_fallback_origin_id,$3),updated_at=NOW() WHERE id=$1`, a.ID, next, a.PrimaryProxyID)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id,created_at) VALUES($1,$2,$3,clock_timestamp())`, a.ID, current, next); err != nil {
		return err
	}
	// The account mutation and scheduler invalidation commit together.
	payload, _ := json.Marshal(map[string]any{"account_ids": []int64{a.ID}})
	_, err = tx.ExecContext(ctx, `INSERT INTO scheduler_outbox(event_type,payload) VALUES('account_bulk_changed',$1)`, string(payload))
	return err
}
