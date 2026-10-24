package proxyfailover

import (
	"context"
	"database/sql"
	"github.com/stretchr/testify/require"
	"testing"
)

// Legacy 408 rows remain harmless even during a rolling upgrade. This fixture
// keeps the old stopped-account shape for source/manual ownership regressions.
func quarantinedFixture(t *testing.T) (*sql.DB, *Manager) {
	t.Helper()
	db := testDB(t)
	_, err := db.Exec(`UPDATE proxies SET status='inactive' WHERE id<>1;
 INSERT INTO accounts(id,status,schedulable,credentials,extra) VALUES
 (10,'active',false,'{"cookie":"fixture-cookie","model_mapping":{"keep":"unchanged"}}','{"custom":"keep"}');
 INSERT INTO adobe_account_health(account_id,scope,failed_at,status_code) VALUES(10,'model-a',NOW()-INTERVAL '1 hour',408);
 INSERT INTO adobe_proxy_408_blocks SELECT account_id,scope,failed_at FROM adobe_account_health;`)
	require.NoError(t, err)
	m := NewManager(db)
	_, err = m.SaveAllocation(context.Background(), AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	m.probe = func(context.Context, proxy) health { return health{Status: "healthy", LatencyMS: 20} }
	require.NoError(t, m.check(context.Background()))
	_, err = db.Exec(`UPDATE accounts SET proxy_id=NULL WHERE id=10`)
	require.NoError(t, err)
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
	_, err := db.Exec(`INSERT INTO adobe_account_health(account_id,scope,success_started_at) VALUES($1,$2,clock_timestamp())
 ON CONFLICT(account_id,scope) DO UPDATE SET success_started_at=clock_timestamp()`, id, scope)
	require.NoError(t, err)
}
func TestLegacy408DoesNotRemoveOrdinaryBinding(t *testing.T) {
	db, m := quarantinedFixture(t)
	_, err := db.Exec(`SELECT set_config('sub2api.proxy_allocator','on',false);
 UPDATE accounts SET proxy_id=1,schedulable=true WHERE id=10`)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		require.NoError(t, m.check(context.Background()))
	}
	assertRouting(t, db, 10, 1, true)
	lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
	require.NoError(t, err)
	require.Nil(t, lease, "ordinary healthy bindings need no 408 recovery lease")
	v, err := m.GetAllocation(context.Background())
	require.NoError(t, err)
	require.Zero(t, v.Quarantined408Accounts)
	require.Zero(t, v.Pending408Release)
}
func TestTemporaryReviewReturnsOriginalSchedulingOnFailureAndExpiry(t *testing.T) {
	for _, mutation := range []string{"timeout", "expiry", "manual_pause", "credentials", "new_error", "verification"} {
		t.Run(mutation, func(t *testing.T) {
			db := testDB(t)
			m := NewManager(db)
			ctx := context.Background()
			_, err := m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
			require.NoError(t, err)
			m.probe = func(context.Context, proxy) health { return health{Status: "healthy", LatencyMS: 20} }
			require.NoError(t, m.check(ctx))
			lease, err := m.AcquireTestProxy(WithTestRouteOptions(ctx, TestRouteOptions{ExcludeSources: []string{"proxy:1"}}), 1, "model-a")
			require.NoError(t, err)
			require.NotNil(t, lease)
			mutations := map[string]string{
				"timeout":      `INSERT INTO adobe_proxy_408_blocks VALUES(1,'model-a',NOW())`,
				"expiry":       `UPDATE adobe_proxy_test_leases SET expires_at=NOW()-INTERVAL '1 second' WHERE account_id=1`,
				"manual_pause": `UPDATE accounts SET extra=COALESCE(extra,'{}')||'{"okad_manual_pause":true}' WHERE id=1`,
				"credentials":  `UPDATE accounts SET credentials='{"cookie":"replacement"}' WHERE id=1`,
				"new_error":    `UPDATE accounts SET status='error',error_message='Authentication failed (401): invalid token' WHERE id=1`,
				"verification": `UPDATE accounts SET extra=COALESCE(extra,'{}')||'{"okad_connection_test":{"gated":true,"state":"running"}}' WHERE id=1`,
			}
			_, err = db.Exec(mutations[mutation])
			require.NoError(t, err)
			if mutation == "expiry" {
				require.NoError(t, m.releaseExpiredTests(ctx))
			} else {
				require.NoError(t, m.FinishTestProxy(ctx, *lease, false, false))
			}
			var sched bool
			require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&sched))
			require.Equal(t, mutation == "timeout" || mutation == "expiry", sched)
			var leases int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_test_leases WHERE account_id=1`).Scan(&leases))
			require.Zero(t, leases)
			if mutation == "timeout" || mutation == "expiry" {
				require.NoError(t, m.check(ctx))
				var bound bool
				require.NoError(t, db.QueryRow(`SELECT proxy_id IS NOT NULL AND schedulable FROM accounts WHERE id=1`).Scan(&bound))
				require.True(t, bound)
			}
		})
	}
}
func TestTemporaryReviewStaleOwnerCannotCompleteNewLease(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	first, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	require.NotNil(t, first)
	_, err = db.Exec(`UPDATE adobe_proxy_test_leases SET expires_at=NOW()-INTERVAL '1 second'`)
	require.NoError(t, err)
	require.NoError(t, m.releaseExpiredTests(ctx))
	second, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.Owner, second.Owner)
	require.NoError(t, m.FinishTestProxy(ctx, *first, false, false))
	var owner string
	require.NoError(t, db.QueryRow(`SELECT owner FROM adobe_proxy_test_leases WHERE account_id=10`).Scan(&owner))
	require.Equal(t, second.Owner, owner)
}

func TestConcurrentTemporaryReviewsKeepCapacityAndBusinessBindings(t *testing.T) {
	db, m := quarantinedFixture(t)
	_, err := db.Exec(`INSERT INTO accounts(id,status,schedulable,credentials) VALUES(11,'active',false,'{"cookie":"second"}')`)
	require.NoError(t, err)
	type result struct {
		lease *TestLease
		err   error
	}
	results := make(chan result, 2)
	for _, id := range []int64{10, 11} {
		go func(id int64) {
			l, e := m.AcquireTestProxy(context.Background(), id, "model-a")
			results <- result{l, e}
		}(id)
	}
	wins, failures := 0, 0
	for range 2 {
		r := <-results
		if r.err != nil {
			require.ErrorIs(t, r.err, ErrNoTestProxy)
			failures++
		} else {
			require.NotNil(t, r.lease)
			wins++
			_, e := m.AcquireTestProxy(context.Background(), r.lease.AccountID, "model-a")
			require.ErrorIs(t, e, ErrTestBusy)
		}
	}
	require.Equal(t, 1, wins)
	require.Equal(t, 1, failures)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_id=1`).Scan(&count))
	require.Equal(t, 3, count)
	assertRouting(t, db, 1, 1, true)
	assertRouting(t, db, 2, 1, true)
}

func TestTemporaryReviewNeedsFreshRouteAndHonorsDisabledAllocation(t *testing.T) {
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
	_, err := db.Exec(`UPDATE proxy_auto_allocation SET enabled=false;UPDATE accounts SET proxy_id=2 WHERE id=10`)
	require.NoError(t, err)
	lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
	require.NoError(t, err)
	require.Nil(t, lease)
	assertRouting(t, db, 10, 2, false)
}
