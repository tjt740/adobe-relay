package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// A result from an older Cookie must not overwrite the newly pushed account.
func (r *accountRepository) RecordConnectionTestResultIfCurrent(ctx context.Context, a *service.Account, success bool, statusCode int) (bool, error) {
	if a.Platform != service.PlatformAdobe || a.Type != service.AccountTypeOAuth {
		err := r.RecordConnectionTestResult(ctx, a.ID, success, statusCode)
		return err == nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"last_check_success": success, "last_check_status_code": statusCode,
		"last_check_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return false, err
	}
	var proxyUpdatedAt *time.Time
	if a.Proxy != nil {
		proxyUpdatedAt = &a.Proxy.UpdatedAt
	}
	result, err := r.sql.ExecContext(ctx, `UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||$2::jsonb,updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND COALESCE(credentials->>'cookie','')=$3
 AND proxy_id IS NOT DISTINCT FROM $4
 AND ($5::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies WHERE id=accounts.proxy_id AND updated_at=$5))`, a.ID, string(payload), a.GetCredential("cookie"), a.ProxyID, proxyUpdatedAt)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

func (r *accountRepository) RecordConnectionTestResult(ctx context.Context, id int64, success bool, statusCode int) error {
	return r.UpdateExtra(ctx, id, map[string]any{
		"last_check_success":     success,
		"last_check_status_code": statusCode,
		"last_check_at":          time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// Store only the failure classification and group membership at test completion.
// This does not alter credentials, scheduling, proxy selection or recovery state.
func (r *accountRepository) RecordAdobeConnectionTestError(ctx context.Context, id int64, statusCode int, reason string, isTimeout bool) error {
	_, err := r.sql.ExecContext(ctx, `INSERT INTO adobe_account_test_errors(account_id, group_ids, status_code, reason, is_timeout)
 SELECT a.id, ARRAY(SELECT group_id FROM account_groups WHERE account_id=a.id), $2, $3, $4
 FROM accounts a WHERE a.id=$1 AND a.platform='adobe' AND a.deleted_at IS NULL`, id, statusCode, reason, isTimeout)
	return err
}
