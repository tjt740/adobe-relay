package proxyfailover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/clash"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionPriorityOnlyUsesHealthyCapacity(t *testing.T) {
	now := time.Now()
	for _, mode := range []string{"healthy", "unhealthy", "stale", "exhausted", "excluded", "cooldown", "full"} {
		t.Run(mode, func(t *testing.T) {
			ps, hs := routeFixtures(now)
			p := ps[2]
			before := p.fingerprint()
			p.sourcePriority = 100
			require.Equal(t, before, p.fingerprint(), "metadata must not invalidate connectivity evidence")
			h := hs[2]
			h.LatencyMS = 9000
			accounts := []allocationAccount{{ID: 9, Eligible: true, Waiting: true, Adobe: true}}
			switch mode {
			case "unhealthy":
				h.Status = "unhealthy"
			case "stale":
				h.CheckedAt = now.Add(-time.Hour)
			case "exhausted":
				p.sourceBlocked = true
			case "excluded":
				p.excluded = true
			case "cooldown":
				p.retryAfter = now.Add(time.Minute)
			case "full":
				for id := int64(1); id <= 3; id++ {
					accounts = append(accounts, allocationAccount{ID: id, Current: 2, ManualProxy: true})
				}
			}
			ps[2], hs[2] = p, h
			expected := int64(1)
			if mode == "healthy" {
				expected = 2
			}
			require.Equal(t, expected, planAllocation(accounts, ps, hs, now)[9])
		})
	}
}

func TestSubscriptionPriorityPreservesStableManualAndPausedBindings(t *testing.T) {
	now := time.Now()
	ps, hs := routeFixtures(now)
	p := ps[2]
	p.sourcePriority = 100
	ps[2] = p
	accounts := []allocationAccount{{ID: 1, Current: 1, Eligible: true}, {ID: 2, Current: 1, ManualProxy: true}, {ID: 3, Current: 1, Eligible: false}, {ID: 4, Eligible: true, Waiting: true}}
	next := planAllocation(accounts, ps, hs, now)
	require.EqualValues(t, 1, next[1])
	require.NotContains(t, next, int64(2))
	require.NotContains(t, next, int64(3))
	require.EqualValues(t, 2, next[4])
	// Faster lower-priority sources must not draw a usable preferred route away.
	h := hs[2]
	h.LatencyEWMA = 9000
	h.LatencyMS = 9000
	h.SlowChecks = 3
	hs[2] = h
	require.EqualValues(t, 2, planAllocation([]allocationAccount{{ID: 1, Current: 2, Eligible: true}}, ps, hs, now)[1])
}

func TestStoredSubscriptionPriorityAffectsAllocationWithoutReload(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	u := "https://preferred.example/sub"
	sum := sha256.Sum256([]byte(u))
	source := hex.EncodeToString(sum[:12])
	state := clash.State{URL: u, Priorities: map[string]int{source: 100}, Nodes: []clash.Node{{Name: "Preferred", ProxyID: 2, SourceID: source, Enabled: true}}}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	ps, err := loadProxies(ctx, db)
	require.NoError(t, err)
	require.Equal(t, 100, ps[2].sourcePriority)
	_, err = db.Exec(`INSERT INTO accounts(id) VALUES(3),(4),(5),(6)`)
	require.NoError(t, err)
	m := NewManager(db)
	m.probe = func(_ context.Context, p proxy) health { return health{Status: "healthy", LatencyMS: p.ID * 100} }
	_, err = m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	for id := int64(3); id <= 5; id++ {
		var route int64
		require.NoError(t, db.QueryRow(`SELECT proxy_id FROM accounts WHERE id=$1`, id).Scan(&route))
		require.EqualValues(t, 2, route)
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_id=2`).Scan(&count))
	require.Equal(t, 3, count)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE id IN (1,2) AND proxy_id=1`).Scan(&count))
	require.Equal(t, 2, count)
}
