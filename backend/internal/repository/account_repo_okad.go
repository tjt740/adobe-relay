package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// RecoverAdobeCookieIfUnchanged replaces only the rejected cookie and its token
// fields. A concurrent cookie/identity/status/proxy edit wins over this recovery.
// State and scheduler invalidation are committed together.
func (r *accountRepository) RecoverAdobeCookieIfUnchanged(ctx context.Context, a *service.Account, cookie string) (bool, error) {
	return r.StageOkadCookie(ctx, a, cookie)
}

// Credential replacement, quarantine, and the durable check are one commit.
// A duplicate callback for the same Cookie cannot replay the test or resume it.
func (r *accountRepository) StageOkadCookie(ctx context.Context, a *service.Account, cookie string) (bool, error) {
	if r == nil || r.sql == nil || a == nil || strings.TrimSpace(cookie) == "" {
		return false, errors.New("invalid adobe recovery update")
	}
	resume := service.OkadConnectionTestMayResume(a)
	email := a.GetCredential("email")
	if email == "" {
		email = recoveryMarker(a)
	}
	if email == "" {
		email = strings.TrimSpace(a.Name)
	}
	snapshot := *a
	snapshot.Credentials = service.MergeCredentials(a.Credentials, map[string]any{"cookie": cookie})
	state, err := json.Marshal(service.NewOkadConnectionTestState(&snapshot, false, resume))
	if err != nil {
		return false, err
	}
	result, err := r.sql.ExecContext(ctx, `
 WITH updated AS (
  UPDATE accounts AS a
  SET credentials = (COALESCE(a.credentials, '{}'::jsonb) - 'access_token' - 'expires_at' - '_token_version' - 'adobe_session_cookie' - 'adobe_session_cookie_source_hash') || jsonb_build_object('cookie', $1::text,'email',$16::text),
      extra = (COALESCE(a.extra,'{}'::jsonb)-'last_check_at'-'last_check_success'-'last_check_status_code') || jsonb_build_object('okad_connection_test',$11::jsonb,'adobeteam_external_email',$16::text),
      status = CASE WHEN $12 THEN 'error' ELSE a.status END,
      error_message = CASE WHEN $12 THEN $13 ELSE a.error_message END,
      schedulable = FALSE,
      updated_at = NOW()
  WHERE a.id = $2 AND a.deleted_at IS NULL AND a.platform = 'adobe' AND a.type = 'oauth'
    AND a.status = $3
    AND COALESCE(a.error_message, '') = $4
    AND COALESCE(a.credentials->>'cookie','') = $5
    AND COALESCE(a.credentials->>'email', '') = $6
    AND a.schedulable = $7
    AND a.proxy_id IS NOT DISTINCT FROM $8
    AND COALESCE(a.extra->>'adobeteam_external_email', '') = $9
    AND (a.expires_at IS NULL OR a.expires_at > NOW())
    AND COALESCE(a.extra->>'okad_manual_pause','false') = $14
    AND NOT (a.credentials->>'cookie'=$1 AND COALESCE(a.extra->'okad_connection_test'->>'key','')=$15)
  RETURNING a.id
 )
 INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
 SELECT $10, updated.id, NULL, NULL FROM updated`, cookie, a.ID, a.Status, a.ErrorMessage,
		a.GetCredential("cookie"), a.GetCredential("email"), a.Schedulable, a.ProxyID, recoveryMarker(a), service.SchedulerOutboxEventAccountChanged,
		string(state), resume, service.OkadConnectionTestPending, fmtBool(a.Extra["okad_manual_pause"] == true), service.OkadConnectionTestKey(&snapshot), email)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	return true, nil
}

func fmtBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func recoveryMarker(a *service.Account) string {
	marker, _ := a.Extra["adobeteam_external_email"].(string)
	return marker
}

// QueueAdobeCookieRecoveryIfUnchanged preserves concurrent manual pauses and
// successful pushes when a request-side login fails or is cancelled.
func (r *accountRepository) QueueAdobeCookieRecoveryIfUnchanged(ctx context.Context, a *service.Account) (bool, error) {
	if r == nil || r.sql == nil || a == nil || a.Platform != service.PlatformAdobe ||
		a.Type != service.AccountTypeOAuth || a.Status != service.StatusActive || !a.Schedulable {
		return false, nil
	}
	result, err := r.sql.ExecContext(ctx, `
 WITH updated AS (
  UPDATE accounts AS a
  SET status = 'error', schedulable = FALSE,
      error_message = 'Authentication failed (401): Okad recovery pending', updated_at = NOW()
  WHERE a.id = $1 AND a.deleted_at IS NULL AND a.platform = 'adobe' AND a.type = 'oauth'
    AND a.status = 'active' AND a.schedulable = TRUE
    AND COALESCE(a.error_message, '') = $2
    AND a.credentials->>'cookie' = $3
    AND COALESCE(a.credentials->>'email', '') = $4
    AND a.proxy_id IS NOT DISTINCT FROM $5
    AND COALESCE(a.extra->>'adobeteam_external_email', '') = $6
    AND (a.expires_at IS NULL OR a.expires_at > NOW())
  RETURNING a.id
 )
 INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
 SELECT $7, id, NULL, NULL FROM updated`, a.ID, a.ErrorMessage, a.GetCredential("cookie"),
		a.GetCredential("email"), a.ProxyID, recoveryMarker(a), service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	return true, nil
}
