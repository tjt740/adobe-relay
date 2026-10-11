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
