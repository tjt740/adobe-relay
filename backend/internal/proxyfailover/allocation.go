package proxyfailover

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

const accountsPerProxy = 3
const allocationWaitingMessage = "自动代理分配：等待可用节点（每个节点最多 3 个账号）"

var ErrAutoManaged = errors.New("automatic proxy allocation is enabled; manage it in IP management")

type AllocationPolicy struct {
	Enabled  bool  `json:"enabled"`
	Revision int64 `json:"revision"`
}

type AllocationNode struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Accounts  int    `json:"accounts"`
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
}

type AllocationView struct {
	AllocationPolicy
	AccountsPerProxy    int              `json:"accounts_per_proxy"`
	Accounts            int              `json:"accounts"`
	Assigned            int              `json:"assigned"`
	Waiting             int              `json:"waiting"`
	BindingOnlyAccounts int              `json:"binding_only_accounts"`
	HealthyNodes        int              `json:"healthy_nodes"`
	AvailableSlots      int              `json:"available_slots"`
	CheckedAt           *time.Time       `json:"checked_at"`
	Nodes               []AllocationNode `json:"nodes"`
}

func (m *Manager) GetAllocation(ctx context.Context) (AllocationView, error) {
	v := AllocationView{AccountsPerProxy: accountsPerProxy, Nodes: []AllocationNode{}}
	if err := m.db.QueryRowContext(ctx, `SELECT enabled,revision,checked_at FROM proxy_auto_allocation WHERE id=1`).Scan(&v.Enabled, &v.Revision, &v.CheckedAt); err != nil {
		return v, err
	}
	accounts, err := loadAllocationAccounts(ctx, m.db)
	if err != nil {
		return v, err
	}
	counts := map[int64]int{}
	for _, a := range accounts {
		counts[a.Current]++
		if !a.Eligible {
			continue
		}
		v.Accounts++
		if a.BindingOnly {
			v.BindingOnlyAccounts++
		}
		if a.Current == 0 || (a.Waiting && !a.BindingOnly) {
			v.Waiting++
		} else {
			v.Assigned++
		}
	}
	ps, err := loadProxies(ctx, m.db)
	if err != nil {
		return v, err
	}
	hs, err := loadHealth(ctx, m.db)
	if err != nil {
		return v, err
	}
	rows, err := m.db.QueryContext(ctx, `SELECT id,name FROM proxies WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var node AllocationNode
		if err = rows.Scan(&node.ID, &node.Name); err != nil {
			return v, err
		}
		node.Accounts = counts[node.ID]
		node.Status = "unknown"
		p, exists := ps[node.ID]
		h := hs[node.ID]
		if !exists || !p.available(time.Now()) {
			node.Status = "unavailable"
		} else if h.fingerprint == p.fingerprint() && time.Since(h.CheckedAt) <= freshness {
			node.Status = h.Status
			node.LatencyMS = h.LatencyMS
		}
		if node.Status == "healthy" {
			v.HealthyNodes++
			v.AvailableSlots += max(0, accountsPerProxy-node.Accounts)
		}
		v.Nodes = append(v.Nodes, node)
	}
	return v, rows.Err()
}

func (m *Manager) SaveAllocation(ctx context.Context, policy AllocationPolicy) (AllocationView, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return AllocationView{}, err
	}
	defer tx.Rollback()
	// Serialize the short mutation phase with account creation/import/bulk edits.
	// Never hold this table lock while making network probes.
	if _, err = tx.ExecContext(ctx, `LOCK TABLE accounts IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return AllocationView{}, err
	}
	var before AllocationPolicy
	if err = tx.QueryRowContext(ctx, `SELECT enabled,revision FROM proxy_auto_allocation WHERE id=1 FOR UPDATE`).Scan(&before.Enabled, &before.Revision); err != nil {
		return AllocationView{}, err
	}
	if before.Revision != policy.Revision {
		return AllocationView{}, ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE proxy_auto_allocation SET enabled=$1,revision=revision+1 WHERE id=1`, policy.Enabled); err != nil {
		return AllocationView{}, err
	}
	if policy.Enabled && !before.Enabled {
		// Initial routes must be checked, including accounts currently using DIRECT.
		if _, err = tx.ExecContext(ctx, `SELECT set_config('sub2api.proxy_allocator','on',true)`); err != nil {
			return AllocationView{}, err
		}
		rows, e := tx.QueryContext(ctx, `UPDATE accounts SET proxy_auto_paused=true,status='error',schedulable=false,error_message=$1,updated_at=NOW()
 WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth' AND status='active' AND schedulable AND (expires_at IS NULL OR expires_at>NOW()) RETURNING id`, allocationWaitingMessage)
		if e != nil {
			return AllocationView{}, e
		}
		ids := []int64{}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return AllocationView{}, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return AllocationView{}, e
		}
		if err = allocationOutbox(ctx, tx, ids); err != nil {
			return AllocationView{}, err
		}
		// A legacy per-account switch must not bypass the shared capacity limit.
		if _, err = tx.ExecContext(ctx, `UPDATE account_proxy_failover SET enabled=false,revision=revision+1,updated_at=NOW() WHERE enabled`); err != nil {
			return AllocationView{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return AllocationView{}, err
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return m.GetAllocation(ctx)
}

type allocationAccount struct {
	ID, Current       int64
	Eligible, Waiting bool
	BindingOnly       bool
}

func loadAllocationAccounts(ctx context.Context, q queryer) ([]allocationAccount, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,COALESCE(proxy_id,0),
 platform='adobe' AND type='oauth',
 COALESCE(proxy_auto_paused AND status='error' AND NOT schedulable AND error_message=$1,false),
 NOT ((expires_at IS NULL OR expires_at>NOW()) AND
 ((status='active' AND schedulable) OR COALESCE(proxy_auto_paused AND status='error' AND NOT schedulable AND error_message=$1,false)))
 FROM accounts WHERE deleted_at IS NULL ORDER BY id`, allocationWaitingMessage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []allocationAccount{}
	for rows.Next() {
		var a allocationAccount
		if err = rows.Scan(&a.ID, &a.Current, &a.Eligible, &a.Waiting, &a.BindingOnly); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func freshHealthy(p proxy, h health, now time.Time) bool {
	return p.available(now) && h.Status == "healthy" && h.fingerprint == p.fingerprint() && now.Sub(h.CheckedAt) <= freshness
}

// Preserve working bindings first, then fill a node to three before using the
// next. Existing inactive Adobe accounts get bindings too, without being resumed.
// Other-platform accounts keep their reserved slots and are never modified.
func planAllocation(accounts []allocationAccount, ps map[int64]proxy, hs map[int64]health, now time.Time) map[int64]int64 {
	used := map[int64]int{}
	next := map[int64]int64{}
	for _, a := range accounts {
		if !a.Eligible && a.Current != 0 {
			used[a.Current]++
		}
	}
	ordered := append([]allocationAccount(nil), accounts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].BindingOnly != ordered[j].BindingOnly {
			return !ordered[i].BindingOnly
		}
		if ordered[i].Waiting != ordered[j].Waiting {
			return !ordered[i].Waiting
		}
		return ordered[i].ID < ordered[j].ID
	})
	for _, a := range ordered {
		if !a.Eligible {
			continue
		}
		p, exists := ps[a.Current]
		h := hs[a.Current]
		keep := exists && freshHealthy(p, h, now)
		// One failed probe, a stale measurement or an upstream outage does not
		// move an established route. Edited/disabled/expired proxies do.
		if !a.Waiting && exists && p.available(now) && h.fingerprint == p.fingerprint() && !(h.Status == "unhealthy" && h.Failures >= 2) {
			keep = true
		}
		if keep && used[a.Current] < accountsPerProxy {
			next[a.ID] = a.Current
			used[a.Current]++
		}
	}
	for _, a := range ordered {
		if !a.Eligible || next[a.ID] != 0 {
			continue
		}
		var best int64
		for id, p := range ps {
			if used[id] >= accountsPerProxy || !freshHealthy(p, hs[id], now) {
				continue
			}
			if best == 0 || used[id] > used[best] || (used[id] == used[best] && (hs[id].LatencyMS < hs[best].LatencyMS || (hs[id].LatencyMS == hs[best].LatencyMS && id < best))) {
				best = id
			}
		}
		next[a.ID] = best
		if best != 0 {
			used[best]++
		}
	}
	return next
}

func (m *Manager) checkAllocation(ctx context.Context, tx *sql.Tx, policy AllocationPolicy) error {
	ps, err := loadProxies(ctx, tx)
	if err != nil {
		return err
	}
	hs, err := loadHealth(ctx, tx)
	if err != nil {
		return err
	}
	needed := map[int64]bool{}
	for id, p := range ps {
		if !p.Deleted {
			needed[id] = true
		}
	}
	if err = m.refreshHealth(ctx, tx, ps, hs, needed, false); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `LOCK TABLE accounts IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	var current AllocationPolicy
	if err = tx.QueryRowContext(ctx, `SELECT enabled,revision FROM proxy_auto_allocation WHERE id=1 FOR SHARE`).Scan(&current.Enabled, &current.Revision); err != nil {
		return err
	}
	if current != policy {
		return nil
	}
	// Lock definitions only after probing, then reload: a concurrent edit cannot
	// inherit the old endpoint's successful result, even on the same proxy ID.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM proxies ORDER BY id FOR SHARE`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	ps, err = loadProxies(ctx, tx)
	if err != nil {
		return err
	}
	if err = persistHealth(ctx, tx, ps, hs); err != nil {
		return err
	}
	accounts, err := loadAllocationAccounts(ctx, tx)
	if err != nil {
		return err
	}
	next := planAllocation(accounts, ps, hs, time.Now())
	if _, err = tx.ExecContext(ctx, `SELECT set_config('sub2api.proxy_allocator','on',true)`); err != nil {
		return err
	}
	changed := []int64{}
	for _, a := range accounts {
		pid, managed := next[a.ID]
		if !managed || (pid == a.Current && (a.BindingOnly || a.Waiting == (pid == 0))) {
			continue
		}
		if a.BindingOnly {
			// Binding a stopped, expired or credential-error account must not
			// change its scheduling state, error, expiry or ownership of a pause.
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=NULLIF($2,0),proxy_fallback_origin_id=NULL,updated_at=NOW() WHERE id=$1`, a.ID, pid)
		} else if pid == 0 {
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=NULL,proxy_fallback_origin_id=NULL,proxy_auto_paused=true,status='error',schedulable=false,error_message=$2,updated_at=NOW() WHERE id=$1`, a.ID, allocationWaitingMessage)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE accounts SET proxy_id=$2,proxy_fallback_origin_id=NULL,proxy_auto_paused=false,status='active',schedulable=true,error_message=NULL,updated_at=NOW() WHERE id=$1`, a.ID, pid)
		}
		if err != nil {
			return err
		}
		changed = append(changed, a.ID)
		if a.Current != pid {
			if _, err = tx.ExecContext(ctx, `INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id,created_at) VALUES($1,$2,$3,clock_timestamp())`, a.ID, a.Current, pid); err != nil {
				return err
			}
		}
	}
	if err = allocationOutbox(ctx, tx, changed); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM proxy_failover_events WHERE created_at < NOW()-INTERVAL '30 days'`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE proxy_auto_allocation SET checked_at=clock_timestamp() WHERE id=1`)
	return err
}

func allocationOutbox(ctx context.Context, tx *sql.Tx, ids []int64) error {
	for start := 0; start < len(ids); start += 500 {
		payload, _ := json.Marshal(map[string]any{"account_ids": ids[start:min(start+500, len(ids))]})
		if _, err := tx.ExecContext(ctx, `INSERT INTO scheduler_outbox(event_type,payload) VALUES('account_bulk_changed',$1)`, string(payload)); err != nil {
			return err
		}
	}
	return nil
}
