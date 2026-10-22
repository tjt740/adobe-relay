package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"

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

func TestOkad408QueueCanClaimUnboundAccountWithoutBypassingOtherModelBlock(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra) VALUES(1,'adobe','oauth','active',true,'{"cookie":"fixture"}','{}');
 INSERT INTO adobe_proxy_408_blocks VALUES(1,'another-model',NOW()-INTERVAL '1 day')`)
	require.NoError(t, err)
	r := &accountRepository{sql: db}
	a := &service.Account{ID: 1, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "fixture"}}
	require.NoError(t, r.QueueOkadConnectionTest(context.Background(), a, service.OkadConnectionTestKey(a), false))
	_, err = db.Exec(`UPDATE proxy_auto_allocation SET enabled=true`)
	require.NoError(t, err)
	job, err := r.ClaimOkadConnectionTest(context.Background())
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Nil(t, job.ProxyID)
	require.NoError(t, r.FinishOkadConnectionTest(context.Background(), *job, "success"))
	var sched bool
	var status string
	require.NoError(t, db.QueryRow(`SELECT status,schedulable FROM accounts WHERE id=1`).Scan(&status, &sched))
	require.Equal(t, service.StatusError, status)
	require.False(t, sched)
}
