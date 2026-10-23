package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/proxyfailover"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Older observations used public aliases, newer ones store the resolved Adobe
// family. Match only aliases the existing catalog proves are the same model.
func adobeHealthMatchingScopes(ctx context.Context, tx *sql.Tx, id int64, scope string) ([]string, error) {
	matching := []string{scope, ""}
	if scope == "" {
		return matching, nil
	}
	model, resolveErr := adobe.ResolveImage(adobe.ImageRequest{ModelID: scope})
	if resolveErr != nil {
		return matching, nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT scope FROM adobe_account_health WHERE account_id=$1
 UNION SELECT scope FROM adobe_proxy_408_blocks WHERE account_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var candidate string
		if err = rows.Scan(&candidate); err != nil {
			return nil, err
		}
		if candidate != "" {
			if other, e := adobe.ResolveImage(adobe.ImageRequest{ModelID: candidate}); e == nil && other.Family == model.Family {
				matching = append(matching, candidate)
			}
		}
	}
	return matching, rows.Err()
}

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
	// Take the table intent lock before row locks, matching the allocator order.
	if _, err = tx.ExecContext(ctx, `LOCK TABLE accounts IN ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	var id int64
	var proxyUpdatedAt *time.Time
	if a.Proxy != nil {
		proxyUpdatedAt = &a.Proxy.UpdatedAt
	}
	// Serialize observations with credential writes. A result from an old Cookie
	// cannot bless its replacement or erase its failures.
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id=$1 AND platform='adobe' AND deleted_at IS NULL
 AND COALESCE(credentials->>'cookie','')=$2 AND COALESCE(credentials->>'api_key','')=$3
 AND proxy_id IS NOT DISTINCT FROM $4
 AND ($5::timestamptz IS NULL OR EXISTS(SELECT 1 FROM proxies WHERE id=accounts.proxy_id AND updated_at=$5)) FOR UPDATE`, a.ID, a.GetCredential("cookie"), a.GetCredential("api_key"), a.ProxyID, proxyUpdatedAt).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	o.ProxyID = a.ProxyID
	o.ProxyUpdatedAt = proxyUpdatedAt
	if a.ProxyID != nil && proxyUpdatedAt != nil && !service.IsAdobeRelayAccount(a) {
		o.SourceID, err = proxyfailover.RecordGeneration(ctx, tx, *a.ProxyID, *proxyUpdatedAt, o.SourceID, id, a.GetCredential("cookie"), a.GetCredential("api_key"), o.Scope, o.Success, o.Code, o.Stage, o.SubmissionRejected, o.StartedAt, o.CheckedAt)
		if err != nil {
			return err
		}
	}
	var matchingScopes []string
	if o.Success || o.Code == 408 {
		matchingScopes, err = adobeHealthMatchingScopes(ctx, tx, id, o.Scope)
		if err != nil {
			return err
		}
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
 WHERE account_id=$1 AND (scope=ANY($5) OR reason='auth') AND failed_at<$3
 AND (success_started_at IS NULL OR success_started_at<$3)`, id, o.CheckedAt, o.StartedAt, o.Source, pq.Array(matchingScopes))
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code,reason,stage,source,alert_failed_at)
 VALUES($1,$2,$3::timestamptz,$4::integer,$5,$6,$7,CASE WHEN $4::integer IN (401,408) THEN $3::timestamptz ELSE NULL END) ON CONFLICT(account_id,scope) DO UPDATE SET
 failed_at=EXCLUDED.failed_at,alert_failed_at=COALESCE(EXCLUDED.alert_failed_at,adobe_account_health.alert_failed_at),status_code=EXCLUDED.status_code,reason=EXCLUDED.reason,stage=EXCLUDED.stage,source=EXCLUDED.source,updated_at=NOW()
 WHERE adobe_account_health.failed_at IS NULL OR adobe_account_health.failed_at<EXCLUDED.failed_at`, id, o.Scope, o.CheckedAt, o.Code, o.Reason, o.Stage, o.Source)
	}
	if err != nil {
		return err
	}
	if o.Success {
		_, err = tx.ExecContext(ctx, `DELETE FROM adobe_proxy_408_blocks b WHERE account_id=$1 AND scope=ANY($2) AND failed_at<$3
 AND NOT EXISTS(SELECT 1 FROM adobe_account_health h WHERE h.account_id=b.account_id AND h.scope=b.scope AND h.failed_at>=$3)`, a.ID, pq.Array(matchingScopes), o.StartedAt)
	} else if o.Code == 408 {
		// A delayed old alias must not recreate an incident after a newer
		// successful request already proved that same physical model healthy.
		_, err = tx.ExecContext(ctx, `WITH proof AS (
 SELECT success_at,success_started_at,success_source FROM adobe_account_health
 WHERE account_id=$1 AND scope=ANY($4) AND (scope<>'' OR $2='') AND success_started_at>$3
 ORDER BY success_started_at DESC LIMIT 1)
 UPDATE adobe_account_health h SET success_at=p.success_at,success_started_at=p.success_started_at,success_source=p.success_source,updated_at=NOW()
 FROM proof p WHERE h.account_id=$1 AND h.scope=$2 AND h.failed_at<p.success_started_at
 AND (h.success_started_at IS NULL OR h.success_started_at<p.success_started_at)`, a.ID, o.Scope, o.CheckedAt, pq.Array(matchingScopes))
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO adobe_proxy_408_blocks(account_id,scope,failed_at)
 SELECT $1,$2,$3 WHERE EXISTS(SELECT 1 FROM adobe_account_health WHERE account_id=$1 AND scope=$2
 AND failed_at=$3 AND (success_started_at IS NULL OR success_started_at<=failed_at))
 ON CONFLICT(account_id,scope) DO UPDATE SET failed_at=EXCLUDED.failed_at WHERE adobe_proxy_408_blocks.failed_at<EXCLUDED.failed_at`, a.ID, o.Scope, o.CheckedAt)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO adobe_proxy_408_holds(account_id,resume_schedulable)
 SELECT a.id,COALESCE(a.schedulable,false) OR COALESCE(a.proxy_auto_paused AND a.error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）',false) FROM accounts a WHERE a.id=$1
 AND EXISTS(SELECT 1 FROM proxy_auto_allocation WHERE enabled)
 AND EXISTS(SELECT 1 FROM adobe_proxy_408_blocks b WHERE b.account_id=a.id)
 ON CONFLICT(account_id) DO UPDATE SET resume_schedulable=adobe_proxy_408_holds.resume_schedulable OR EXCLUDED.resume_schedulable`, a.ID)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `WITH changed AS (UPDATE accounts SET schedulable=false,updated_at=NOW() WHERE id=$1 AND schedulable
 AND EXISTS(SELECT 1 FROM proxy_auto_allocation WHERE enabled)
 AND EXISTS(SELECT 1 FROM adobe_proxy_408_holds WHERE account_id=$1) RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id,payload) SELECT 'account_changed',id,NULL FROM changed`, a.ID)
		}
	}
	if err != nil {
		return err
	}
	if err = proxyfailover.Refresh408Metadata(ctx, tx, a.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return err
	}
	// Keep the latest observation and an independent successful evidence record.
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)||jsonb_build_object('adobe_health',
 COALESCE(extra->'adobe_health','{}'::jsonb)||jsonb_build_object('latest',CASE WHEN (extra->'adobe_health'->'latest'->>'checked_at')::timestamptz>$4 THEN extra->'adobe_health'->'latest' ELSE $2::jsonb END,'pending',
 EXISTS(SELECT 1 FROM adobe_account_health h WHERE h.account_id=$1 AND h.failed_at IS NOT NULL AND (h.success_started_at IS NULL OR h.success_started_at<=h.failed_at)))
 ||CASE WHEN $3 THEN CASE WHEN (extra->'adobe_health'->'last_success'->>'checked_at')::timestamptz>$4 THEN '{}'::jsonb ELSE jsonb_build_object('last_success',$2::jsonb) END
 ELSE CASE WHEN (extra->'adobe_health'->'last_failure'->>'checked_at')::timestamptz>$4 THEN '{}'::jsonb ELSE jsonb_build_object('last_failure',$2::jsonb) END END),updated_at=NOW()
 WHERE id=$1`, id, string(raw), o.Success, o.CheckedAt)
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
