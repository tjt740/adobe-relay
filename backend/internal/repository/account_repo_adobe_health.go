package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) RecordAdobeHealth(ctx context.Context, a *service.Account, o service.AdobeHealthObservation) error {
	beginner, ok := r.sql.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return errors.New("health telemetry transaction unavailable")
	}
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	// Serialize observations with credential writes. A result from an old Cookie
	// cannot bless its replacement or erase its failures.
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id=$1 AND platform='adobe' AND deleted_at IS NULL
 AND COALESCE(credentials->>'cookie','')=$2 AND COALESCE(credentials->>'api_key','')=$3 FOR UPDATE`, a.ID, a.GetCredential("cookie"), a.GetCredential("api_key")).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.Success {
		_, err = tx.ExecContext(ctx, `INSERT INTO adobe_account_health(account_id,scope,success_at,success_started_at,success_source)
 VALUES($1,$2,$3,$4,$5) ON CONFLICT(account_id,scope) DO UPDATE SET
 success_at=EXCLUDED.success_at,success_started_at=EXCLUDED.success_started_at,success_source=EXCLUDED.success_source,updated_at=NOW()
 WHERE adobe_account_health.success_started_at IS NULL OR adobe_account_health.success_started_at<EXCLUDED.success_started_at`, id, o.Scope, o.CheckedAt, o.StartedAt, o.Source)
		if err != nil {
			return err
		}
		// An actual generation proves account-wide authentication. Model-specific
		// failures need success on that model; an older in-flight success never wins.
		_, err = tx.ExecContext(ctx, `UPDATE adobe_account_health SET success_at=$2,success_started_at=$3,success_source=$4,updated_at=NOW()
 WHERE account_id=$1 AND (scope='' OR reason='auth') AND failed_at<$3
 AND (success_started_at IS NULL OR success_started_at<$3)`, id, o.CheckedAt, o.StartedAt, o.Source)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code,reason,stage,source,alert_failed_at)
 VALUES($1,$2,$3::timestamptz,$4::integer,$5,$6,$7,CASE WHEN $4::integer IN (401,408) THEN $3::timestamptz ELSE NULL END) ON CONFLICT(account_id,scope) DO UPDATE SET
 failed_at=EXCLUDED.failed_at,alert_failed_at=COALESCE(EXCLUDED.alert_failed_at,adobe_account_health.alert_failed_at),status_code=EXCLUDED.status_code,reason=EXCLUDED.reason,stage=EXCLUDED.stage,source=EXCLUDED.source,updated_at=NOW()
 WHERE adobe_account_health.failed_at IS NULL OR adobe_account_health.failed_at<EXCLUDED.failed_at`, id, o.Scope, o.CheckedAt, o.Code, o.Reason, o.Stage, o.Source)
	}
	if err != nil {
		return err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return err
	}
	// Keep the latest observation and an independent successful evidence record.
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||jsonb_build_object('adobe_health',
 COALESCE(extra->'adobe_health','{}'::jsonb)||jsonb_build_object('latest',$2::jsonb,'pending',
 EXISTS(SELECT 1 FROM adobe_account_health h WHERE h.account_id=$1 AND h.failed_at IS NOT NULL AND (h.success_started_at IS NULL OR h.success_started_at<=h.failed_at)))
 ||CASE WHEN $3 THEN jsonb_build_object('last_success',$2::jsonb) ELSE jsonb_build_object('last_failure',$2::jsonb) END),updated_at=NOW()
 WHERE id=$1`, id, string(raw), o.Success)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *accountRepository) AccountRuntimeRevision(ctx context.Context) (string, error) {
	var revision string
	reader, ok := r.sql.(interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	})
	if !ok {
		return "", errors.New("revision reader unavailable")
	}
	err := reader.QueryRowContext(ctx, `SELECT MD5(COALESCE(string_agg(id::text||':'||COALESCE(updated_at::text,''),',' ORDER BY id),'')) FROM accounts`).Scan(&revision)
	return revision, err
}
