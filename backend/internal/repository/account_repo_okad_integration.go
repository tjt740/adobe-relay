package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// The allocator binds credential-paused accounts without enabling them. Wait
// for that binding before probing, rather than accidentally using DIRECT.
func (r *accountRepository) OkadVerificationProxyReady(ctx context.Context, a *service.Account) (bool, error) {
	if a.ProxyID != nil && (a.Proxy == nil || !a.Proxy.IsActive() || a.Proxy.IsExpired(time.Now())) {
		return false, nil
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT NOT enabled OR $1::bigint IS NOT NULL FROM proxy_auto_allocation WHERE id=1`, a.ProxyID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	ready := false
	if rows.Next() {
		err = rows.Scan(&ready)
	}
	if err != nil {
		return false, err
	}
	return ready, rows.Err()
}

func (r *accountRepository) ApplyOkadCredential(ctx context.Context, a *service.Account, p service.OkadCredentialUpdate) (bool, error) {
	old, _ := a.Extra["okad"].(map[string]any)
	msg := strings.ToLower(a.ErrorMessage)
	auth := strings.HasPrefix(msg, "authentication failed (401):") || strings.HasPrefix(msg, "oauth 401 (no refresh_token):") || (strings.HasPrefix(msg, "token refresh failed (non-retryable): adobe cookie is no longer valid") && !strings.Contains(msg, "invalid_client") && !strings.Contains(msg, "invalid_scope"))
	resume := (a.Status == service.StatusActive && a.Schedulable) || (a.Status == service.StatusError && (auth || (a.ErrorMessage == service.OkadVerificationPending && old["resume"] == true)))
	if a.Extra["okad_manual_pause"] == true || (a.ExpiresAt != nil && !a.ExpiresAt.After(time.Now())) {
		resume = false
	}
	state := map[string]any{"issuer": p.Issuer, "source_key": p.SourceKey, "version": p.Version, "verification": "pending", "message": "Cookie 已写入，等待通过当前代理验证", "resume": resume, "received_at": time.Now().UTC().Format(time.RFC3339), "model_verified": false}
	snapshot := *a
	snapshot.Credentials = service.MergeCredentials(a.Credentials, map[string]any{"cookie": p.Cookie})
	checkRaw, err := json.Marshal(service.NewOkadConnectionTestState(&snapshot, true, resume))
	if err != nil {
		return false, err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	result, err := r.sql.ExecContext(ctx, `WITH updated AS (
 UPDATE accounts a SET credentials=(COALESCE(credentials,'{}'::jsonb)-'access_token'-'expires_at'-'_token_version') || jsonb_build_object('cookie',$2::text),
 extra=(COALESCE(extra,'{}'::jsonb)-'okad_recovery'-'last_check_at'-'last_check_success'-'last_check_status_code')||jsonb_build_object('okad',$3::jsonb,'okad_connection_test',$12::jsonb),
 status=CASE WHEN $4 THEN 'error' ELSE status END,
 error_message=CASE WHEN $4 THEN $5 ELSE error_message END,
 schedulable=CASE WHEN $4 THEN FALSE ELSE schedulable END,updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND updated_at=$6 AND COALESCE(credentials->>'cookie','')=$7
 AND (COALESCE(extra->'okad'->>'issuer','')='' OR (extra->'okad'->>'issuer'=$8 AND extra->'okad'->>'source_key'=$9))
 AND (CASE WHEN COALESCE(extra->'okad'->>'version','') ~ '^[0-9]{1,16}$' THEN (extra->'okad'->>'version')::bigint ELSE 0 END)<$10
 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $11,id,NULL,NULL FROM updated`, a.ID, p.Cookie, string(raw), resume, service.OkadVerificationPending, a.UpdatedAt, a.GetCredential("cookie"), p.Issuer, p.SourceKey, p.Version, service.SchedulerOutboxEventAccountChanged, string(checkRaw))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if n > 0 {
		r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	}
	return n > 0, err
}

func (r *accountRepository) CompleteOkadVerification(ctx context.Context, a *service.Account, fields map[string]any, code, message string) (bool, error) {
	prior, _ := a.Extra["okad"].(map[string]any)
	state := map[string]any{}
	for k, v := range prior {
		state[k] = v
	}
	state["verification"] = code
	state["message"] = message
	state["verified_at"] = time.Now().UTC().Format(time.RFC3339)
	definitiveFailure := code == "auth_failed" || code == "identity_mismatch" || code == "quota_blocked" || code == "entitlement_blocked"
	quarantine := a.Status == service.StatusActive && a.Schedulable && definitiveFailure
	if quarantine {
		state["resume"] = true
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return false, err
	}
	tokenFields := map[string]any{}
	for _, key := range []string{"access_token", "expires_at"} {
		if v, ok := fields[key]; ok {
			tokenFields[key] = v
		}
	}
	tokens, err := json.Marshal(tokenFields)
	if err != nil {
		return false, err
	}
	// A transient probe may replace verification=auth_failed with retry while
	// the original auth quarantine still belongs to this integration.
	ownsPause := prior["resume"] == true && a.Status == service.StatusError && !a.Schedulable && (a.ErrorMessage == service.OkadVerificationPending || strings.HasPrefix(a.ErrorMessage, "Authentication failed (401):"))
	resume := ownsPause && a.Extra["okad_manual_pause"] != true && code == "verified" && (a.ExpiresAt == nil || a.ExpiresAt.After(time.Now()))
	if check, ok := a.Extra["okad_connection_test"].(map[string]any); ok && check["gated"] == true && check["state"] != "success" {
		resume = false // The image check, not a credential-only probe, owns release.
	}
	failAuth := (ownsPause || quarantine) && code == "auth_failed"
	var proxyUpdatedAt *time.Time
	if a.Proxy != nil {
		proxyUpdatedAt = &a.Proxy.UpdatedAt
	}
	result, err := r.sql.ExecContext(ctx, `WITH updated AS (
 UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||jsonb_build_object('okad',$2::jsonb),
 credentials=(CASE WHEN $13 THEN COALESCE(credentials,'{}'::jsonb)-'access_token'-'expires_at'-'_token_version' ELSE COALESCE(credentials,'{}'::jsonb) END)||$3::jsonb,
 status=CASE WHEN $4 THEN 'active' WHEN $11 THEN 'error' ELSE status END,
 schedulable=CASE WHEN $4 THEN TRUE WHEN $11 THEN FALSE ELSE schedulable END,
 error_message=CASE WHEN $4 THEN '' WHEN $5 THEN $6 WHEN $11 THEN $12 ELSE error_message END,updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND updated_at=$7 AND credentials->>'cookie'=$8
 AND proxy_id IS NOT DISTINCT FROM $9
 AND ($14::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies p WHERE p.id=accounts.proxy_id AND p.updated_at=$14))
 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload) SELECT $10,id,NULL,NULL FROM updated`, a.ID, string(raw), string(tokens), resume, failAuth, message, a.UpdatedAt, a.GetCredential("cookie"), a.ProxyID, service.SchedulerOutboxEventAccountChanged, quarantine, service.OkadVerificationPending, code == "auth_failed" || code == "identity_mismatch", proxyUpdatedAt)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if n > 0 {
		r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	}
	return n > 0, err
}

func (r *accountRepository) ListOkadInventory(ctx context.Context, after int64, limit int, since *time.Time) ([]service.OkadInventoryItem, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT a.id,COALESCE(a.credentials->>'email',a.extra->>'adobeteam_external_email',''),a.status,a.schedulable,a.proxy_id,COALESCE(a.extra->'okad','{}'::jsonb),COALESCE(r.attempts,0),r.next_attempt_at,a.updated_at,
 COALESCE(a.extra->'okad_connection_test','{}'::jsonb)||jsonb_build_object('last_check_success',a.extra->'last_check_success','last_check_status_code',a.extra->'last_check_status_code','last_check_at',a.extra->'last_check_at')
 FROM accounts a LEFT JOIN adobe_cookie_recovery_attempts r ON r.account_id=a.id
 WHERE a.deleted_at IS NULL AND a.platform='adobe' AND a.type='oauth' AND a.id>$1 AND ($3::timestamptz IS NULL OR a.updated_at>=$3)
 ORDER BY a.id LIMIT $2`, after, limit, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.OkadInventoryItem, 0)
	for rows.Next() {
		var item service.OkadInventoryItem
		var raw []byte
		var checkRaw []byte
		if err := rows.Scan(&item.ID, &item.Email, &item.Status, &item.Schedulable, &item.ProxyID, &raw, &item.Attempts, &item.NextAttemptAt, &item.UpdatedAt, &checkRaw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item.State); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(checkRaw, &item.ConnectionTest); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *accountRepository) RecordOkadRecoveryState(ctx context.Context, a *service.Account, code string, next time.Time) error {
	raw, _ := json.Marshal(map[string]any{"code": code, "next_check_at": next.UTC().Format(time.RFC3339)})
	_, err := r.sql.ExecContext(ctx, `UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||jsonb_build_object('okad_recovery',$2::jsonb),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL AND COALESCE(credentials->>'cookie','')=$3`, a.ID, string(raw), a.GetCredential("cookie"))
	return err
}
