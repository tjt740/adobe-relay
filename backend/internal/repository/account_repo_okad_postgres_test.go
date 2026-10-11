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

func newOkadRecoveryPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("OKAD_RECOVERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OKAD_RECOVERY_TEST_DATABASE_URL to a disposable PostgreSQL instance")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	schema := fmt.Sprintf("okad_recovery_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = root.Exec("DROP SCHEMA " + schema + " CASCADE") })
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE accounts(id bigint PRIMARY KEY,platform text,type text,status text,error_message text,schedulable bool,credentials jsonb,extra jsonb,proxy_id bigint,expires_at timestamptz,deleted_at timestamptz,updated_at timestamptz);
 CREATE TABLE proxies(id bigint PRIMARY KEY);
 CREATE TABLE scheduler_outbox(event_type text,account_id bigint,group_id bigint,payload jsonb);`)
	require.NoError(t, err)
	for _, name := range []string{"241_account_proxy_failover.sql", "242_proxy_auto_allocation.sql", "243_proxy_auto_allocation_all_accounts.sql", "244_proxy_auto_allocation_allocator_bypass.sql", "245_adobe_cookie_recovery_attempts.sql"} {
		migration, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		_, err = db.Exec(string(migration))
		require.NoError(t, err, name)
	}
	return db
}

func TestOkadRecoveryPostgres(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	var err error
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	message := "OAuth 401 (no refresh_token): Invalid bearer token"
	reset := func(status, message string, schedulable bool) {
		_, err = db.Exec(`TRUNCATE accounts, scheduler_outbox CASCADE;
 INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,proxy_id,expires_at,deleted_at,updated_at) VALUES(1,'adobe','oauth','error','',false,'{"cookie":"old","email":"test@example.test","access_token":"rejected","model_mapping":{"gpt-image-2":"keep"}}','{}',NULL,NULL,NULL,NOW())`)
		require.NoError(t, err)
		_, err = db.Exec("UPDATE accounts SET status=$1,error_message=$2,schedulable=$3", status, message, schedulable)
		require.NoError(t, err)
	}
	snapshot := &service.Account{ID: 1, Platform: service.PlatformAdobe, Type: service.AccountTypeOAuth, Status: service.StatusError, ErrorMessage: message, Credentials: map[string]any{"cookie": "old", "email": "test@example.test"}}
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
	// Failed logins may queue only the exact active snapshot, never a pause or
	// credential/identity/proxy edit made while the login was running.
	active := *snapshot
	active.Status, active.ErrorMessage, active.Schedulable = service.StatusActive, "", true
	for _, mutation := range []string{"", "UPDATE accounts SET status='disabled'", "UPDATE accounts SET schedulable=false", "UPDATE accounts SET credentials=credentials || '{\"cookie\":\"newer\"}'", "UPDATE accounts SET proxy_id=9"} {
		reset(service.StatusActive, "", true)
		if mutation != "" {
			_, err = db.Exec(mutation)
			require.NoError(t, err)
		}
		applied, err = repo.QueueAdobeCookieRecoveryIfUnchanged(ctx, &active)
		require.NoError(t, err)
		require.Equal(t, mutation == "", applied, mutation)
	}
	// Reproduce both orders of the real recovery/Okad push using the production
	// allocation trigger. Clearing a proxy wait must not destroy queue ownership.
	_, err = db.Exec("UPDATE proxy_auto_allocation SET enabled=true")
	require.NoError(t, err)
	for _, callbackFirst := range []bool{false, true} {
		reset(service.StatusError, message, false)
		if callbackFirst {
			_, err = db.Exec(`UPDATE accounts SET credentials=credentials || '{"cookie":"fresh"}'`)
			require.NoError(t, err)
			require.NoError(t, repo.ClearError(ctx, 1))
		}
		_, err = repo.RecoverAdobeCookieIfUnchanged(ctx, snapshot, "fresh")
		require.NoError(t, err)
		require.NoError(t, repo.ClearError(ctx, 1))
		var queued bool
		var errorMessage string
		require.NoError(t, db.QueryRow("SELECT status,schedulable,proxy_auto_paused,error_message FROM accounts WHERE id=1").Scan(&status, &schedulable, &queued, &errorMessage))
		require.Equal(t, service.StatusError, status)
		require.False(t, schedulable)
		require.True(t, queued)
		require.Equal(t, "自动代理分配：等待可用节点（每个节点最多 3 个账号）", errorMessage)
		// The allocator can now pick this queued account up and resume it.
		tx, err := db.Begin()
		require.NoError(t, err)
		_, err = tx.Exec("SELECT set_config('sub2api.proxy_allocator','on',true)")
		require.NoError(t, err)
		_, err = tx.Exec("UPDATE accounts SET status='active',schedulable=true,proxy_id=7,proxy_auto_paused=false,error_message=NULL")
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		require.NoError(t, repo.ClearError(ctx, 1))
		require.NoError(t, db.QueryRow("SELECT schedulable FROM accounts WHERE id=1").Scan(&schedulable))
		require.True(t, schedulable)
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
