package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// RecoverAdobeCookieIfUnchanged replaces only the rejected cookie and its token
// fields. A concurrent cookie/identity/status/proxy edit wins over this recovery.
// State and scheduler invalidation are committed together.
func (r *accountRepository) RecoverAdobeCookieIfUnchanged(ctx context.Context, a *service.Account, cookie string) (bool, error) {
	if r == nil || r.sql == nil || a == nil || strings.TrimSpace(cookie) == "" {
		return false, errors.New("invalid adobe recovery update")
	}
	result, err := r.sql.ExecContext(ctx, `
 WITH updated AS (
  UPDATE accounts AS a
  SET credentials = (COALESCE(a.credentials, '{}'::jsonb) - 'access_token' - 'expires_at' - '_token_version') || jsonb_build_object('cookie', $1::text),
      status = 'active', error_message = NULL,
      schedulable = CASE WHEN a.status = 'error' THEN TRUE ELSE a.schedulable END,
      updated_at = NOW()
  WHERE a.id = $2 AND a.deleted_at IS NULL AND a.platform = 'adobe' AND a.type = 'oauth'
    AND a.status IN ('active', 'error') AND a.status = $3
    AND COALESCE(a.error_message, '') = $4
    AND a.credentials->>'cookie' = $5
    AND COALESCE(a.credentials->>'email', '') = $6
    AND a.schedulable = $7
    AND a.proxy_id IS NOT DISTINCT FROM $8
    AND COALESCE(a.extra->>'adobeteam_external_email', '') = $9
    AND (a.expires_at IS NULL OR a.expires_at > NOW())
  RETURNING a.id
 )
 INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
 SELECT $10, updated.id, NULL, NULL FROM updated`, cookie, a.ID, a.Status, a.ErrorMessage,
		a.GetCredential("cookie"), a.GetCredential("email"), a.Schedulable, a.ProxyID, recoveryMarker(a), service.SchedulerOutboxEventAccountChanged)
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

func recoveryMarker(a *service.Account) string {
	marker, _ := a.Extra["adobeteam_external_email"].(string)
	return marker
}
