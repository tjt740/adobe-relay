package proxyfailover

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func quarantinedFixture(t *testing.T) (*sql.DB, *Manager) {
	t.Helper()
	db := testDB(t)
	_, err := db.Exec(`UPDATE proxies SET status='inactive' WHERE id<>1;
 INSERT INTO accounts(id,proxy_id,status,schedulable,credentials,extra) VALUES
 (10,1,'active',true,'{"cookie":"fixture-cookie","model_mapping":{"keep":"unchanged"}}','{"custom":"keep"}');
 INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code) VALUES(10,'model-a',NOW()-INTERVAL '1 hour',408);
 INSERT INTO adobe_proxy_408_blocks(account_id,scope,failed_at) SELECT account_id,scope,failed_at FROM adobe_account_health;`)
	require.NoError(t, err)
	m := NewManager(db)
	_, err = m.SaveAllocation(context.Background(), AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	m.probe = func(_ context.Context, _ proxy) health { return health{Status: "healthy", LatencyMS: 20} }
	require.NoError(t, m.check(context.Background()))
	return db, m
}

func assertRouting(t *testing.T, db *sql.DB, id, proxy int64, schedulable bool) {
	t.Helper()
	var actual int64
	var scheduling bool
	require.NoError(t, db.QueryRow(`SELECT COALESCE(proxy_id,0),schedulable FROM accounts WHERE id=$1`, id).Scan(&actual, &scheduling))
	require.Equal(t, proxy, actual)
	require.Equal(t, schedulable, scheduling)
}

func proveLease(t *testing.T, db *sql.DB, id int64, scope string) {
	t.Helper()
	_, err := db.Exec(`WITH cleared AS (DELETE FROM adobe_proxy_408_blocks WHERE account_id=$1 AND scope=$2 RETURNING account_id)
 UPDATE adobe_account_health SET success_started_at=clock_timestamp() WHERE account_id=$1 AND scope=$2`, id, scope)
	require.NoError(t, err)
}

func Test408QuarantineReleasesSlotsAndTestFailureReturnsBorrowedSlot(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	assertRouting(t, db, 10, 0, false)
	v, err := m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, v.Quarantined408Accounts)
	require.Zero(t, v.Pending408Release)
	require.Zero(t, v.Waiting, "quarantined accounts are not regular allocation demand")
	require.Equal(t, 1, v.AvailableSlots)
	// An inactive binding may lend its place, while the two business accounts stay pinned.
	_, err = db.Exec(`INSERT INTO accounts(id,proxy_id,status,schedulable,error_message) VALUES(3,1,'disabled',false,'operator disabled')`)
	require.NoError(t, err)
	lease, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	require.NotNil(t, lease)
	assertRouting(t, db, 10, 1, false)
	var phase string
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_proxy_408'->>'state' FROM accounts WHERE id=10`).Scan(&phase))
	require.Equal(t, "testing", phase)
	assertRouting(t, db, 3, 0, false)
	assertRouting(t, db, 1, 1, true)
	assertRouting(t, db, 2, 1, true)
	_, err = m.AcquireTestProxy(ctx, 10, "model-a")
	require.ErrorIs(t, err, ErrTestBusy)
	require.NoError(t, m.check(ctx), "regular maintenance must retain a running test lease")
	assertRouting(t, db, 10, 1, false)
	v, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, v.TemporaryTestBindings)
	require.Zero(t, v.ReclaimableSlots, "a running test is not reclaimable")
	require.NoError(t, m.FinishTestProxy(ctx, *lease, false, true))
	assertRouting(t, db, 10, 0, false)
	require.NoError(t, db.QueryRow(`SELECT extra->'adobe_proxy_408'->>'state' FROM accounts WHERE id=10`).Scan(&phase))
	require.Equal(t, "quarantined", phase)
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 3, 1, false)
	var preserved bool
	require.NoError(t, db.QueryRow(`SELECT credentials->'model_mapping'='{"keep":"unchanged"}'::jsonb AND extra->>'custom'='keep'
 AND status='active' AND COALESCE(error_message,'')='' FROM accounts WHERE id=10`).Scan(&preserved))
	require.True(t, preserved, "quarantine changes routing and its own scheduling hold only")
}

