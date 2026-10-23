package proxyfailover

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestTransientSourceReviewDoesNotInvent408Quarantine(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		paused, new408, expired, success bool
	}{
		{"429 returns its temporary pause", false, false, false, false},
		{"expired review returns its own pause", false, false, true, false},
		{"successful transient review only returns its original eligibility", false, false, false, true},
		{"operator pause remains paused", true, false, false, false},
		{"actual new 408 remains quarantined", false, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			ctx := context.Background()
			m := NewManager(db)
			_, err := db.Exec(`INSERT INTO accounts(id,proxy_id,status,schedulable,credentials,extra) VALUES(10,1,'active',$1,'{"cookie":"fixture"}',jsonb_build_object('okad_manual_pause',$2::boolean))`, !tc.paused, tc.paused)
			require.NoError(t, err)
			_, err = m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
			require.NoError(t, err)
			m.probe = func(context.Context, proxy) health { return health{Status: "healthy", LatencyMS: 10} }
			require.NoError(t, m.check(ctx))
			lease, err := m.AcquireTestProxy(WithTestRouteOptions(ctx, TestRouteOptions{ExcludeSources: []string{"proxy:1"}}), 10, "model-a")
			require.NoError(t, err)
			require.NotNil(t, lease)
			var resume bool
			require.NoError(t, db.QueryRow(`SELECT return_schedulable FROM adobe_proxy_test_leases WHERE account_id=10`).Scan(&resume))
			require.Equal(t, !tc.paused, resume)
			until := time.Now().Add(15 * time.Second)
			_, err = db.Exec(`UPDATE accounts SET temp_unschedulable_until=$1 WHERE id=10`, until)
			require.NoError(t, err)
			if tc.new408 {
				_, err = db.Exec(`INSERT INTO adobe_proxy_408_blocks(account_id,scope,failed_at) VALUES(10,'model-a',NOW())`)
				require.NoError(t, err)
			}
			if tc.success {
				_, err = db.Exec(`INSERT INTO adobe_account_health(account_id,scope,success_started_at) VALUES(10,'model-a',clock_timestamp())`)
				require.NoError(t, err)
			}
			if tc.expired {
				_, err = db.Exec(`UPDATE adobe_proxy_test_leases SET expires_at=NOW()-INTERVAL '1 second' WHERE account_id=10`)
				require.NoError(t, err)
				require.NoError(t, m.releaseExpiredTests(ctx))
			} else {
				require.NoError(t, m.FinishTestProxy(ctx, *lease, tc.success, false))
			}
			var expectedProxy int64
			if tc.success {
				require.NoError(t, db.QueryRow(`SELECT COALESCE(proxy_id,0) FROM accounts WHERE id=10`).Scan(&expectedProxy))
				require.NotZero(t, expectedProxy)
			}
			assertRouting(t, db, 10, expectedProxy, !tc.paused && !tc.new408)
			var quarantined, manual bool
			var cooldown time.Time
			require.NoError(t, db.QueryRow(`SELECT EXISTS(SELECT 1 FROM adobe_proxy_408_holds WHERE account_id=10),extra->'okad_manual_pause'='true'::jsonb,temp_unschedulable_until FROM accounts WHERE id=10`).Scan(&quarantined, &manual, &cooldown))
			require.Equal(t, tc.new408, quarantined)
			require.Equal(t, tc.paused, manual)
			require.WithinDuration(t, until, cooldown, time.Millisecond)
		})
	}
}
