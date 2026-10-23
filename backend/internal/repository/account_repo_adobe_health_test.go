package repository

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdobeHealthPostgresEvidenceSurvivesQuietWindowsAndStaleResults(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	migration, err := os.ReadFile("../../migrations/248_adobe_account_health.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE accounts ADD COLUMN name text;
 INSERT INTO accounts(id,name,platform,type,status,credentials,extra) VALUES(1,'fixture','adobe','oauth','active','{"cookie":"c"}','{}');
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,group_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);`)
	require.NoError(t, err)
	ctx := context.Background()
	r := &accountRepository{sql: db}
	a := &service.Account{ID: 1, Credentials: map[string]any{"cookie": "c"}}
	base := time.Now().Add(-time.Hour)
	record := func(success bool, model string, started, completed time.Time, code int) {
		require.NoError(t, r.RecordAdobeHealth(ctx, a, service.AdobeHealthObservation{Success: success, Scope: model, Code: code, Reason: "upstream", Source: "test", StartedAt: started, CheckedAt: completed}))
	}
	blocks := func(want int) {
		var n int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_blocks WHERE account_id=1`).Scan(&n))
		require.Equal(t, want, n)
	}
	report := func() *service.AdobeHealthReport {
		p, e := (&opsRepository{db: db}).GetAdobeHealthReport(ctx, time.Now().Add(-10*time.Minute), time.Now(), nil)
		require.NoError(t, e)
		return p
	}
	record(false, "model-a", base, base.Add(time.Minute), 408)
	blocks(1)
	require.Equal(t, 1, report().Pending, "old failures remain unresolved outside the rolling window")
	require.Zero(t, report().RecentPending)
	record(true, "model-b", base.Add(2*time.Minute), base.Add(3*time.Minute), 200)
	blocks(1)
	require.Equal(t, 1, report().Pending, "another model cannot clear model-a")
	record(true, "model-a", base, base.Add(4*time.Minute), 200)
	blocks(1)
	require.Equal(t, 1, report().Pending, "an earlier in-flight request cannot recover a later failure")
	record(false, "model-a", base.Add(4*time.Minute), base.Add(5*time.Minute), 500)
	blocks(1)
	require.Equal(t, 1, report().Pending, "other error codes must not erase a pending 401/408")
	_, err = db.Exec(`UPDATE accounts SET credentials='{"cookie":"new"}' WHERE id=1`)
	require.NoError(t, err)
	record(true, "model-a", base.Add(6*time.Minute), base.Add(7*time.Minute), 200)
	blocks(1)
	require.Equal(t, 1, report().Pending, "old Cookie success cannot clear current health")
	a.Credentials["cookie"] = "new"
	_, err = db.Exec(`INSERT INTO proxies(id) VALUES(12); UPDATE accounts SET proxy_id=12 WHERE id=1`)
	require.NoError(t, err)
	record(true, "model-a", base.Add(6*time.Minute), base.Add(7*time.Minute), 200)
	require.Equal(t, 1, report().Pending, "old proxy result cannot recover current route")
	proxyID := int64(12)
	a.ProxyID = &proxyID
	a.Proxy = &service.Proxy{ID: proxyID}
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=12`).Scan(&a.Proxy.UpdatedAt))
	_, err = db.Exec(`UPDATE proxies SET updated_at=updated_at+INTERVAL '1 second' WHERE id=12`)
	require.NoError(t, err)
	record(true, "model-a", base.Add(6*time.Minute), base.Add(7*time.Minute), 200)
	require.Equal(t, 1, report().Pending, "edited proxy invalidates an older success")
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=12`).Scan(&a.Proxy.UpdatedAt))
	record(true, "model-a", base.Add(8*time.Minute), base.Add(9*time.Minute), 200)
	blocks(0)
	require.Zero(t, report().Pending)
	require.Equal(t, 1, report().Recovered)
	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_health' FROM accounts WHERE id=1`).Scan(&raw))
	var health map[string]any
	require.NoError(t, json.Unmarshal(raw, &health))
	require.Equal(t, false, health["pending"])
	require.Contains(t, health, "last_success")
	record(false, "model-a", base, base.Add(time.Minute), 408)
	blocks(0)
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_health' FROM accounts WHERE id=1`).Scan(&raw))
	require.NoError(t, json.Unmarshal(raw, &health))
	require.Equal(t, true, health["latest"].(map[string]any)["success"], "out-of-order persistence cannot replace newer displayed evidence")
	require.Zero(t, report().Pending)
	// An actual auth success can recover another model's account-wide auth failure.
	require.NoError(t, r.RecordAdobeHealth(ctx, a, service.AdobeHealthObservation{Scope: "model-c", Code: 401, Reason: "auth", Source: "business", StartedAt: base.Add(10 * time.Minute), CheckedAt: base.Add(11 * time.Minute)}))
	record(true, "model-a", base.Add(12*time.Minute), base.Add(13*time.Minute), 200)
	require.Zero(t, report().Pending)
}

