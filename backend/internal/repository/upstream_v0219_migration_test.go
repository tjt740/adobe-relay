package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestUpstreamV0219UpgradePreservesCurrentForkState(t *testing.T) {
	dsn := os.Getenv("OKAD_RECOVERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set OKAD_RECOVERY_TEST_DATABASE_URL to a disposable PostgreSQL instance")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	databaseName := fmt.Sprintf("upstream_upgrade_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE DATABASE " + databaseName)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = root.Exec("DROP DATABASE " + databaseName) })
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Del("search_path")
	u.Path = "/" + databaseName
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Reproduce the installed fork and apply only the three incoming SQL files.
	newFiles := map[string]bool{
		"246_enable_gemini_image_generation_groups.sql": true,
		"247_scoped_admin_api_keys.sql":                 true,
		"248_group_image_count_billing.sql":             true,
	}
	baseline := fstest.MapFS{}
	entries, err := migrations.FS.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || newFiles[entry.Name()] {
			continue
		}
		body, err := migrations.FS.ReadFile(entry.Name())
		require.NoError(t, err)
		baseline[entry.Name()] = &fstest.MapFile{Data: body}
	}
	ctx := context.Background()
	// The installed fork already contains migration 121's report-width repair.
	// Bootstrap that historical repair before replaying its older backfill;
	// PostgreSQL validates the long constant even when there are no user rows.
	bootstrap := fstest.MapFS{}
	for name, entry := range baseline {
		if name < "109_" || name == "121_auth_identity_migration_report_type_widen.sql" {
			bootstrap[name] = entry
		}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, bootstrap))
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	_, err = db.Exec(`
INSERT INTO users(id,email,password_hash) VALUES (1,'upgrade@example.test','synthetic');
INSERT INTO proxies(id,name,protocol,host,port) VALUES (1,'preserved','http','127.0.0.1',8080);
INSERT INTO groups(id,name,platform) VALUES (2,'manual group 2','adobe'),(7,'manual group 7','adobe');
INSERT INTO accounts(id,name,platform,type,credentials,extra,proxy_id,status,schedulable)
VALUES (1,'paused Adobe','adobe','oauth','{"cookie":"synthetic","model_mapping":{"gpt-image-2.5":"keep"}}','{"adobe_manual_disabled":true}',1,'error',false);
INSERT INTO account_groups(account_id,group_id) VALUES (1,2),(1,7);
INSERT INTO user_platform_quotas(user_id,platform,daily_limit_usd) VALUES (1,'adobe',12.5),(1,'kiro',NULL);
INSERT INTO settings(key,value) VALUES ('ops_adobe_alert_mail_private','{"synthetic":true}'),('keep_setting','keep');`)
	require.NoError(t, err)
	const snapshotSQL = `SELECT jsonb_build_object(
'accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),
'proxies',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM proxies p),
'bindings',(SELECT jsonb_agg(to_jsonb(b) ORDER BY account_id,group_id) FROM account_groups b),
'groups',(SELECT jsonb_agg(jsonb_build_array(id,name,platform,status,rate_multiplier) ORDER BY id) FROM groups),
'settings',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM settings s),
'quotas',(SELECT jsonb_agg(to_jsonb(q) ORDER BY id) FROM user_platform_quotas q))::text`
	var before, after string
	require.NoError(t, db.QueryRow(snapshotSQL).Scan(&before))
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db), "reapplying must remain idempotent")
	require.NoError(t, db.QueryRow(snapshotSQL).Scan(&after))
	var enabled int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM groups WHERE image_count_billing`).Scan(&enabled))
	require.Zero(t, enabled, "new per-image switch must default off")
	var keys int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM scoped_admin_api_keys`).Scan(&keys))
	require.Zero(t, keys, "upgrade must not create administrator keys")
	require.JSONEq(t, before, after)
}
