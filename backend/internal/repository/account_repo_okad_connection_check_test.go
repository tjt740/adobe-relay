package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOkadConnectionTestQueuePostgres(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,proxy_id,updated_at)
 VALUES(1,'adobe','oauth','inactive',false,'{"cookie":"cookie-1","email":"one@example.test"}',
 '{"okad":{"verification":"pending","version":1},"keep":true}',7,NOW())`)
	require.NoError(t, err)
	a := &service.Account{ID: 1, Platform: "adobe", Type: "oauth", Credentials: map[string]any{"cookie": "cookie-1"}}
	proxyID := int64(7)
	a.ProxyID = &proxyID
	key := service.OkadConnectionTestKey(a)
	require.NoError(t, repo.QueueOkadConnectionTest(ctx, a, key, true))
	job, err := repo.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.Nil(t, job, "reliable deliveries wait for identity/proxy verification")
	_, err = db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{okad,verification}','"verified"') WHERE id=1`)
	require.NoError(t, err)
	job, err = repo.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.Equal(t, &service.OkadConnectionTestJob{AccountID: 1, Key: key, ProxyID: &proxyID}, job)
	// Delivery retries and a new repository/process cannot re-claim a started job.
	require.NoError(t, repo.QueueOkadConnectionTest(ctx, a, key, true))
	restarted := &accountRepository{sql: db}
	duplicate, err := restarted.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.Nil(t, duplicate)
	require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "success"))
	require.NoError(t, repo.QueueOkadConnectionTest(ctx, a, key, true))
	duplicate, err = repo.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.Nil(t, duplicate)
	// The next Cookie has its own check; an old result cannot finish/overwrite it.
	_, err = db.Exec(`UPDATE accounts SET credentials=jsonb_set(credentials,'{cookie}','"cookie-2"') WHERE id=1`)
	require.NoError(t, err)
	applied, err := repo.RecordConnectionTestResultIfCurrent(ctx, a, false, 408)
	require.NoError(t, err)
	require.False(t, applied)
	a.Credentials["cookie"] = "cookie-2"
	require.NoError(t, repo.QueueOkadConnectionTest(ctx, a, service.OkadConnectionTestKey(a), false))
	newJob, err := restarted.ClaimOkadConnectionTest(ctx)
	require.NoError(t, err)
	require.NotEqual(t, job.Key, newJob.Key)
	require.NoError(t, repo.FinishOkadConnectionTest(ctx, *job, "failed"))
	applied, err = repo.RecordConnectionTestResultIfCurrent(ctx, a, true, 200)
	require.NoError(t, err)
	require.True(t, applied)
	var state, status, cookie string
	var schedulable, keep, success bool
	var proxy int
	require.NoError(t, db.QueryRow(`SELECT extra->'okad_connection_test'->>'state',status,schedulable,proxy_id,
 credentials->>'cookie',(extra->>'keep')::bool,(extra->>'last_check_success')::bool FROM accounts WHERE id=1`).Scan(&state, &status, &schedulable, &proxy, &cookie, &keep, &success))
	require.Equal(t, "running", state)
	require.Equal(t, "inactive", status)
	require.False(t, schedulable)
	require.Equal(t, 7, proxy)
	require.Equal(t, "cookie-2", cookie)
	require.True(t, keep)
	require.True(t, success)
}