func TestAdobeHealthPostgresRevisionAndPopulation(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	raw, err := os.ReadFile("../../migrations/248_adobe_account_health.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE accounts ADD COLUMN name text;
 INSERT INTO accounts(id,name,platform,status,credentials,extra,updated_at) SELECT n,'test',CASE WHEN n=5 THEN 'openai' ELSE 'adobe' END,'disabled','{}','{}',NOW() FROM generate_series(1,5)n;
 UPDATE accounts SET deleted_at=NOW() WHERE id=4;
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,group_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);`)
	require.NoError(t, err)
	ctx := context.Background()
	r := &accountRepository{sql: db}
	before, err := r.AccountRuntimeRevision(ctx)
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, r.RecordAdobeHealth(ctx, &service.Account{ID: 1}, service.AdobeHealthObservation{Code: 408, Reason: "timeout", StartedAt: now.Add(-time.Minute), CheckedAt: now}))
	after, err := r.AccountRuntimeRevision(ctx)
	require.NoError(t, err)
	require.NotEqual(t, before, after)
	p, err := (&opsRepository{db: db}).GetAdobeHealthReport(ctx, now.Add(-time.Minute), now.Add(time.Second), nil)
	require.NoError(t, err)
	require.EqualValues(t, 3, p.Total)
	require.Equal(t, 1, p.Pending)
	require.Equal(t, 2, p.Unknown)
	require.InDelta(t, 100.0/3, p.Ratio, 0.001)
	require.NoError(t, r.RecordAdobeHealth(ctx, &service.Account{ID: 2}, service.AdobeHealthObservation{Success: true, Source: "business", StartedAt: now, CheckedAt: now}))
	p, err = (&opsRepository{db: db}).GetAdobeHealthReport(ctx, now.Add(-time.Minute), now.Add(time.Second), nil)
	require.NoError(t, err)
	for _, a := range p.Accounts {
		if a.ID == 2 {
			require.Equal(t, "business", a.Source, "first successful record must retain its real source")
		}
	}

}

func TestAdobe408PostgresHoldAliasesAndMultipleModels(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra)
 VALUES(1,'adobe','oauth','active',true,'{"cookie":"fixture","model_mapping":{"keep":"keep"}}','{"custom":"keep"}');
 UPDATE proxy_auto_allocation SET enabled=true`)
	require.NoError(t, err)
	r := &accountRepository{sql: db}
	a := &service.Account{ID: 1, Credentials: map[string]any{"cookie": "fixture"}}
	base := time.Now().Add(-time.Hour)
	record := func(model string, code int, offset time.Duration) {
		require.NoError(t, r.RecordAdobeHealth(context.Background(), a, service.AdobeHealthObservation{Scope: model, Code: code, Success: code == 200, StartedAt: base.Add(offset), CheckedAt: base.Add(offset + time.Second), Source: "test", Reason: "upstream"}))
	}
	record("gpt-image-2", 408, 0)
	var sched, resume, preserved bool
	require.NoError(t, db.QueryRow(`SELECT a.schedulable,h.resume_schedulable,
 a.status='active' AND a.extra->>'custom'='keep' AND a.credentials->'model_mapping'='{"keep":"keep"}'::jsonb
 FROM accounts a JOIN adobe_proxy_408_holds h ON h.account_id=a.id WHERE a.id=1`).Scan(&sched, &resume, &preserved))
	require.False(t, sched)
	require.True(t, resume)
	require.True(t, preserved)
	record("gpt-image-2", 500, time.Minute)
	record("gpt-image-2.5-flare", 408, 2*time.Minute)
	record("firefly-gpt-image-2", 200, 3*time.Minute)
	var models string
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_proxy_408'->'models' FROM accounts WHERE id=1`).Scan(&models))
	require.JSONEq(t, `["gpt-image-2.5-flare"]`, models, "known aliases recover the same family, never another model")
	var oldPending int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_account_health WHERE scope='gpt-image-2' AND (success_started_at IS NULL OR success_started_at<=failed_at)`).Scan(&oldPending))
	require.Zero(t, oldPending, "the visible health incident must also recover for the old alias")
	record("firefly-gpt-image-2-5-flare", 200, 4*time.Minute)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_blocks`).Scan(&oldPending))
	require.Zero(t, oldPending)
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
	require.False(t, sched, "health evidence alone cannot authorize the lease completion or resume an account")
}

