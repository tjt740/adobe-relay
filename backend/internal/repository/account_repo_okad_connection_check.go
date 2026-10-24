package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) QueueOkadConnectionTest(ctx context.Context, a *service.Account, key string, requireVerification bool) error {
	resume := service.OkadConnectionTestMayResume(a)
	if state, ok := a.Extra["okad"].(map[string]any); ok && state["resume"] == true && a.ErrorMessage == service.OkadVerificationPending {
		resume = true
	}
	raw, err := json.Marshal(service.NewOkadConnectionTestState(a, requireVerification, resume))
	if err != nil {
		return err
	}
	_, err = r.sql.ExecContext(ctx, `WITH updated AS (UPDATE accounts SET
 extra=(COALESCE(extra,'{}'::jsonb)-'last_check_at'-'last_check_success'-'last_check_status_code')||jsonb_build_object('okad_connection_test',$4::jsonb),
 status=CASE WHEN $5 AND status='active' THEN 'error' ELSE status END,
 error_message=CASE WHEN $5 AND status='active' THEN $6 ELSE error_message END,
 schedulable=false,updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND credentials->>'cookie'=$3 AND COALESCE(extra->'okad_connection_test'->>'key','')<>$2
 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $7,id,NULL,NULL FROM updated`, a.ID, key, a.GetCredential("cookie"), string(raw), resume, service.OkadConnectionTestPending, service.SchedulerOutboxEventAccountChanged)
	if err == nil {
		r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	}
	return err
}

func (r *accountRepository) ClaimOkadConnectionTest(ctx context.Context) (*service.OkadConnectionTestJob, error) {
	// Never replay a possibly submitted generation after a crashed process.
	_, err := r.sql.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test}',extra->'okad_connection_test'||jsonb_build_object('state','interrupted','finished_at',NOW())),updated_at=NOW()
 WHERE deleted_at IS NULL AND extra->'okad_connection_test'->>'state'='running'
 AND (extra->'okad_connection_test'->>'started_at')::timestamptz<NOW()-INTERVAL '5 minutes'`)
	if err != nil {
		return nil, err
	}
	job := &service.OkadConnectionTestJob{}
	rows, err := r.sql.QueryContext(ctx, `WITH candidate AS (
 SELECT id FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND COALESCE(credentials->>'cookie','')<>''
 AND extra->'okad_connection_test'->>'state' IN ('pending','proxy_pending')
 AND (extra->'okad_connection_test'->>'next_check_at' IS NULL OR (extra->'okad_connection_test'->>'next_check_at')::timestamptz<=NOW())
 AND extra->'okad_connection_test'->>'key'=encode(sha256(convert_to(credentials->>'cookie','UTF8')),'hex')
 AND (extra->'okad_connection_test'->>'require_verification'='false' OR extra->'okad'->>'verification'='verified')
 AND (proxy_id IS NOT NULL OR NOT EXISTS(SELECT 1 FROM proxy_auto_allocation WHERE enabled)
 OR extra->'manual_proxy_enabled'='true'::jsonb)
 ORDER BY extra->'okad_connection_test'->>'queued_at',id FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE accounts a SET extra=jsonb_set(a.extra,'{okad_connection_test}',a.extra->'okad_connection_test'||
 jsonb_build_object('state','running','started_at',NOW())),updated_at=NOW()
 FROM candidate WHERE a.id=candidate.id RETURNING a.id,a.extra->'okad_connection_test'->>'key',a.proxy_id,
 (SELECT updated_at FROM proxies WHERE id=a.proxy_id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	if err = rows.Scan(&job.AccountID, &job.Key, &job.ProxyID, &job.ProxyUpdatedAt); err != nil {
		return nil, err
	}
	return job, rows.Err()
}

func (r *accountRepository) FinishOkadConnectionTest(ctx context.Context, job service.OkadConnectionTestJob, state string) error {
	var manualStartedAt any
	if job.Manual && !job.StartedAt.IsZero() {
		manualStartedAt = job.StartedAt
	}
	result, err := r.sql.ExecContext(ctx, `WITH eligible AS (
 SELECT id,(($3='success' OR ($3='failed'
 AND extra->>'last_check_status_code'='408' AND extra->'last_check_success'='false'::jsonb
 AND (extra->>'last_check_at')::timestamptz >= COALESCE($10::timestamptz,(extra->'okad_connection_test'->>'started_at')::timestamptz)
 AND ($10::timestamptz IS NULL OR extra->'okad_connection_test'->>'queued_at' IS NULL
 OR $10::timestamptz >= (extra->'okad_connection_test'->>'queued_at')::timestamptz)
 AND (status='active' OR error_message=$6)))
 AND (extra->'okad_connection_test'->>'require_verification'='false' OR extra->'okad'->>'verification'='verified')
 AND extra->'okad'->>'verification' IS DISTINCT FROM 'quota_blocked'
 AND (status='active' OR (status='error' AND (error_message IN ($6,$7) OR LOWER(error_message) LIKE 'authentication failed (401):%' OR LOWER(error_message) LIKE 'token refresh failed (non-retryable): adobe cookie is no longer valid%')))
 AND LOWER(COALESCE(error_message,'')) NOT LIKE ALL(ARRAY['%taste_exhausted%','%quota exhausted%','%credits exhausted%','%model_not_entitled%','%user_not_entitled%','%not entitled%','%invalid_client%','%invalid_scope%'])
 AND (expires_at IS NULL OR expires_at>NOW())) AS verified
 FROM accounts WHERE id=$1 AND deleted_at IS NULL
 AND extra->'okad_connection_test'->>'key'=$2
 AND encode(sha256(convert_to(COALESCE(credentials->>'cookie',''),'UTF8')),'hex')=$2
 AND proxy_id IS NOT DISTINCT FROM $4
 AND ($5::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies WHERE id=accounts.proxy_id AND updated_at=$5))
 AND (extra->'okad_connection_test'->>'state'='running' OR $9)
 FOR UPDATE), updated AS (
 UPDATE accounts a SET extra=jsonb_set(a.extra,'{okad_connection_test}',
 a.extra->'okad_connection_test'||CASE WHEN e.verified THEN jsonb_build_object('gated',false) ELSE '{}'::jsonb END||jsonb_build_object('state',$3::text,'finished_at',NOW(),'next_check_at',CASE WHEN $3='proxy_pending' THEN NOW()+INTERVAL '30 seconds' ELSE NULL END)),
 status=CASE WHEN e.verified THEN 'active' ELSE a.status END,
 schedulable=CASE WHEN e.verified AND a.extra->'okad_connection_test'->>'gated'='true'
 AND a.extra->'okad_connection_test'->>'resume'='true' AND COALESCE(a.extra->>'okad_manual_pause','false')<>'true'
 AND NOT EXISTS(SELECT 1 FROM adobe_proxy_test_leases l WHERE l.account_id=a.id) THEN true ELSE a.schedulable END,
 error_message=CASE WHEN e.verified THEN NULL ELSE a.error_message END,updated_at=NOW()
 FROM eligible e WHERE a.id=e.id RETURNING a.id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $8,id,NULL,NULL FROM updated`, job.AccountID, job.Key, state, job.ProxyID, job.ProxyUpdatedAt, service.OkadConnectionTestPending, service.OkadVerificationPending, service.SchedulerOutboxEventAccountChanged, job.Manual, manualStartedAt)
	if err == nil {
		if n, _ := result.RowsAffected(); n > 0 {
			r.syncSchedulerAccountSnapshotDetached(ctx, job.AccountID)
		}
	}
	return err
}