func TestOkadManualTimeoutUsesActualTestStart(t *testing.T) {
	for _, tc := range []struct {
		name, mutation        string
		noStart, automatic    bool
		clearGate, scheduling bool
	}{
		{"manual check without queue start", "", false, false, true, true},
		{"stale result", `UPDATE accounts SET extra=jsonb_set(extra,'{last_check_at}',to_jsonb(NOW()-INTERVAL '1 hour'))`, false, false, false, false},
		{"test predates queue", `UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test,queued_at}',to_jsonb(NOW()))`, false, false, false, false},
		{"no actual start", "", true, false, false, false},
		{"automatic job cannot use manual bound", "", false, true, false, false},
		{"operator pause", `UPDATE accounts SET extra=extra||'{"okad_manual_pause":true}'`, false, false, true, false},
		{"already stopped before test", `UPDATE accounts SET extra=jsonb_set(extra,'{okad_connection_test,resume}','false')`, false, false, true, false},
		{"quota blocked", `UPDATE accounts SET extra=jsonb_set(extra,'{okad,verification}','"quota_blocked"')`, false, false, false, false},
		{"quota with verification disabled", `UPDATE accounts SET extra=jsonb_set(jsonb_set(extra,'{okad,verification}','"quota_blocked"'),'{okad_connection_test,require_verification}','false')`, false, false, false, false},
		{"401 result", `UPDATE accounts SET extra=jsonb_set(extra,'{last_check_status_code}','401')`, false, false, false, false},
		{"new auth error", `UPDATE accounts SET error_message='Authentication failed (401): fresh rejection'`, false, false, false, false},
		{"new Cookie", `UPDATE accounts SET credentials=jsonb_set(credentials,'{cookie}','"replacement"')`, false, false, false, false},
		{"new route", `UPDATE accounts SET proxy_id=8`, false, false, false, false},
		{"inactive", `UPDATE accounts SET status='inactive'`, false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newOkadRecoveryPostgres(t)
			_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,proxy_id,credentials,extra)
 VALUES(1,'adobe','oauth','error','Okad credentials pending connection test',false,7,'{"cookie":"fixture"}',
 jsonb_build_object('last_check_status_code',408,'last_check_success',false,'last_check_at',NOW()-INTERVAL '1 minute',
 'okad',jsonb_build_object('verification','verified'), 'okad_connection_test',jsonb_build_object('gated',true,'state','running','resume',true,
 'require_verification',true,'queued_at',NOW()-INTERVAL '10 minutes','key',encode(sha256(convert_to('fixture','UTF8')),'hex'))))`)
			require.NoError(t, err)
			if tc.mutation != "" {
				_, err = db.Exec(tc.mutation)
				require.NoError(t, err)
			}
			proxy := int64(7)
			a := &service.Account{Credentials: map[string]any{"cookie": "fixture"}}
			job := service.OkadConnectionTestJob{AccountID: 1, Key: service.OkadConnectionTestKey(a), ProxyID: &proxy, Manual: !tc.automatic}
			if !tc.noStart {
				job.StartedAt = time.Now().Add(-2 * time.Minute)
			}
			require.NoError(t, (&accountRepository{sql: db}).FinishOkadConnectionTest(context.Background(), job, "failed"))
			var sched, gated bool
			require.NoError(t, db.QueryRow(`SELECT schedulable,(extra->'okad_connection_test'->>'gated')::bool FROM accounts WHERE id=1`).Scan(&sched, &gated))
			require.Equal(t, tc.scheduling, sched)
			require.Equal(t, !tc.clearGate, gated)
		})
	}
}

func TestOkadConnectionTestQueuePostgresConcurrentClaims(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,updated_at)
 SELECT i,'adobe','oauth','active',true,jsonb_build_object('cookie','cookie-'||i),'{}',NOW() FROM generate_series(1,200) i`)
	require.NoError(t, err)
	for id := int64(1); id <= 200; id++ {
		a := &service.Account{ID: id, Credentials: map[string]any{"cookie": fmt.Sprintf("cookie-%d", id)}}
		require.NoError(t, repo.QueueOkadConnectionTest(ctx, a, service.OkadConnectionTestKey(a), false))
	}
	var wg sync.WaitGroup
	results := make(chan int64, 200)
	errors := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				job, err := repo.ClaimOkadConnectionTest(ctx)
				if err != nil {
					errors <- err
					return
				}
				if job == nil {
					return
				}
				results <- job.AccountID
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	seen := map[int64]bool{}
	for id := range results {
		require.False(t, seen[id], "account %d claimed twice", id)
		seen[id] = true
	}
	require.Len(t, seen, 200)
}

func TestOkad408HistoryDoesNotBlockSuccessfulQueueCompletion(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,proxy_id) VALUES(1,'adobe','oauth','active',true,'{"cookie":"fixture"}','{}',7);
 INSERT INTO adobe_proxy_408_blocks VALUES(1,'another-model',NOW()-INTERVAL '1 day')`)
	require.NoError(t, err)
	r := &accountRepository{sql: db}
	a := &service.Account{ID: 1, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "fixture"}}
	require.NoError(t, r.QueueOkadConnectionTest(context.Background(), a, service.OkadConnectionTestKey(a), false))
	job, err := r.ClaimOkadConnectionTest(context.Background())
	require.NoError(t, err)
	require.NotNil(t, job)
	require.NoError(t, r.FinishOkadConnectionTest(context.Background(), *job, "success"))
	var sched bool
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
	require.True(t, sched)
}

func TestOkadTimeoutReturnsQueuePauseWithoutSuccess(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		code                    int
		verified, paused, stale bool
	}{
		{"408 verified", 408, true, false, false}, {"408 manual pause", 408, true, true, false},
		{"408 unverified identity", 408, false, false, false}, {"401 still gated", 401, true, false, false},
		{"stale timeout record", 408, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newOkadRecoveryPostgres(t)
			_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,proxy_id) VALUES(1,'adobe','oauth','active',true,'{"cookie":"fixture"}',jsonb_build_object('okad_manual_pause',$1::boolean,'okad',jsonb_build_object('verification',CASE WHEN $2 THEN 'verified' ELSE 'pending' END)),7)`, tc.paused, tc.verified)
			require.NoError(t, err)
			r := &accountRepository{sql: db}
			a := &service.Account{ID: 1, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "fixture"}}
			require.NoError(t, r.QueueOkadConnectionTest(context.Background(), a, service.OkadConnectionTestKey(a), true))
			if !tc.verified {
				_, err = db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{okad,verification}','"verified"')`)
				require.NoError(t, err)
			}
			job, err := r.ClaimOkadConnectionTest(context.Background())
			require.NoError(t, err)
			require.NotNil(t, job)
			if !tc.verified {
				_, err = db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{okad,verification}','"pending"')`)
				require.NoError(t, err)
			}
			_, err = db.Exec(`UPDATE accounts SET extra=extra||jsonb_build_object('last_check_success',false,'last_check_status_code',$1::integer,'last_check_at',CASE WHEN $2 THEN NOW()-INTERVAL '1 hour' ELSE clock_timestamp() END)`, tc.code, tc.stale)
			require.NoError(t, err)
			require.NoError(t, r.FinishOkadConnectionTest(context.Background(), *job, "failed"))
			var sched, gated bool
			var code int
			var state string
			require.NoError(t, db.QueryRow(`SELECT schedulable,(extra->'okad_connection_test'->>'gated')::bool,(extra->>'last_check_status_code')::int,extra->'okad_connection_test'->>'state' FROM accounts WHERE id=1`).Scan(&sched, &gated, &code, &state))
			release := tc.code == 408 && tc.verified && !tc.stale
			require.Equal(t, release && !tc.paused, sched)
			require.Equal(t, !release, gated)
			require.Equal(t, tc.code, code)
			require.Equal(t, "failed", state)
		})
	}
}
