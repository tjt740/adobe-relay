package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestAccountConnectionTestPostgresPreservesAccountConfiguration(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := &accountRepository{sql: db, client: client}
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,updated_at)
 VALUES(1,'adobe','oauth','active',false,'{"cookie":"fixture-cookie"}','{"existing_setting":true}',NOW())`)
	require.NoError(t, err)
	for _, success := range []bool{false, true} {
		code := 401
		if success {
			code = 200
		}
		require.NoError(t, repo.RecordConnectionTestResult(context.Background(), 1, success, code))
		var status, cookie string
		var schedulable bool
		var raw []byte
		require.NoError(t, db.QueryRow(`SELECT status,schedulable,credentials->>'cookie',extra FROM accounts WHERE id=1`).Scan(&status, &schedulable, &cookie, &raw))
		var extra map[string]any
		require.NoError(t, json.Unmarshal(raw, &extra))
		require.Equal(t, success, extra["last_check_success"])
		require.Equal(t, float64(code), extra["last_check_status_code"])
		_, err := time.Parse(time.RFC3339Nano, extra["last_check_at"].(string))
		require.NoError(t, err)
		require.Equal(t, true, extra["existing_setting"])
		require.Equal(t, "fixture-cookie", cookie)
		require.Equal(t, "active", status)
		require.False(t, schedulable, "recording a test must not unpause the account")
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduler_outbox`).Scan(&count))
	require.Zero(t, count, "observational results should not rebuild scheduler buckets")
}

func TestAccountConnectionTestPostgresAlertHistorySurvivesRecovery(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := &accountRepository{sql: db, client: client}
	_, err := db.Exec(`INSERT INTO accounts(id,platform,status,schedulable,credentials,extra,proxy_id) VALUES
 (1,'adobe','active',false,'{"cookie":"keep"}','{}',9), (2,'openai','active',true,'{}','{}',NULL),
 (3,'adobe','active',true,'{}','{}',NULL);
 UPDATE accounts SET deleted_at=NOW() WHERE id=3;
 INSERT INTO account_groups VALUES(1,7),(1,8);`)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, repo.RecordConnectionTestResult(ctx, 1, false, 408))
	require.NoError(t, repo.RecordAdobeConnectionTestError(ctx, 1, 408, "adobe_upstream", false))
	require.NoError(t, repo.RecordAdobeConnectionTestError(ctx, 1, 408, "", true))
	require.NoError(t, repo.RecordConnectionTestResult(ctx, 1, true, 200))
	for _, id := range []int64{2, 3, 99} {
		require.NoError(t, repo.RecordAdobeConnectionTestError(ctx, id, 401, "", false))
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.Error(t, repo.RecordAdobeConnectionTestError(canceled, 1, 401, "", false))
	var total, timeouts, groups, proxy, outbox int
	var cookie string
	var paused, success bool
	require.NoError(t, db.QueryRow(`SELECT count(*),count(*) FILTER(WHERE is_timeout),count(*) FILTER(WHERE group_ids=ARRAY[7,8]::bigint[]) FROM adobe_account_test_errors`).Scan(&total, &timeouts, &groups))
	require.Equal(t, 2, total)
	require.Equal(t, 1, timeouts)
	require.Equal(t, 2, groups)
	require.NoError(t, db.QueryRow(`SELECT NOT schedulable,proxy_id,credentials->>'cookie',(extra->>'last_check_success')::bool FROM accounts WHERE id=1`).Scan(&paused, &proxy, &cookie, &success))
	require.True(t, paused)
	require.True(t, success)
	require.Equal(t, 9, proxy)
	require.Equal(t, "keep", cookie)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scheduler_outbox`).Scan(&outbox))
	require.Zero(t, outbox)
}
