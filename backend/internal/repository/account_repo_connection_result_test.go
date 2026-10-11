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
