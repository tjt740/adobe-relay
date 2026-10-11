package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

// Reserve before contacting Okad. The unique row and conditional upsert cover
// concurrent services, processes, cookie rotations and application restarts.
func (r *accountRepository) ClaimAdobeCookieRecovery(ctx context.Context, a *service.Account, timeout time.Duration) (*service.AdobeCookieRecoveryAttempt, error) {
	claim := &service.AdobeCookieRecoveryAttempt{}
	rows, err := r.sql.QueryContext(ctx, `
 INSERT INTO adobe_cookie_recovery_attempts AS recovery
   (account_id, attempt_id, attempts, last_attempt_at, next_attempt_at)
 SELECT a.id, $1, 1, NOW(), NOW() + make_interval(secs => $2)
 FROM accounts a
 WHERE a.id = $3 AND a.deleted_at IS NULL AND a.platform = 'adobe' AND a.type = 'oauth'
   AND a.status = $4 AND COALESCE(a.error_message, '') = $5
   AND a.credentials->>'cookie' = $6 AND COALESCE(a.credentials->>'email', '') = $7
   AND a.schedulable = $8 AND a.proxy_id IS NOT DISTINCT FROM $9
   AND COALESCE(a.extra->>'adobeteam_external_email', '') = $10
   AND (a.expires_at IS NULL OR a.expires_at > NOW())
 ON CONFLICT (account_id) DO UPDATE
 SET attempt_id = EXCLUDED.attempt_id, attempts = recovery.attempts + 1,
     last_attempt_at = NOW(), next_attempt_at = EXCLUDED.next_attempt_at, finished_at = NULL
 WHERE recovery.attempts < $11 AND recovery.next_attempt_at <= NOW()
 RETURNING attempt_id, attempts`, uuid.NewString(), (timeout + service.AdobeCookieRecoveryRetryDelay).Seconds(),
		a.ID, a.Status, a.ErrorMessage, a.GetCredential("cookie"), a.GetCredential("email"), a.Schedulable,
		a.ProxyID, recoveryMarker(a), service.AdobeCookieRecoveryMaxAttempts)
	err = scanOkadRecoveryRow(rows, err, &claim.ID, &claim.Count)
	if err == nil {
		claim.Allowed = true
		return claim, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	rows, err = r.sql.QueryContext(ctx, `SELECT attempts, finished_at IS NULL AND next_attempt_at > NOW()
 FROM adobe_cookie_recovery_attempts WHERE account_id = $1`, a.ID)
	err = scanOkadRecoveryRow(rows, err, &claim.Count, &claim.InFlight)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil // account changed before the reservation
	}
	return claim, err
}

func (r *accountRepository) FinishAdobeCookieRecovery(ctx context.Context, id int64, attemptID string, success bool) error {
	delay := service.AdobeCookieRecoveryRetryDelay
	if success {
		delay = service.AdobeCookieRecoveryHealthyDelay
	}
	_, err := r.sql.ExecContext(ctx, `UPDATE adobe_cookie_recovery_attempts
 SET finished_at = NOW(), next_attempt_at = NOW() + make_interval(secs => $3)
 WHERE account_id = $1 AND attempt_id = $2`, id, attemptID, delay.Seconds())
	return err
}

// Require a real upstream success using the current cookie after 15 minutes.
// A push/clear-error alone never renews the budget; stale responses cannot do so.
func (r *accountRepository) ConfirmAdobeCookieRecovery(ctx context.Context, a *service.Account) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM adobe_cookie_recovery_attempts r USING accounts a
 WHERE r.account_id = $1 AND a.id = r.account_id AND a.deleted_at IS NULL
   AND a.platform = 'adobe' AND a.type = 'oauth' AND a.status = 'active'
   AND a.credentials->>'cookie' = $2
   AND COALESCE(a.credentials->>'email', '') = $3
   AND COALESCE(a.extra->>'adobeteam_external_email', '') = $4
   AND COALESCE(r.finished_at, r.next_attempt_at) <= NOW() - make_interval(secs => $5)`,
		a.ID, a.GetCredential("cookie"), a.GetCredential("email"), recoveryMarker(a), service.AdobeCookieRecoveryHealthyDelay.Seconds())
	return err
}

func (r *accountRepository) SetAdobeCookieRecoveryErrorIfUnchanged(ctx context.Context, a *service.Account, message string) (bool, error) {
	if a == nil || a.ErrorMessage == message || (a.Status != service.StatusError && (a.Status != service.StatusActive || !a.Schedulable)) {
		return false, nil
	}
	result, err := r.sql.ExecContext(ctx, `WITH updated AS (
 UPDATE accounts a SET status = 'error', schedulable = FALSE, error_message = $1, updated_at = NOW()
 WHERE a.id = $2 AND a.deleted_at IS NULL AND a.platform = 'adobe' AND a.type = 'oauth'
   AND a.status = $3 AND COALESCE(a.error_message, '') = $4
   AND a.credentials->>'cookie' = $5 AND COALESCE(a.credentials->>'email', '') = $6
   AND a.schedulable = $7 AND a.proxy_id IS NOT DISTINCT FROM $8
   AND COALESCE(a.extra->>'adobeteam_external_email', '') = $9
   AND (a.expires_at IS NULL OR a.expires_at > NOW())
 RETURNING a.id
 ) INSERT INTO scheduler_outbox(event_type, account_id, group_id, payload)
 SELECT $10, id, NULL, NULL FROM updated`, message, a.ID, a.Status, a.ErrorMessage,
		a.GetCredential("cookie"), a.GetCredential("email"), a.Schedulable, a.ProxyID, recoveryMarker(a), service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err == nil && n > 0 {
		r.syncSchedulerAccountSnapshotDetached(ctx, a.ID)
	}
	return n > 0, err
}

func scanOkadRecoveryRow(rows *sql.Rows, err error, dest ...any) error {
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest...)
}
