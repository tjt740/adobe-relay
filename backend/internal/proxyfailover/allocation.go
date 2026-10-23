package proxyfailover

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/clash"
)

const accountsPerProxy = 3
const allocationWaitingMessage = "自动代理分配：等待可用节点（每个节点最多 3 个账号）"

var ErrManualProxy = errors.New("manual IP mode is enabled; disable it in account settings before configuring automatic failover")

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
	ReclaimableSlots             int `json:"reclaimable_slots"`
	BusinessWaiting              int `json:"business_waiting"`
	VerificationPriorityAccounts int `json:"verification_priority_accounts"`
	VerificationWaiting          int `json:"verification_waiting"`
	Quarantined408Accounts       int `json:"quarantined_408_accounts"`
	TemporaryTestBindings        int `json:"temporary_test_bindings"`
	Pending408Release            int `json:"pending_408_release"`
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
	inactiveCounts := map[int64]int{}
	for _, a := range accounts {
		counts[a.Current]++
		if a.ManualProxy {
			continue
		}
		v.Accounts++
		if a.TestLease {
			v.TemporaryTestBindings++
			continue
		}
		if a.Quarantined408 {
			v.Quarantined408Accounts++
			if a.Current != 0 {
				v.Pending408Release++
			}
			continue
		}
		if a.BindingOnly && !a.VerificationPriority {
			inactiveCounts[a.Current]++
		}
		if a.VerificationPriority {
			v.VerificationPriorityAccounts++
			if a.Current == 0 {
				v.VerificationWaiting++
			}
		}
		if !a.BindingOnly && (a.Current == 0 || a.Waiting) {
			v.BusinessWaiting++
		}
		if !a.Eligible {
			continue
		}
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
		} else if h.fingerprint == p.fingerprint() && time.Since(h.CheckedAt) <= clash.ProbeFreshness(len(ps)) {
			node.Status = h.Status
			node.LatencyMS = h.LatencyMS
		}
		if node.Status == "healthy" {
			v.HealthyNodes++
			v.AvailableSlots += max(0, accountsPerProxy-node.Accounts)
			v.ReclaimableSlots += min(accountsPerProxy, inactiveCounts[node.ID])
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
 WHERE deleted_at IS NULL AND extra->'manual_proxy_enabled' IS DISTINCT FROM 'true'::jsonb AND status='active' AND schedulable AND (auto_pause_on_expired IS NOT TRUE OR expires_at IS NULL OR expires_at>NOW())
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=accounts.id)
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=accounts.id) RETURNING id`, allocationWaitingMessage)
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
		if _, err = tx.ExecContext(ctx, `UPDATE account_proxy_failover SET enabled=false,revision=revision+1,updated_at=NOW() WHERE enabled AND NOT EXISTS(SELECT 1 FROM accounts a WHERE a.id=account_proxy_failover.account_id AND a.extra->'manual_proxy_enabled'='true'::jsonb)`); err != nil {
			return AllocationView{}, err
		}
	}
	if err = Refresh408Metadata(ctx, tx, 0); err != nil {
		return AllocationView{}, err
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
	LastSwitch        time.Time
	BindingOnly       bool
	// Priority affects capacity only. Okad retains ownership of verification
	// and scheduling; assigning a route must never resume this account.
	VerificationPriority bool
	Quarantined408       bool
	ManualProxy          bool
	TestLease            bool
}

func (a allocationAccount) priority() int {
	if !a.BindingOnly {
		return 0
	}
	if a.VerificationPriority {
		return 1
	}
	return 2
}

func loadAllocationAccounts(ctx context.Context, q queryer) ([]allocationAccount, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,COALESCE(proxy_id,0),
 true,
	 COALESCE(proxy_auto_paused AND status='error' AND NOT schedulable AND error_message=$1,false)
	 OR COALESCE(status='error' AND schedulable AND NOT proxy_auto_paused AND error_message=$1,false),
	 NOT ((auto_pause_on_expired IS NOT TRUE OR expires_at IS NULL OR expires_at>NOW()) AND
	 ((status='active' AND schedulable) OR COALESCE(proxy_auto_paused AND status='error' AND NOT schedulable AND error_message=$1,false)
	 OR COALESCE(status='error' AND schedulable AND NOT proxy_auto_paused AND error_message=$1,false)))
	 ,COALESCE((SELECT created_at FROM proxy_failover_events e WHERE e.account_id=accounts.id ORDER BY e.id DESC LIMIT 1),'epoch'::timestamptz),
	 COALESCE(platform='adobe' AND type='oauth' AND status='error'
	 AND (expires_at IS NULL OR expires_at>NOW())
	 AND extra->'okad'->'resume'='true'::jsonb
	 AND extra->'okad_manual_pause' IS DISTINCT FROM 'true'::jsonb
	 AND extra->'okad'->>'verification' IN ('pending','proxy_pending','verification_retry')
	 AND error_message IN ('Okad credentials pending verification','Okad credentials pending connection test')
	 AND NULLIF(BTRIM(credentials->>'cookie'),'') IS NOT NULL,false),
	 EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=accounts.id)
	 OR EXISTS(SELECT 1 FROM adobe_proxy_408_holds h WHERE h.account_id=accounts.id),
	 EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=accounts.id AND l.expires_at>NOW()),
 COALESCE(extra->'manual_proxy_enabled'='true'::jsonb,false)
 FROM accounts WHERE deleted_at IS NULL ORDER BY id`, allocationWaitingMessage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []allocationAccount{}
	for rows.Next() {
		var a allocationAccount
		if err = rows.Scan(&a.ID, &a.Current, &a.Eligible, &a.Waiting, &a.BindingOnly, &a.LastSwitch, &a.VerificationPriority, &a.Quarantined408, &a.TestLease, &a.ManualProxy); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func freshHealthy(p proxy, h health, now time.Time) bool {
	return p.available(now) && h.Status == "healthy" && h.fingerprint == p.fingerprint() && now.Sub(h.CheckedAt) <= freshness
}

// Preserve business routes, then reserve capacity for recoverable verification
// before retaining inactive bindings. Verification only gets a route, not a
// scheduling release; manual pauses and failed credentials keep their state.
func planAllocation(accounts []allocationAccount, ps map[int64]proxy, hs map[int64]health, now time.Time) map[int64]int64 {
	used := map[int64]int{}
	next := map[int64]int64{}
	for _, a := range accounts {
		if a.ManualProxy {
			if a.Current != 0 {
				used[a.Current]++
			}
			continue
		}
		if a.Quarantined408 && !a.TestLease {
			next[a.ID] = 0
			continue
		}
		if (!a.Eligible || a.TestLease) && a.Current != 0 {
			used[a.Current]++
		}
	}
	ordered := append([]allocationAccount(nil), accounts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Waiting != ordered[j].Waiting {
			return !ordered[i].Waiting
		}
		return ordered[i].ID < ordered[j].ID
	})
	fresh := func(id int64) bool {
		p, ok := ps[id]
		h := hs[id]
		return ok && p.available(now) && h.Status == "healthy" && h.fingerprint == p.fingerprint() && now.Sub(h.CheckedAt) <= clash.ProbeFreshness(len(ps))
	}
	best := func(except int64) int64 {
		var chosen int64
		for id := range ps {
			if id == except || used[id] >= accountsPerProxy || !fresh(id) {
				continue
			}
			if chosen != 0 && ps[id].sourceTrafficAvailable != ps[chosen].sourceTrafficAvailable {
				if ps[id].sourceTrafficAvailable {
					chosen = id
				}
				continue
			}
			if chosen == 0 || routeScore(hs[id], used[id]) < routeScore(hs[chosen], used[chosen]) || (routeScore(hs[id], used[id]) == routeScore(hs[chosen], used[chosen]) && id < chosen) {
				chosen = id
			}
		}
		return chosen
	}
	for priority := 0; priority < 3; priority++ {
		for _, a := range ordered {
			if a.ManualProxy || !a.Eligible || a.Quarantined408 || a.TestLease || a.priority() != priority {
				continue
			}
			p, exists := ps[a.Current]
			h := hs[a.Current]
			keep := fresh(a.Current)
			if (!a.Waiting || a.BindingOnly) && exists && p.available(now) && h.fingerprint == p.fingerprint() && !(h.Status == "unhealthy" && h.Failures >= 2) {
				keep = true
			}
			if keep && used[a.Current] < accountsPerProxy {
				next[a.ID] = a.Current
				used[a.Current]++
			}
		}
		for _, a := range ordered {
			if a.ManualProxy || !a.Eligible || a.Quarantined408 || a.TestLease || a.priority() != priority || next[a.ID] != 0 {
				continue
			}
			id := best(0)
			next[a.ID] = id
			if id != 0 {
				used[id]++
			}
		}
		if priority == 0 {
			for _, a := range ordered {
				current := next[a.ID]
				if a.ManualProxy || a.BindingOnly || a.Quarantined408 || a.TestLease || current == 0 || current != a.Current || !fresh(current) || now.Sub(a.LastSwitch) < softSwitchCooldown {
					continue
				}
				candidate := best(current)
				if candidate != 0 && (!ps[current].sourceTrafficAvailable || ps[candidate].sourceTrafficAvailable) && betterLatencyRoute(hs[current], hs[candidate]) {
					next[a.ID] = candidate
					used[current]--
					used[candidate]++
				}
			}
		}
	}
	return next
}

func (m *Manager) checkAllocation(ctx context.Context, tx *sql.Tx, policy AllocationPolicy) error {
	return m.allocate(ctx, tx, policy, true)
}
func (m *Manager) allocate(ctx context.Context, tx *sql.Tx, policy AllocationPolicy, probe bool) error {
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
	if probe {
		if err = m.refreshHealth(ctx, tx, ps, hs, needed, false); err != nil {
			return err
		}
	}
	// Lock source admission before accounts, matching subscription import order.
	var sourceDocument []byte
	if err = tx.QueryRowContext(ctx, `SELECT document FROM clash_subscription WHERE id=1 FOR SHARE`).Scan(&sourceDocument); err != nil {
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
	if err = quarantine408(ctx, tx); err != nil {
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
		if a.ManualProxy || a.Quarantined408 || a.TestLease {
			continue
		}
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
