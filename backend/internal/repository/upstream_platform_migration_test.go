package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestUpstreamV0217UpgradeFromForkPreservesBusinessState(t *testing.T) {
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

	// All migrations already present in the deployed fork, with only the seven
	// newly imported upstream files withheld until after the business fixture.
	newFiles := map[string]bool{
		"241_add_payment_order_bonus_amount.sql":        true,
		"241_add_typesafe_platform.sql":                 true,
		"242_drop_platform_check_constraints.sql":       true,
		"242_group_i18n.sql":                            true,
		"243_gemini_cached_contents.sql":                true,
		"244_gemini_cached_contents_channel_usage.sql":  true,
		"245_gemini_cached_contents_mutation_lease.sql": true,
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
	require.JSONEq(t, before, after)
}

// New upstream migrations run after the fork already has Adobe/Kiro rows.
// The temporary TypeSafe CHECK must accept those rows before the next migration
// moves validation into the shared application platform catalog.
func TestUpstreamV0217MigrationPreservesForkPlatformRows(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`
CREATE TABLE user_platform_quotas(platform text NOT NULL, daily_limit_usd numeric);
CREATE TABLE composite_model_routes(target_platform text NOT NULL, upstream_model text);
INSERT INTO user_platform_quotas VALUES ('adobe', 12.5), ('kiro', NULL);
INSERT INTO composite_model_routes VALUES ('adobe', 'gpt-image-2.5'), ('kiro', 'claude-opus-4-8');`)
	require.NoError(t, err)

	for _, name := range []string{"241_add_typesafe_platform.sql", "242_drop_platform_check_constraints.sql"} {
		body, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = db.Exec(string(body))
		require.NoError(t, err, name)
	}
	var limits, routes string
	require.NoError(t, db.QueryRow(`SELECT json_agg(q ORDER BY platform)::text FROM user_platform_quotas q`).Scan(&limits))
	require.JSONEq(t, `[{"platform":"adobe","daily_limit_usd":12.5},{"platform":"kiro","daily_limit_usd":null}]`, limits)
	require.NoError(t, db.QueryRow(`SELECT json_agg(r ORDER BY target_platform)::text FROM composite_model_routes r`).Scan(&routes))
	require.JSONEq(t, `[{"target_platform":"adobe","upstream_model":"gpt-image-2.5"},{"target_platform":"kiro","upstream_model":"claude-opus-4-8"}]`, routes)
}
