package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// A late IMS failure cannot undo an operator pause or condemn a replacement
// proxy, even when the Cookie text itself has not changed.
func (r *accountRepository) SetAdobeErrorIfCurrent(ctx context.Context, a *service.Account, message string) (bool, error) {
	if a == nil || a.GetCredential("cookie") == "" {
		return false, nil
	}
	var proxyUpdatedAt *time.Time
	if a.Proxy != nil {
		proxyUpdatedAt = &a.Proxy.UpdatedAt
	}
	result, err := r.sql.ExecContext(ctx, `WITH updated AS (
 UPDATE accounts SET status='error',schedulable=false,error_message=$2,updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND credentials->>'cookie'=$3 AND status=$4 AND schedulable=$5
 AND proxy_id IS NOT DISTINCT FROM $6
 AND COALESCE(extra->>'okad_manual_pause','false')=$7
 AND ($8::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies WHERE id=accounts.proxy_id AND updated_at=$8))
 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $9,id,NULL,NULL FROM updated`, a.ID, message, a.GetCredential("cookie"), a.Status, a.Schedulable, a.ProxyID, fmtBool(a.Extra["okad_manual_pause"] == true), proxyUpdatedAt, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if n > 0 {
		r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	}
	return n > 0, err
}
