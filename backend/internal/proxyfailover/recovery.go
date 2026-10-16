package proxyfailover

import (
	"context"
	"time"
)

// Recover uses measured health and an immediate try-lock. Requests never wait
// for network probes, start Adobe requests, or form a recovery queue.
func (m *Manager) Recover(ctx context.Context) bool {
	if m == nil {
		return false
	}
	now := time.Now().UnixNano()
	last := m.demandAt.Load()
	if now-last < int64(2*time.Second) || !m.demandAt.CompareAndSwap(last, now) {
		return false
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return false
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='50ms'`); err != nil {
		return false
	}
	var leader bool
	if err = tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(241092601)`).Scan(&leader); err != nil || !leader {
		return false
	}
	var p AllocationPolicy
	if err = tx.QueryRowContext(ctx, `SELECT enabled,revision FROM proxy_auto_allocation WHERE id=1`).Scan(&p.Enabled, &p.Revision); err != nil || !p.Enabled {
		return false
	}
	if err = m.allocate(ctx, tx, p, false); err != nil {
		return false
	}
	return tx.Commit() == nil
}

type PoolFailure struct {
	Reason     string
	RetryAfter int
}

func (m *Manager) DiagnosePool(ctx context.Context, groupID *int64) PoolFailure {
	result := PoolFailure{Reason: "accounts_temporarily_unavailable", RetryAfter: 30}
	if m == nil || groupID == nil {
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	var total, enabled, waiting, credentials int
	err := m.db.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE a.schedulable),count(*) FILTER(WHERE a.proxy_auto_paused),count(*) FILTER(WHERE a.schedulable AND a.status='error' AND NOT a.proxy_auto_paused)
 FROM accounts a JOIN account_groups ag ON ag.account_id=a.id WHERE a.deleted_at IS NULL AND a.platform='adobe' AND ag.group_id=$1`, *groupID).Scan(&total, &enabled, &waiting, &credentials)
	if err != nil {
		return result
	}
	switch {
	case total == 0:
		result.Reason = "empty_account_pool"
		result.RetryAfter = 60
	case waiting > 0:
		result.Reason = "proxy_unavailable"
	case enabled == 0:
		result.Reason = "no_enabled_accounts"
		result.RetryAfter = 60
	case credentials == enabled:
		result.Reason = "account_authorization_required"
		result.RetryAfter = 60
	}
	return result
}
