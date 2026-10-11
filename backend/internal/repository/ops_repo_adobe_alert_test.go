package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdobeAlertPostgresDeduplicatesAccountsAndIncludesRecoveredAttempts(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`ALTER TABLE accounts ADD COLUMN name text;
 INSERT INTO accounts(id,name,status) VALUES(1,'one','active'),(2,'two','error'),(3,'three','active');
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,group_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);`)
	require.NoError(t, err)
	now := time.Now().UTC()
	start := now.Add(-10 * time.Minute)
	insert := func(id, account, group int, platform string, code any, message, events, owner string, at time.Time) {
		_, err := db.Exec(`INSERT INTO ops_error_logs VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'')`, id, at, account, group, platform, code, message, events, owner)
		require.NoError(t, err)
	}
	// The final account and platform differ from the failed Adobe attempts.
	insert(1, 99, 1, "openai", 401, "", `[{"platform":"adobe","account_id":1,"upstream_status_code":401,"reason":"adobe_quota_exhausted"},{"platform":"adobe","account_id":2,"upstream_status_code":401,"reason":"adobe_auth"},{"platform":"adobe","account_id":1,"upstream_status_code":408}]`, "provider", now.Add(-time.Minute))
	insert(2, 3, 1, "adobe", 401, "model_not_entitled", `[]`, "provider", now.Add(-time.Minute))
	insert(3, 1, 1, "adobe", 401, "unclassified", `null`, "provider", now.Add(-time.Minute))
	insert(4, 1, 1, "adobe", nil, "client API key invalid", `[]`, "client", now.Add(-time.Minute))
	insert(5, 1, 1, "adobe", 408, "system under load", `{}`, "provider", now.Add(-time.Minute))
	insert(6, 4, 1, "adobe", 401, "expired", `[]`, "provider", start.Add(-time.Second))
	insert(7, 5, 2, "adobe", 408, "load", `[]`, "provider", now.Add(-time.Minute))
	insert(8, 6, 1, "openai", 401, "expired", `[]`, "provider", now.Add(-time.Minute))
	insert(9, 7, 1, "adobe", 408, "future", `[]`, "provider", now)
	insert(10, 99, 1, "adobe", 408, "", `[{"platform":"adobe","account_id":"malformed","upstream_status_code":408}]`, "provider", now.Add(-time.Minute))
	group := int64(1)
	r := &opsRepository{db: db}
	rows, err := r.GetAdobeErrorAccounts(context.Background(), start, now, &group)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.EqualValues(t, 1, rows[0].AccountID)
	require.EqualValues(t, 2, rows[0].Count401)
	require.EqualValues(t, 2, rows[0].Count408)
	require.EqualValues(t, 1, rows[0].Quota401)
	require.EqualValues(t, 1, rows[0].Unknown401)
	require.EqualValues(t, 1, rows[1].Auth401)
	require.EqualValues(t, 1, rows[2].Entitled401)
	all, err := r.GetAdobeErrorAccounts(context.Background(), start, now, nil)
	require.NoError(t, err)
	require.Len(t, all, 4)
}

func TestOkadRecoveryPostgresCooldownDoesNotSpendLoginBudget(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra) VALUES(1,'adobe','oauth','error','bad',false,'{"cookie":"c","email":"e"}','{}')`)
	require.NoError(t, err)
	// Use the same database arbitration, including stale/duplicate completion.
	r := &accountRepository{sql: db}
	_, err = db.Exec(`INSERT INTO adobe_cookie_recovery_attempts VALUES(1,'one',1,NOW(),NOW()+INTERVAL '10 minutes',NULL)`)
	require.NoError(t, err)
	require.NoError(t, r.PostponeAdobeCookieRecovery(ctx, 1, "stale", time.Now().Add(time.Hour)))
	require.NoError(t, r.PostponeAdobeCookieRecovery(ctx, 1, "one", time.Now().Add(15*time.Minute)))
	require.NoError(t, r.PostponeAdobeCookieRecovery(ctx, 1, "one", time.Now().Add(time.Hour)))
	var attempts int
	var seconds float64
	require.NoError(t, db.QueryRow(`SELECT attempts,EXTRACT(EPOCH FROM next_attempt_at-NOW()) FROM adobe_cookie_recovery_attempts`).Scan(&attempts, &seconds))
	require.Zero(t, attempts)
	require.InDelta(t, 900, seconds, 2)
}
