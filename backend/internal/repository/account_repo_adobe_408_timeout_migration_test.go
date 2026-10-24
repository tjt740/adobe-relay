package repository

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestAdobe408TimeoutMigrationReturnsOnlyOwnedPauses(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,updated_at)
 SELECT i,'adobe','oauth','active',false,'{"cookie":"keep-cookie","access_token":"keep-token"}',
 '{"custom":"keep","adobe_proxy_408":{"managed":true}}',NOW() FROM generate_series(1,15)i;
 INSERT INTO account_groups SELECT i,7 FROM generate_series(1,15)i;
 INSERT INTO adobe_proxy_408_holds SELECT i,i<>3 FROM generate_series(1,11)i;
 INSERT INTO adobe_proxy_408_blocks SELECT i,'gpt-image-2',NOW() FROM generate_series(1,11)i;
 UPDATE accounts SET extra=extra||'{"okad_manual_pause":true}' WHERE id=2;
 UPDATE accounts SET status='inactive' WHERE id=4;
 UPDATE accounts SET expires_at=NOW()-INTERVAL '1 day' WHERE id=5;
 UPDATE accounts SET status='error',error_message='Authentication failed (401): bad' WHERE id=6;
 UPDATE accounts SET status='error',error_message='Adobe credits exhausted' WHERE id=7;
 UPDATE accounts SET extra=extra||'{"okad_connection_test":{"gated":true,"state":"pending","require_verification":true},"okad":{"verification":"pending"}}' WHERE id=8;
 UPDATE accounts SET status='error',proxy_auto_paused=true,error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）' WHERE id=9;
 INSERT INTO adobe_proxy_test_leases(account_id,owner,proxy_id,proxy_updated_at,credential_hash,expires_at) VALUES(10,'keep-owner',1,NOW(),'keep-hash',NOW()+INTERVAL '5 minutes');
 UPDATE accounts SET extra=extra||'{"manual_proxy_enabled":true}' WHERE id=11;
 UPDATE accounts SET status='error',error_message='Okad credentials pending connection test',extra=extra||jsonb_build_object(
 'last_check_status_code',408,'last_check_success',false,'last_check_at',NOW(),
 'okad',jsonb_build_object('verification','verified'),
 'okad_connection_test',jsonb_build_object('gated',true,'state','failed','resume',true,'require_verification',true,
 'started_at',NOW()-INTERVAL '1 minute','key',encode(sha256(convert_to('keep-cookie','UTF8')),'hex'))) WHERE id>=12;
 UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test,key}','"stale-cookie-key"') WHERE id=12;
 UPDATE accounts SET extra=jsonb_set(extra,'{okad,verification}','"pending"') WHERE id=14;
 UPDATE accounts SET extra=jsonb_set(extra,'{last_check_status_code}','401') WHERE id=15;
 INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code) SELECT i,'gpt-image-2',NOW(),408 FROM generate_series(1,15)i;`)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../migrations/254_adobe_408_timeout_scheduling.sql")
	require.NoError(t, err)
	apply := func() {
		tx, e := db.BeginTx(context.Background(), nil)
		require.NoError(t, e)
		defer tx.Rollback()
		_, e = tx.Exec(string(raw))
		require.NoError(t, e)
		require.NoError(t, tx.Commit())
	}
	apply()
	var active []int64
	rows, err := db.Query(`SELECT id FROM accounts WHERE schedulable ORDER BY id`)
	require.NoError(t, err)
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		active = append(active, id)
	}
	require.NoError(t, rows.Close())
	require.Equal(t, []int64{1, 9, 11, 13}, active)
	var count int
	for _, query := range []string{`SELECT count(*) FROM accounts WHERE credentials='{"cookie":"keep-cookie","access_token":"keep-token"}'::jsonb AND extra->>'custom'='keep'`, `SELECT count(*) FROM account_groups`, `SELECT count(*) FROM adobe_account_health WHERE status_code=408 AND success_started_at IS NULL`} {
		require.NoError(t, db.QueryRow(query).Scan(&count))
		require.Equal(t, 15, count)
	}
	for _, table := range []string{"adobe_proxy_408_blocks", "adobe_proxy_408_holds"} {
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM `+table).Scan(&count))
		require.Zero(t, count)
	}
	var resume bool
	var owner, hash string
	require.NoError(t, db.QueryRow(`SELECT owner,credential_hash,return_schedulable FROM adobe_proxy_test_leases WHERE account_id=10`).Scan(&owner, &hash, &resume))
	require.Equal(t, "keep-owner", owner)
	require.Equal(t, "keep-hash", hash)
	require.True(t, resume)
	_, err = db.Exec(`UPDATE accounts SET schedulable=false WHERE id=1`)
	require.NoError(t, err)
	apply()
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&resume))
	require.False(t, resume, "re-running cannot reclaim a later operator pause")
}
