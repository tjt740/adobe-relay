package repository

import (
	"context"
	"database/sql"
	"time"
)

func (r *opsRepository) ClaimAdobeAlertEmail(ctx context.Context, eventID, ruleID int64, now time.Time, cooldown time.Duration) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Serializes all incidents of this rule, including manual/worker races.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(731946,$1::integer)`, ruleID); err != nil {
		return false, err
	}
	var eligible bool
	err = tx.QueryRowContext(ctx, `SELECT status='firing' AND NOT email_sent
 AND COALESCE(dimensions->>'adobe_email_state','') NOT IN ('sending','sent')
 FROM ops_alert_events WHERE id=$1 AND rule_id=$2 FOR UPDATE`, eventID, ruleID).Scan(&eligible)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil || !eligible {
		return false, err
	}
	var recent bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ops_alert_events WHERE rule_id=$1 AND
 (NULLIF(dimensions->>'adobe_email_attempt_at','')::timestamptz > $2 OR (email_sent AND fired_at > $2)))`, ruleID, now.Add(-cooldown)).Scan(&recent)
	if err != nil || recent {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE ops_alert_events SET dimensions=COALESCE(dimensions,'{}'::jsonb) ||
 jsonb_build_object('adobe_email_state','sending','adobe_email_attempt_at',$2::text) WHERE id=$1`, eventID, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *opsRepository) FinishAdobeAlertEmail(ctx context.Context, eventID int64, sent bool) error {
	state := "failed"
	if sent {
		state = "sent"
	}
	_, err := r.db.ExecContext(ctx, `UPDATE ops_alert_events SET email_sent=email_sent OR $2,
 dimensions=COALESCE(dimensions,'{}'::jsonb) || jsonb_build_object('adobe_email_state',$3::text)
 WHERE id=$1 AND dimensions->>'adobe_email_state'='sending'`, eventID, sent, state)
	return err
}
