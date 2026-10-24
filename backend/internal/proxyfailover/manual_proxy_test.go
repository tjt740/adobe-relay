package proxyfailover

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManualBindingsReserveCapacityWithoutBeingManaged(t *testing.T) {
	now := time.Now()
	ps := map[int64]proxy{1: {ID: 1, Status: "active"}}
	hs := map[int64]health{1: {Status: "healthy", CheckedAt: now, fingerprint: ps[1].fingerprint()}}
	accounts := []allocationAccount{
		{ID: 1, Current: 1, ManualProxy: true, Eligible: true},
		{ID: 2, Current: 1, ManualProxy: true, BindingOnly: true},
		{ID: 3, ManualProxy: true, Eligible: true}, // Explicit direct routing stays direct.
		{ID: 4, Eligible: true, Waiting: true},
		{ID: 5, Eligible: true, Waiting: true},
	}
	next := planAllocation(accounts, ps, hs, now)
	for _, id := range []int64{1, 2, 3} {
		require.NotContains(t, next, id)
	}
	require.EqualValues(t, 1, next[4])
	require.Zero(t, next[5], "manual bindings consume the same node's capacity")
}

func TestManual408BindingSurvivesMaintenanceAndReturnsToNormalAllocation(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE accounts SET proxy_id=1,extra=extra||'{"manual_proxy_enabled":true}' WHERE id=10`)
	require.NoError(t, err)
	assertRouting(t, db, 10, 1, false) // Enabling the mode does not resume the account.
	for i := 0; i < 2; i++ {
		require.NoError(t, m.check(ctx))
	}
	assertRouting(t, db, 10, 1, false)
	lease, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	require.Nil(t, lease, "manual checks use the selected route without borrowing a replacement")
	var manual bool
	require.NoError(t, db.QueryRow(`SELECT extra->'manual_proxy_enabled'='true'::jsonb AND NOT (extra ? 'adobe_proxy_408') FROM accounts WHERE id=10`).Scan(&manual))
	require.True(t, manual)
	_, err = db.Exec(`UPDATE accounts SET schedulable=true WHERE id=10`)
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 10, 1, true)
	_, err = db.Exec(`UPDATE accounts SET extra=jsonb_set(extra,'{manual_proxy_enabled}','false') WHERE id=10`)
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 10, 1, true)
	var blocks int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_408_blocks WHERE account_id=10`).Scan(&blocks))
	require.Equal(t, 1, blocks, "mode changes never erase unresolved model failures")
}

func TestManualAccountsBypassGlobalActivationAndLegacyFailover(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	_, err := m.Save(ctx, 1, Policy{Enabled: true, PrimaryProxyID: 1, BackupProxyIDs: []int64{2}, CurrentProxyID: 1})
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET extra='{"manual_proxy_enabled":true}' WHERE id=1;
 INSERT INTO accounts(id,proxy_id,extra) VALUES(9,NULL,'{"manual_proxy_enabled":true}');`)
	require.NoError(t, err)
	_, err = m.Save(ctx, 1, Policy{Enabled: true, PrimaryProxyID: 1, BackupProxyIDs: []int64{2}, CurrentProxyID: 1, Revision: 1})
	require.ErrorIs(t, err, ErrManualProxy)
	v, err := m.Get(ctx, 1)
	require.NoError(t, err)
	require.True(t, v.ManualProxy)
	require.Equal(t, "manual", v.State)
	m.probe = func(_ context.Context, p proxy) health {
		if p.ID == 1 {
			return health{Status: "unhealthy", Failures: 2}
		}
		return health{Status: "healthy"}
	}
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 1, 1, true)
	_, err = m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 1, 1, true)
	assertRouting(t, db, 9, 0, true)
	// A policy selected before the operator toggled manual mode cannot switch later.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	require.NoError(t, m.switchProxy(ctx, tx, accountPolicy{ID: 1, Policy: Policy{CurrentProxyID: 1}}, 2, nil, nil))
	require.NoError(t, tx.Commit())
	assertRouting(t, db, 1, 1, true)
}

func TestInFlightTestCannotReleaseNewManualOwnership(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	lease, err := m.AcquireTestProxy(ctx, 10, "model-a")
	require.NoError(t, err)
	require.NotNil(t, lease)
	_, err = db.Exec(`UPDATE accounts SET extra=extra||'{"manual_proxy_enabled":true}' WHERE id=10`)
	require.NoError(t, err)
	require.NoError(t, m.FinishTestProxy(ctx, *lease, false, false))
	assertRouting(t, db, 10, 1, false)
	require.NoError(t, m.check(ctx))
	assertRouting(t, db, 10, 1, false)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_proxy_test_leases WHERE account_id=10`).Scan(&count))
	require.Zero(t, count)
}

func Test408CheckCannotBorrowAManuallyPinnedInactiveBinding(t *testing.T) {
	db, m := quarantinedFixture(t)
	_, err := db.Exec(`INSERT INTO accounts(id,proxy_id,status,schedulable,extra) VALUES(3,1,'disabled',false,'{"manual_proxy_enabled":true}')`)
	require.NoError(t, err)
	lease, err := m.AcquireTestProxy(context.Background(), 10, "model-a")
	require.ErrorIs(t, err, ErrNoTestProxy)
	require.Nil(t, lease)
	assertRouting(t, db, 3, 1, false)
	assertRouting(t, db, 10, 0, false)
}
