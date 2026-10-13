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
	report := func() *service.AdobeHealthReport {
		p, e := (&opsRepository{db: db}).GetAdobeHealthReport(ctx, time.Now().Add(-10*time.Minute), time.Now(), nil)
		require.NoError(t, e)
		return p
	}
	record(false, "model-a", base, base.Add(time.Minute), 408)
	require.Equal(t, 1, report().Pending, "old failures remain unresolved outside the rolling window")
	require.Zero(t, report().RecentPending)
	record(true, "model-b", base.Add(2*time.Minute), base.Add(3*time.Minute), 200)
	require.Equal(t, 1, report().Pending, "another model cannot clear model-a")
	record(true, "model-a", base, base.Add(4*time.Minute), 200)
	require.Equal(t, 1, report().Pending, "an earlier in-flight request cannot recover a later failure")
	record(false, "model-a", base.Add(4*time.Minute), base.Add(5*time.Minute), 500)
	require.Equal(t, 1, report().Pending, "other error codes must not erase a pending 401/408")
	_, err = db.Exec(`UPDATE accounts SET credentials='{"cookie":"new"}' WHERE id=1`)
	require.NoError(t, err)
	record(true, "model-a", base.Add(6*time.Minute), base.Add(7*time.Minute), 200)
	require.Equal(t, 1, report().Pending, "old Cookie success cannot clear current health")
	a.Credentials["cookie"] = "new"
	record(true, "model-a", base.Add(8*time.Minute), base.Add(9*time.Minute), 200)
	require.Zero(t, report().Pending)
	require.Equal(t, 1, report().Recovered)
	var raw []byte
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_health' FROM accounts WHERE id=1`).Scan(&raw))
	var health map[string]any
	require.NoError(t, json.Unmarshal(raw, &health))
	require.Equal(t, false, health["pending"])
	require.Contains(t, health, "last_success")
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
