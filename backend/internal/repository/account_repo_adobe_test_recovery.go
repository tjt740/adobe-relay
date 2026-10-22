package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// RecoverAdobeAfterSuccessfulTestIfCurrent is shared by single, bulk and Okad
// full-generation checks. It never changes operator scheduling, credentials or
// proxy bindings, and a result cannot clear a newer failure or route change.
func (r *accountRepository) RecoverAdobeAfterSuccessfulTestIfCurrent(ctx context.Context, a *service.Account, started time.Time) (bool, error) {
	if a == nil || a.Platform != service.PlatformAdobe || a.Type != service.AccountTypeOAuth || started.IsZero() {
		return false, nil
	}
	var proxyUpdatedAt *time.Time
	if a.Proxy != nil {
		proxyUpdatedAt = &a.Proxy.UpdatedAt
	}
	result, err := r.sql.ExecContext(ctx, `WITH eligible AS (
 SELECT a.id FROM accounts a WHERE a.id=$1 AND a.deleted_at IS NULL AND a.platform='adobe' AND a.type='oauth'
 AND COALESCE(a.credentials->>'cookie','')=$2 AND a.proxy_id IS NOT DISTINCT FROM $3
 AND ($4::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies WHERE id=a.proxy_id AND updated_at=$4 AND deleted_at IS NULL AND status='active'))
 AND a.status=$5 AND COALESCE(a.error_message,'')=$6 AND a.schedulable=$7
 AND (COALESCE(a.proxy_auto_paused,false)=false OR a.error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）')
 AND (a.expires_at IS NULL OR a.expires_at>NOW())
 AND (a.proxy_id IS NOT NULL OR NOT EXISTS(SELECT 1 FROM proxy_auto_allocation WHERE enabled))
 AND (COALESCE(a.extra->'okad_connection_test'->>'require_verification','false')='false' OR a.extra->'okad'->>'verification'='verified')
 AND a.rate_limited_at IS NOT DISTINCT FROM $8::timestamptz
 AND a.rate_limit_reset_at IS NOT DISTINCT FROM $9::timestamptz
 AND a.overload_until IS NOT DISTINCT FROM $10::timestamptz
 AND a.temp_unschedulable_until IS NOT DISTINCT FROM $11::timestamptz
 AND COALESCE(a.temp_unschedulable_reason,'')=$12
 AND NOT EXISTS(SELECT 1 FROM adobe_account_health h WHERE h.account_id=a.id AND h.failed_at>=$13)
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id)
 AND a.status IN ('active','error')
 AND (a.status='active' OR a.error_message IN ('Clash 订阅节点已移除，请重新选择代理并恢复账号调度',
 '自动代理分配：等待可用节点（每个节点最多 3 个账号）',$15,$16)
 OR LOWER(a.error_message) LIKE 'authentication failed (401):%'
 OR LOWER(a.error_message) LIKE 'oauth 401 (no refresh_token):%'
 OR LOWER(a.error_message) LIKE 'token refresh failed (non-retryable): adobe cookie is no longer valid%')
 AND (a.status='error' OR a.rate_limited_at IS NOT NULL OR a.rate_limit_reset_at IS NOT NULL OR a.overload_until IS NOT NULL OR a.temp_unschedulable_until IS NOT NULL)
 FOR UPDATE), updated AS (
 UPDATE accounts a SET status='active',error_message=NULL,proxy_auto_paused=false,rate_limited_at=NULL,rate_limit_reset_at=NULL,
 overload_until=NULL,temp_unschedulable_until=NULL,temp_unschedulable_reason=NULL,updated_at=NOW()
 FROM eligible e WHERE a.id=e.id RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $14,id,NULL,NULL FROM updated`,
		a.ID, a.GetCredential("cookie"), a.ProxyID, proxyUpdatedAt, a.Status, a.ErrorMessage, a.Schedulable,
		a.RateLimitedAt, a.RateLimitResetAt, a.OverloadUntil, a.TempUnschedulableUntil, a.TempUnschedulableReason, started,
		service.SchedulerOutboxEventAccountChanged, service.OkadConnectionTestPending, service.OkadVerificationPending)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err == nil && n > 0 {
		r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	}
	return n > 0, err
}