func Test408TestPromotionRequiresCorrespondingModelEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, proof                  string
		other, success, keep, resume bool
	}{
		{name: "current model success", proof: "model-a", success: true, keep: true, resume: true},
		{name: "success cannot be claimed without evidence", success: true},
		{name: "another model cannot clear failure", proof: "model-b", success: true},
		{name: "another unresolved model keeps quarantine", proof: "model-a", other: true, success: true},
		{name: "canceled completion is not promoted", proof: "model-a", success: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, m := quarantinedFixture(t)
			if tc.other {
				_, err := db.Exec(`INSERT INTO adobe_proxy_408_blocks VALUES(10,'model-b',NOW()-INTERVAL '1 minute')`)
				require.NoError(t, err)
			}
			lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
			require.NoError(t, err)
			if tc.proof != "" {
				proveLease(t, db, 10, tc.proof)
			}
			require.NoError(t, m.FinishTestProxy(context.Background(), *lease, tc.success, true))
			pid := int64(0)
			if tc.keep {
				pid = 1
			}
			assertRouting(t, db, 10, pid, tc.resume)
			var holds int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_holds WHERE account_id=10`).Scan(&holds))
			if tc.keep {
				require.Zero(t, holds)
			} else {
				require.Equal(t, 1, holds)
			}
		})
	}
}

func Test408PromotionPreservesManualPauseAndTestRecoveryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, mutation string
		allow          bool
	}{
		{"manual pause during test", `UPDATE accounts SET extra=extra||'{"okad_manual_pause":true}' WHERE id=10`, true},
		{"scheduled test recovery disabled", `SELECT 1`, false},
		{"cooldown added during test", `UPDATE accounts SET temp_unschedulable_until=NOW()+INTERVAL '1 hour' WHERE id=10`, true},
		{"expiry during test", `UPDATE accounts SET expires_at=NOW()-INTERVAL '1 second' WHERE id=10`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, m := quarantinedFixture(t)
			lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
			require.NoError(t, err)
			proveLease(t, db, 10, "model-a")
			_, err = db.Exec(tc.mutation)
			require.NoError(t, err)
			require.NoError(t, m.FinishTestProxy(context.Background(), *lease, true, tc.allow))
			assertRouting(t, db, 10, 1, false)
		})
	}
}

func Test408ExpiredLeaseAndStaleCompletionCannotResumeOrOverrideNewOwner(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	first, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE adobe_proxy_test_leases SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 10, 0, false)
	second, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	require.NotEqual(t, first.Owner, second.Owner)
	require.NoError(t, m.FinishTestProxy(ctx, *first, false, false))
	assertRouting(t, db, 10, 1, false)
	var owner string
	require.NoError(t, db.QueryRow(`SELECT owner FROM adobe_proxy_test_leases WHERE account_id=10`).Scan(&owner))
	require.Equal(t, second.Owner, owner)
	_, err = db.Exec(`UPDATE accounts SET credentials=credentials||'{"cookie":"new-cookie"}' WHERE id=10`)
	require.NoError(t, err)
	proveLease(t, db, 10, "model-a")
	require.NoError(t, m.FinishTestProxy(ctx, *second, true, true))
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 10, 0, false)
}

func Test408LateSuccessCannotPromoteAnExpiredLease(t *testing.T) {
	db, m := quarantinedFixture(t)
	lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
	require.NoError(t, err)
	proveLease(t, db, 10, "model-a")
	_, err = db.Exec(`UPDATE adobe_proxy_test_leases SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	require.NoError(t, m.FinishTestProxy(context.Background(), *lease, true, true))
	assertRouting(t, db, 10, 0, false)
}

func Test408ExpiredTestReleasesAfterDisablingWithoutChangingManualBindings(t *testing.T) {
	db, m := quarantinedFixture(t)
	_, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
	require.NoError(t, err)
	v, err := m.GetAllocation(context.Background())
	require.NoError(t, err)
	_, err = m.SaveAllocation(context.Background(), AllocationPolicy{Enabled: false, Revision: v.Revision})
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET proxy_id=3 WHERE id=1; UPDATE adobe_proxy_test_leases SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	require.NoError(t, m.check(context.Background()))
	assertRouting(t, db, 10, 0, false)
	assertRouting(t, db, 1, 3, true)
	var managed bool
	require.NoError(t, db.QueryRow(`SELECT (extra->'adobe_proxy_408'->>'managed')::bool FROM accounts WHERE id=10`).Scan(&managed))
	require.False(t, managed)
}

func Test408PromotionKeepsNewCookieResumeIntentAndDisabledRecoveryPaused(t *testing.T) {
	t.Run("new cookie keeps original check resume intent", func(t *testing.T) {
		db, m := quarantinedFixture(t)
		_, err := db.Exec(`UPDATE adobe_proxy_408_holds SET resume_schedulable=false;
 UPDATE accounts SET extra=extra||'{"okad_connection_test":{"gated":true,"state":"success","resume":true,"require_verification":true},"okad":{"verification":"verified"}}' WHERE id=10`)
		require.NoError(t, err)
		lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
		require.NoError(t, err)
		proveLease(t, db, 10, "model-a")
		require.NoError(t, m.FinishTestProxy(context.Background(), *lease, true, true))
		assertRouting(t, db, 10, 1, true)
	})
	t.Run("allocator pause cannot bypass disabled recovery", func(t *testing.T) {
		db, m := quarantinedFixture(t)
		lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE accounts SET status='error',schedulable=false,proxy_auto_paused=true,error_message='自动代理分配：等待可用节点（每个节点最多 3 个账号）' WHERE id=10`)
		require.NoError(t, err)
		proveLease(t, db, 10, "model-a")
		require.NoError(t, m.FinishTestProxy(context.Background(), *lease, true, false))
		require.NoError(t, m.check(context.Background()))
		assertRouting(t, db, 10, 1, false)
	})
}

func Test408ParallelLeasesRespectCapacityAndNeverDisplaceBusiness(t *testing.T) {
	db, m := quarantinedFixture(t)
	_, err := db.Exec(`INSERT INTO accounts(id,status,schedulable,credentials) VALUES(11,'active',false,'{"cookie":"second"}');
 INSERT INTO adobe_proxy_408_blocks VALUES(11,'model-a',NOW()-INTERVAL '1 hour')`)
	require.NoError(t, err)
	results := make(chan *TestLease, 2)
	errorsCh := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []int64{10, 11} {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			l, e := m.AcquireTestProxy(context.Background(), id, "model-a")
			results <- l
			errorsCh <- e
		}(id)
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	var winner *TestLease
	for l := range results {
		if l != nil {
			require.Nil(t, winner)
			winner = l
		}
	}
	require.NotNil(t, winner)
	failures := 0
	for e := range errorsCh {
		if e != nil {
			require.True(t, errors.Is(e, ErrNoTestProxy), e)
			failures++
		}
	}
	require.Equal(t, 1, failures)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_id=1`).Scan(&n))
	require.Equal(t, 3, n)
	assertRouting(t, db, 1, 1, true)
	assertRouting(t, db, 2, 1, true)
	require.NoError(t, m.FinishTestProxy(context.Background(), *winner, false, false))
}

func Test408TestLeaseRequiresFreshHealthyRouteAndHonorsDisabledAllocation(t *testing.T) {
	for _, change := range []string{
		`UPDATE proxy_failover_health SET checked_at=NOW()-INTERVAL '1 hour'`,
		`UPDATE proxy_failover_health SET status='unknown'`,
		`UPDATE proxies SET expires_at=NOW()-INTERVAL '1 second' WHERE id=1`,
		`UPDATE proxies SET updated_at=NOW()+INTERVAL '1 second' WHERE id=1`,
	} {
		t.Run(change, func(t *testing.T) {
			db, m := quarantinedFixture(t)
			_, err := db.Exec(change)
			require.NoError(t, err)
			_, err = m.AcquireTestProxy(context.Background(), 10, "model-a")
			require.ErrorIs(t, err, ErrNoTestProxy)
			assertRouting(t, db, 10, 0, false)
		})
	}
	db, m := quarantinedFixture(t)
	_, err := db.Exec(`UPDATE proxy_auto_allocation SET enabled=false; UPDATE accounts SET proxy_id=2 WHERE id=10`)
	require.NoError(t, err)
	lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
	require.NoError(t, err)
	require.Nil(t, lease)
	assertRouting(t, db, 10, 2, false)
}
