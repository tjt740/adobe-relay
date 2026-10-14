package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadInventoryHealthEvidencePostgres(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	r := &accountRepository{sql: db}
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,updated_at) VALUES
 (1,'adobe','oauth','inactive',false,'{"email":"one@example.test","cookie":"private-cookie"}','{}',NOW()),
 (2,'adobe','oauth','active',true,'{"email":"two@example.test"}','{}',NOW()),
 (3,'adobe','oauth','active',true,'{"email":"three@example.test"}','{}',NOW());
 INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code,reason,source) VALUES
 (1,'model-a',NOW()-INTERVAL '2 days',408,'timeout','business'),
 (1,'model-b',NOW()-INTERVAL '1 day',401,'auth','test');`)
	require.NoError(t, err)
	ctx := context.Background()
	read := func() []service.OkadInventoryItem {
		items, err := r.ListOkadInventory(ctx, 0, 200, nil)
		require.NoError(t, err)
		return items
	}
	items := read()
	require.Len(t, items, 3)
	require.Equal(t, "error", items[0].AccountHealth.State)
	require.Equal(t, 401, items[0].AccountHealth.Errors[0].Code)
	require.Equal(t, 408, items[0].AccountHealth.Errors[1].Code)
	require.Equal(t, "unknown", items[1].AccountHealth.State)
	require.False(t, items[0].Schedulable, "the projection must preserve manual pause")
	// Updating a Cookie or succeeding on another model cannot clear old failures.
	_, err = db.Exec(`UPDATE accounts SET credentials=credentials||'{"cookie":"replacement-private"}' WHERE id=1;
 UPDATE adobe_account_health SET success_at=NOW(),success_started_at=NOW() WHERE account_id=1 AND scope='model-b'`)
	require.NoError(t, err)
	items = read()
	require.Len(t, items[0].AccountHealth.Errors, 1)
	require.Equal(t, 408, items[0].AccountHealth.Errors[0].Code)
	_, err = db.Exec(`UPDATE adobe_account_health SET success_at=NOW(),success_started_at=NOW() WHERE account_id=1;
 UPDATE accounts SET temp_unschedulable_until=NOW()+INTERVAL '1 minute',temp_unschedulable_reason='{"status_code":502,"message":"private"}' WHERE id=2;
 UPDATE accounts SET rate_limit_reset_at=NOW()+INTERVAL '1 minute' WHERE id=3`)
	require.NoError(t, err)
	items = read()
	require.Equal(t, "recovered", items[0].AccountHealth.State)
	require.Equal(t, 502, items[1].AccountHealth.Errors[0].Code)
	require.Equal(t, 429, items[2].AccountHealth.Errors[0].Code)
	raw, err := json.Marshal(items)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private")
	page, err := r.ListOkadInventory(ctx, 1, 1, nil)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.EqualValues(t, 2, page[0].ID)
	future := time.Now().Add(time.Hour)
	page, err = r.ListOkadInventory(ctx, 0, 200, &future)
	require.NoError(t, err)
	require.Empty(t, page)
}
