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

func TestAdobeAlertPostgresUnionsTestsAndBusinessWithoutDoubleCountingAccounts(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`ALTER TABLE accounts ADD COLUMN name text;
 INSERT INTO accounts(id,name,status) SELECT n,'fixture','active' FROM generate_series(1,10) n;
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,group_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);`)
	require.NoError(t, err)
	now := time.Now().UTC()
	start := now.Add(-10 * time.Minute)
	_, err = db.Exec(`INSERT INTO ops_error_logs VALUES(1,$1,1,7,'adobe',401,'invalid_credentials','[]','provider','upstream_http');
 `, now.Add(-time.Minute))
	require.NoError(t, err)
	for _, id := range []int{1, 1, 2, 3, 4, 5} {
		_, err = db.Exec(`INSERT INTO adobe_account_test_errors(created_at,account_id,group_ids,status_code,reason,is_timeout) VALUES($1,$2,'{7,8}',408,'',true)`, now.Add(-time.Minute), id)
		require.NoError(t, err)
	}
	// Boundaries, captured groups and 401 category from typed test errors.
	_, err = db.Exec(`INSERT INTO adobe_account_test_errors(created_at,account_id,group_ids,status_code,reason) VALUES
 ($1,6,'{7}',408,''),($2,7,'{7}',401,'adobe_auth'),($3,8,'{8}',401,'adobe_quota_exhausted'),($4,5,'{7}',401,'adobe_not_entitled')`, start.Add(-time.Microsecond), now, now.Add(-time.Minute), start)
	require.NoError(t, err)
	r := &opsRepository{db: db}
	group := int64(7)
	rows, err := r.GetAdobeErrorAccounts(context.Background(), start, now, &group)
	require.NoError(t, err)
	require.Len(t, rows, 5, "one account in both sources is still one affected account")
	require.EqualValues(t, 1, rows[0].AccountID)
	require.EqualValues(t, 1, rows[0].BusinessErrors)
	require.EqualValues(t, 2, rows[0].TestErrors)
	require.EqualValues(t, 2, rows[0].TestTimeouts)
	require.EqualValues(t, 1, rows[0].Auth401)
	require.EqualValues(t, 5, rows[1].AccountID)
	require.EqualValues(t, 1, rows[1].Entitled401)
	all, err := r.GetAdobeErrorAccounts(context.Background(), start, now, nil)
	require.NoError(t, err)
	require.Len(t, all, 6)
	_, err = r.GetAdobeErrorAccounts(context.Background(), now, now, nil)
	require.Error(t, err)
}

func TestAdobeAlertPostgresConcurrentBatchHistory(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	db.SetMaxOpenConns(10)
	_, err := db.Exec(`ALTER TABLE accounts ADD COLUMN name text;
 INSERT INTO accounts(id,name,platform,status) SELECT n,'fixture','adobe','active' FROM generate_series(1,200) n;
 INSERT INTO account_groups SELECT n,7 FROM generate_series(1,200) n;
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,group_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);`)
	require.NoError(t, err)
	start := time.Now().Add(-time.Second)
	recorder := &accountRepository{sql: db}
	results := make(chan error, 10)
	for worker := 0; worker < 10; worker++ {
		go func(worker int) {
			for id := worker + 1; id <= 200; id += 10 {
				for _, code := range []int{401, 408} {
					if err := recorder.RecordAdobeConnectionTestError(context.Background(), int64(id), code, "", code == 408); err != nil {
						results <- err
						return
					}
				}
			}
			results <- nil
		}(worker)
	}
	for worker := 0; worker < 10; worker++ {
		require.NoError(t, <-results)
	}
	rows, err := (&opsRepository{db: db}).GetAdobeErrorAccounts(context.Background(), start, time.Now(), nil)
	require.NoError(t, err)
	require.Len(t, rows, 200)
	for _, row := range rows {
		require.EqualValues(t, 1, row.Count401)
		require.EqualValues(t, 1, row.Count408)
		require.EqualValues(t, 2, row.TestErrors)
		require.Zero(t, row.BusinessErrors)
	}
	t.Logf("200 accounts, 400 failures, 10 concurrent writers: %s", time.Since(start)-time.Second)
}

func TestAdobeAlertPostgresPopulationIncludesPausedAndExcludesDeletedAccounts(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`ALTER TABLE accounts ADD COLUMN name text;
 INSERT INTO accounts(id,name,platform,type,status,schedulable,deleted_at) VALUES
 (1,'active','adobe','oauth','active',true,NULL),
 (2,'paused','adobe','oauth','active',false,NULL),
 (3,'error','adobe','oauth','error',false,NULL),
 (4,'deleted','adobe','oauth','active',true,NOW()),
 (5,'other','openai','oauth','active',true,NULL),
 (6,'relay','adobe','apikey','active',true,NULL);
 INSERT INTO account_groups VALUES(1,7),(2,7),(2,7),(3,8),(4,7),(5,7),(6,8);
 CREATE TABLE ops_error_logs(id bigint,created_at timestamptz,account_id bigint,group_id bigint,platform text,upstream_status_code int,upstream_error_message text,upstream_errors jsonb,error_owner text,error_source text);
 INSERT INTO ops_error_logs SELECT id,NOW()-INTERVAL '1 minute',id,7,'adobe',401,'invalid_credentials','[]','provider','upstream_http' FROM accounts WHERE id<=5;
 INSERT INTO adobe_account_test_errors(created_at,account_id,group_ids,status_code) VALUES(NOW()-INTERVAL '1 minute',6,'{8}',408);`)
	require.NoError(t, err)
	repo := &opsRepository{db: db}
	ctx := context.Background()
	end := time.Now()
	start := end.Add(-10 * time.Minute)
	accounts, total, err := repo.GetAdobeErrorAccountPopulation(ctx, start, end, nil)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Len(t, accounts, 4)
	ids := make([]int64, 0)
	for _, a := range accounts {
		ids = append(ids, a.AccountID)
	}
	require.ElementsMatch(t, []int64{1, 2, 3, 6}, ids)
	group := int64(7)
	accounts, total, err = repo.GetAdobeErrorAccountPopulation(ctx, start, end, &group)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, accounts, 2)
	legacy, err := repo.GetAdobeErrorAccounts(ctx, start, end, nil)
	require.NoError(t, err)
	require.Len(t, legacy, 6, "legacy count-only rules retain their historical scope")
	_, err = db.Exec(`TRUNCATE ops_error_logs,adobe_account_test_errors`)
	require.NoError(t, err)
	accounts, total, err = repo.GetAdobeErrorAccountPopulation(ctx, start, end, nil)
	require.NoError(t, err)
	require.EqualValues(t, 4, total)
	require.Empty(t, accounts)
}
