package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadMetadataPostgresPreservesBusinessAndChecksIdentity(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,proxy_id,updated_at)
 SELECT id,'adobe','oauth','error','HTTP 408',false,'{"email":"a@outlook.com","cookie":"keep-cookie","access_token":"keep-token"}',
 '{"adobeteam_external_email":"a@outlook.com","okad_manual_pause":true,"last_check_status_code":408}',788,NOW() FROM generate_series(1,9) id;
 UPDATE accounts SET extra=extra||'{"okad":{"issuer":"site","source_key":"external:7","version":5,"verification":"verified"}}' WHERE id=2;
 UPDATE accounts SET extra=extra||'{"okad":{"issuer":"foreign","source_key":"external:7"}}' WHERE id=3;
 UPDATE accounts SET credentials=credentials||'{"email":"different@outlook.com"}' WHERE id=4;
 UPDATE accounts SET deleted_at=NOW() WHERE id=5;
 UPDATE accounts SET platform='openai' WHERE id=6;
 UPDATE accounts SET extra=extra-'adobeteam_external_email' WHERE id=7;
 UPDATE accounts SET extra=extra||'{"okad_account":{"issuer":"foreign","source_key":"external:7"}}' WHERE id=8;
 UPDATE accounts SET type='apikey' WHERE id=9;`)
	require.NoError(t, err)
	snapshot := func() string {
		var v string
		require.NoError(t, db.QueryRow(`SELECT jsonb_agg(to_jsonb(a)-'updated_at'-'extra'||jsonb_build_object('extra',extra-'okad_account') ORDER BY id)::text FROM accounts a`).Scan(&v))
		return v
	}
	before := snapshot()
	yes := true
	items := []service.OkadAccountMetadataUpdate{}
	for id := int64(1); id <= 9; id++ {
		items = append(items, service.OkadAccountMetadataUpdate{ID: id, OkadAccountMetadata: service.OkadAccountMetadata{Issuer: "site", SourceKey: "external:7", Email: "a@outlook.com", IsCompany: &yes}})
	}
	n, err := repo.SyncOkadAccountMetadata(context.Background(), items)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	require.Equal(t, before, snapshot(), "only classification metadata may change")
	n, err = repo.SyncOkadAccountMetadata(context.Background(), items)
	require.NoError(t, err)
	require.Zero(t, n, "unchanged metadata is idempotent")
	inventory, err := repo.ListOkadInventory(context.Background(), 0, 200, nil)
	require.NoError(t, err)
	require.Equal(t, "a@outlook.com", inventory[0].ImportedEmail)
	require.Equal(t, true, inventory[0].AccountMetadata["is_company"])
	no := false
	items[0].IsCompany = &no
	n, err = repo.SyncOkadAccountMetadata(context.Background(), items[:1])
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT extra FROM accounts WHERE id=1`).Scan(&raw))
	var extra map[string]any
	require.NoError(t, json.Unmarshal(raw, &extra))
	require.Equal(t, false, extra["okad_account"].(map[string]any)["is_company"])
	var jobs int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduler_outbox`).Scan(&jobs))
	require.Zero(t, jobs, "no credential tests, recovery or scheduling jobs")
	require.Equal(t, before, snapshot())
}
