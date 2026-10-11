package repository

import (
	"context"
	"time"
)

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
