package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadRecoveryPostgres(t *testing.T) {
	dsn := os.Getenv("OKAD_RECOVERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OKAD_RECOVERY_TEST_DATABASE_URL to a disposable PostgreSQL instance")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer root.Close()
	schema := fmt.Sprintf("okad_recovery_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	defer root.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE accounts(id bigint PRIMARY KEY,platform text,type text,status text,error_message text,schedulable bool,credentials jsonb,extra jsonb,proxy_id bigint,expires_at timestamptz,deleted_at timestamptz,updated_at timestamptz);
 CREATE TABLE scheduler_outbox(event_type text,account_id bigint,group_id bigint,payload jsonb);`)
	require.NoError(t, err)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	message := "OAuth 401 (no refresh_token): Invalid bearer token"
	reset := func(status, message string, schedulable bool) {
		_, err = db.Exec(`TRUNCATE accounts, scheduler_outbox;
 INSERT INTO accounts VALUES(1,'adobe','oauth','error','',false,'{"cookie":"old","email":"test@example.test","access_token":"rejected","model_mapping":{"gpt-image-2":"keep"}}','{}',NULL,NULL,NULL,NOW())`)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE accounts SET status=$1,error_message=$2,schedulable=$3", status, message, schedulable)
		require.NoError(t, err)
	}
	snapshot := &service.Account{ID: 1, Status: service.StatusError, ErrorMessage: message, Credentials: map[string]any{"cookie": "old", "email": "test@example.test"}}
	reset(service.StatusError, message, false)
	applied, err := repo.RecoverAdobeCookieIfUnchanged(ctx, snapshot, "fresh")
	require.NoError(t, err)
	require.True(t, applied)
	var status string
	var schedulable bool
	var raw []byte
	require.NoError(t, db.QueryRow("SELECT status,schedulable,credentials FROM accounts WHERE id=1").Scan(&status, &schedulable, &raw))
	require.Equal(t, service.StatusActive, status)
	require.True(t, schedulable)
	var credentials map[string]any
	require.NoError(t, json.Unmarshal(raw, &credentials))
	require.Equal(t, "fresh", credentials["cookie"])
	require.NotContains(t, credentials, "access_token")
	require.Contains(t, credentials, "model_mapping")
	// A newer cookie/status wins; a stale recovery creates no second outbox event.
	applied, err = repo.RecoverAdobeCookieIfUnchanged(ctx, snapshot, "stale")
	require.NoError(t, err)
	require.False(t, applied)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM scheduler_outbox").Scan(&count))
	require.Equal(t, 1, count)
	// Native Okad reauthorization must resume auth quarantine, preserving pauses
	// for proxy problems, quota errors and already active accounts.
	for _, tc := range []struct {
		status, message string
		resume          bool
	}{
		{service.StatusError, message, true},
		{service.StatusError, "Authentication failed (401): Okad recovery pending", true},
		{service.StatusError, "Token refresh failed (non-retryable): adobe cookie is no longer valid: invalid_credentials", true},
		{service.StatusError, "Clash 订阅节点已移除，请重新选择代理并恢复账号调度", false},
		{service.StatusError, "Quota exhausted", false},
		{service.StatusActive, "", false},
	} {
		reset(tc.status, tc.message, false)
		require.NoError(t, repo.ClearError(ctx, 1))
		require.NoError(t, db.QueryRow("SELECT schedulable FROM accounts WHERE id=1").Scan(&schedulable))
		require.Equal(t, tc.resume, schedulable, tc.message)
	}
	// The outbox failure rolls back the account mutation too.
	reset(service.StatusError, message, false)
	_, err = db.Exec("ALTER TABLE scheduler_outbox ADD CONSTRAINT reject_event CHECK (event_type = 'reject')")
	require.NoError(t, err)
	_, err = repo.RecoverAdobeCookieIfUnchanged(ctx, snapshot, "must-not-commit")
	require.Error(t, err)
	require.NoError(t, db.QueryRow("SELECT status FROM accounts WHERE id=1").Scan(&status))
	require.Equal(t, service.StatusError, status)
}
