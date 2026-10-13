package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) QueueOkadConnectionTest(ctx context.Context, a *service.Account, key string, requireVerification bool) error {
	_, err := r.sql.ExecContext(ctx, `UPDATE accounts SET
 extra=COALESCE(extra,'{}'::jsonb)||jsonb_build_object('okad_connection_test',
 jsonb_build_object('key',$2::text,'state','pending','require_verification',$4::boolean,'queued_at',NOW())), updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND credentials->>'cookie'=$3 AND COALESCE(extra->'okad_connection_test'->>'key','')<>$2`, a.ID, key, a.GetCredential("cookie"), requireVerification)
	return err
}

func (r *accountRepository) ClaimOkadConnectionTest(ctx context.Context) (*service.OkadConnectionTestJob, error) {
	job := &service.OkadConnectionTestJob{}
	rows, err := r.sql.QueryContext(ctx, `WITH candidate AS (
 SELECT id FROM accounts WHERE deleted_at IS NULL AND platform='adobe' AND type='oauth'
 AND COALESCE(credentials->>'cookie','')<>''
 AND extra->'okad_connection_test'->>'state'='pending'
 AND extra->'okad_connection_test'->>'key'=encode(sha256(convert_to(credentials->>'cookie','UTF8')),'hex')
 AND (extra->'okad_connection_test'->>'require_verification'='false' OR extra->'okad'->>'verification'='verified')
 ORDER BY extra->'okad_connection_test'->>'queued_at',id FOR UPDATE SKIP LOCKED LIMIT 1)
 UPDATE accounts a SET extra=jsonb_set(a.extra,'{okad_connection_test}',a.extra->'okad_connection_test'||
 jsonb_build_object('state','running','started_at',NOW())),updated_at=NOW()
 FROM candidate WHERE a.id=candidate.id RETURNING a.id,a.extra->'okad_connection_test'->>'key'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	if err := rows.Scan(&job.AccountID, &job.Key); err != nil {
		return nil, err
	}
	return job, rows.Err()
}

func (r *accountRepository) FinishOkadConnectionTest(ctx context.Context, job service.OkadConnectionTestJob, state string) error {
	_, err := r.sql.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test}',
 extra->'okad_connection_test'||jsonb_build_object('state',$3::text,'finished_at',NOW())),updated_at=NOW()
 WHERE id=$1 AND deleted_at IS NULL AND extra->'okad_connection_test'->>'key'=$2
 AND encode(sha256(convert_to(COALESCE(credentials->>'cookie',''),'UTF8')),'hex')=$2
 AND extra->'okad_connection_test'->>'state'='running'`, job.AccountID, job.Key, state)
	return err
}