func TestAdobe408PostgresMigrationBackfillsOnlyUnresolvedAdobeFailures(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform) VALUES(1,'adobe'),(2,'adobe'),(3,'openai'),(4,'adobe');
 INSERT INTO adobe_account_health(account_id,scope,status_code,failed_at,success_started_at) VALUES
 (1,'model-a',408,NOW()-INTERVAL '1 day',NULL),
 (2,'model-a',408,NOW()-INTERVAL '1 day',NOW()),
 (3,'model-a',408,NOW(),NULL),
 (4,'model-a',401,NOW(),NULL)`)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../migrations/251_adobe_408_proxy_quarantine.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.Exec(string(raw))
		require.NoError(t, err)
	}
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_blocks WHERE account_id=1`).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_blocks WHERE account_id<>1`).Scan(&n))
	require.Zero(t, n)
}

func TestAdobe408PostgresDelayedAliasFailureCannotReopenRecoveredModel(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra) VALUES(1,'adobe','oauth','active',true,'{"cookie":"fixture"}','{}')`)
	require.NoError(t, err)
	r := &accountRepository{sql: db}
	a := &service.Account{ID: 1, Credentials: map[string]any{"cookie": "fixture"}}
	now := time.Now()
	require.NoError(t, r.RecordAdobeHealth(context.Background(), a, service.AdobeHealthObservation{Success: true, Code: 200, Scope: "firefly-gpt-image-2", StartedAt: now.Add(-time.Minute), CheckedAt: now, Source: "test"}))
	require.NoError(t, r.RecordAdobeHealth(context.Background(), a, service.AdobeHealthObservation{Code: 408, Scope: "gpt-image-2", StartedAt: now.Add(-10 * time.Minute), CheckedAt: now.Add(-9 * time.Minute), Source: "test"}))
	var blocks int
	var pending bool
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_blocks`).Scan(&blocks))
	require.Zero(t, blocks)
	require.NoError(t, db.QueryRow(`SELECT (extra->'adobe_health'->>'pending')::bool FROM accounts WHERE id=1`).Scan(&pending))
	require.False(t, pending)
}

func TestAdobeHealthPostgresMigrationKeepsOpenIncidentFailures(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`CREATE TABLE ops_alert_rules(id bigint,window_minutes int,metric_type text);
 CREATE TABLE ops_alert_events(rule_id bigint,status text,fired_at timestamptz);
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);
 INSERT INTO ops_alert_rules VALUES(9,10,'adobe_401_408_accounts');
 INSERT INTO ops_alert_events VALUES(9,'firing',NOW()-INTERVAL '1 hour');
 INSERT INTO accounts(id,platform) VALUES(1,'adobe'),(2,'adobe');
 INSERT INTO adobe_account_test_errors(account_id,created_at,status_code) VALUES(1,NOW()-INTERVAL '65 minutes',408),(2,NOW()-INTERVAL '2 hours',401);
 INSERT INTO ops_error_logs VALUES(1,NOW()-INTERVAL '64 minutes',99,'openai',401,'','[{"platform":"adobe","account_id":2,"upstream_status_code":401}]','provider','upstream_http');`)
	require.NoError(t, err)
	for _, name := range []string{"248_adobe_account_health.sql", "249_adobe_health_legacy_evidence.sql"} {
		raw, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		_, err = db.Exec(string(raw))
		require.NoError(t, err, name)
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM adobe_account_health WHERE success_at IS NULL AND alert_failed_at IS NOT NULL AND reason='legacy'`).Scan(&count))
	require.Equal(t, 2, count)
	var duration float64
	require.NoError(t, db.QueryRow(`SELECT EXTRACT(EPOCH FROM NOW()-failed_at)/60 FROM adobe_account_health WHERE account_id=2`).Scan(&duration))
	require.InDelta(t, 64, duration, 1)
}

func TestManual408HealthRecordsFailureWithoutTakingRoutingOwnership(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra) VALUES(1,'adobe','oauth','active',true,'{"cookie":"c"}','{"manual_proxy_enabled":true,"custom":"keep"}');
 UPDATE proxy_auto_allocation SET enabled=true;`)
	require.NoError(t, err)
	now := time.Now()
	repo := &accountRepository{sql: db}
	// The request started before manual mode; the current row controls routing ownership.
	require.NoError(t, repo.RecordAdobeHealth(context.Background(), &service.Account{ID: 1, Credentials: map[string]any{"cookie": "c"}}, service.AdobeHealthObservation{Code: 408, Scope: "model-a", Reason: "timeout", Source: "business", StartedAt: now.Add(-time.Minute), CheckedAt: now}))
	var schedulable bool
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&schedulable))
	require.True(t, schedulable)
	var blocks, holds int
	require.NoError(t, db.QueryRow(`SELECT (SELECT count(*) FROM adobe_proxy_408_blocks WHERE account_id=1),(SELECT count(*) FROM adobe_proxy_408_holds WHERE account_id=1)`).Scan(&blocks, &holds))
	require.Equal(t, 1, blocks)
	require.Zero(t, holds)
	var state string
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_proxy_408'->>'state' FROM accounts WHERE id=1`).Scan(&state))
	require.Equal(t, "manual", state)
}
