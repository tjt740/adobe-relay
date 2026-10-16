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

func routeFixtures(now time.Time) (map[int64]proxy, map[int64]health) {
	ps := map[int64]proxy{1: {ID: 1, Status: "active"}, 2: {ID: 2, Status: "active"}}
	hs := map[int64]health{}
	for id, p := range ps {
		hs[id] = health{ProxyID: id, Status: "healthy", LatencyMS: 700, fingerprint: p.fingerprint(), CheckedAt: now, SuccessPermille: 1000}
	}
	return ps, hs
}
func TestBusinessAccountsReclaimInactiveCapacity(t *testing.T) {
	now := time.Now()
	ps, hs := routeFixtures(now)
	delete(ps, 2)
	delete(hs, 2)
	accounts := []allocationAccount{{ID: 1, Current: 1, Eligible: true, BindingOnly: true}, {ID: 2, Current: 1, Eligible: true, BindingOnly: true}, {ID: 3, Current: 1, Eligible: true, BindingOnly: true}, {ID: 99, Eligible: true, Waiting: true}}
	next := planAllocation(accounts, ps, hs, now)
	require.EqualValues(t, 1, next[99])
	require.Zero(t, next[3])
	require.EqualValues(t, 1, next[1])
	require.EqualValues(t, 1, next[2])
}
func TestStableSlowRouteSwitchAndFallback(t *testing.T) {
	now := time.Now()
	ps, hs := routeFixtures(now)
	h := hs[1]
	h.LatencyMS = 5100
	h.LatencyEWMA = 5000
	h.SlowChecks = 3
	hs[1] = h
	accounts := []allocationAccount{{ID: 1, Current: 1, Eligible: true}}
	require.EqualValues(t, 2, planAllocation(accounts, ps, hs, now)[1])
	accounts[0].LastSwitch = now.Add(-time.Minute)
	require.EqualValues(t, 1, planAllocation(accounts, ps, hs, now)[1], "soft switch cooldown")
	accounts[0].LastSwitch = time.Time{}
	h.SlowChecks = 2
	hs[1] = h
	require.EqualValues(t, 1, planAllocation(accounts, ps, hs, now)[1], "brief latency spike is insufficient")
	h.SlowChecks = 3
	hs[1] = h
	delete(ps, 2)
	delete(hs, 2)
	require.EqualValues(t, 1, planAllocation(accounts, ps, hs, now)[1], "a slow working node is preferable to no route")
	accounts = append(accounts, allocationAccount{ID: 2, Eligible: true, Waiting: true})
	require.EqualValues(t, 1, planAllocation(accounts, ps, hs, now)[2], "slow nodes remain a last usable fallback")
}
func TestBlockedSubscriptionAndLargePoolFreshness(t *testing.T) {
	now := time.Now()
	ps, hs := routeFixtures(now)
	p := ps[1]
	p.sourceBlocked = true
	ps[1] = p
	accounts := []allocationAccount{{ID: 1, Current: 1, Eligible: true, LastSwitch: now}}
	require.EqualValues(t, 2, planAllocation(accounts, ps, hs, now)[1], "known exhaustion bypasses soft cooldown")
	for id := int64(3); id <= 350; id++ {
		ps[id] = proxy{ID: id, Status: "inactive"}
	}
	h := hs[2]
	h.CheckedAt = now.Add(-90 * time.Second)
	hs[2] = h
	require.EqualValues(t, 2, planAllocation(accounts, ps, hs, now)[1], "large pools get a bounded full-sweep freshness window")
	h.CheckedAt = now.Add(-5 * time.Minute)
	hs[2] = h
	require.Zero(t, planAllocation(accounts, ps, hs, now)[1], "ancient health must never admit a new route")
}
func TestQualityObservationResetsOnEndpointEdit(t *testing.T) {
	now := time.Now()
	h := health{Status: "healthy", LatencyMS: 5000, CheckedAt: now, fingerprint: "same"}
	h = qualityObservation(health{}, h)
	require.Equal(t, 1, h.SlowChecks)
	for i := 0; i < 2; i++ {
		old := h
		h.CheckedAt = h.CheckedAt.Add(30 * time.Second)
		h = qualityObservation(old, h)
	}
	require.Equal(t, 3, h.SlowChecks)
	old := h
	h.fingerprint = "edited"
	h = qualityObservation(old, h)
	require.Equal(t, 1, h.SlowChecks)
	h.LatencyMS = 500
	h.CheckedAt = h.CheckedAt.Add(time.Second)
	h = qualityObservation(old, h)
	require.Zero(t, h.SlowChecks)
}
func TestRecoveryUsesCachedHealthAndPreservesManualPause(t *testing.T) {
	db := testDB(t)
	m := NewManager(db)
	ctx := context.Background()
	_, err := m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET status='active',schedulable=false,error_message=NULL,proxy_auto_paused=false WHERE id=1`)
	require.NoError(t, err)
	m.probe = func(context.Context, proxy) health { return health{Status: "healthy", LatencyMS: 500} }
	require.NoError(t, m.check(ctx))
	_, err = db.Exec(`INSERT INTO accounts(id) VALUES(3)`)
	require.NoError(t, err)
	m.probe = func(context.Context, proxy) health { t.Error("demand recovery must not probe"); return health{} }
	require.True(t, m.Recover(ctx))
	require.False(t, m.Recover(ctx), "demand repair is globally throttled")
	var paused, schedulable bool
	require.NoError(t, db.QueryRow(`SELECT proxy_auto_paused,schedulable FROM accounts WHERE id=3`).Scan(&paused, &schedulable))
	require.False(t, paused)
	require.True(t, schedulable)
	require.NoError(t, db.QueryRow(`SELECT schedulable FROM accounts WHERE id=1`).Scan(&schedulable))
	require.False(t, schedulable)
	// Source-wide exhaustion excludes every related proxy without a probe.
	sum := sha256.Sum256([]byte("https://subscription.example/secret"))
	sourceID := hex.EncodeToString(sum[:12])
	state := clash.State{URL: "https://subscription.example/secret", Sources: []clash.Source{{ID: sourceID, Traffic: &clash.SourceTraffic{Total: 1, Download: 1}}}, Nodes: []clash.Node{{ProxyID: 1, SourceID: sourceID, Enabled: true}}}
	raw, _ := json.Marshal(state)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	ps, err := loadProxies(ctx, db)
	require.NoError(t, err)
	require.True(t, ps[1].sourceBlocked)
	m.demandAt.Store(0)
	require.True(t, m.Recover(ctx))
	var remaining int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_id=1`).Scan(&remaining))
	require.Zero(t, remaining)
}

func TestNewBindingsPreferConfirmedTrafficAndKeepUnknownAsFallback(t *testing.T) {
	now := time.Now()
	ps, hs := routeFixtures(now)
	p := ps[2]
	p.sourceTrafficAvailable = true
	ps[2] = p
	h := hs[2]
	h.LatencyMS = 900
	hs[2] = h
	accounts := []allocationAccount{{ID: 1, Eligible: true, Waiting: true}}
	require.EqualValues(t, 2, planAllocation(accounts, ps, hs, now)[1])
	p.sourceBlocked = true
	ps[2] = p
	require.EqualValues(t, 1, planAllocation(accounts, ps, hs, now)[1], "unknown traffic with fresh connectivity remains usable when no confirmed-balance route is available")
}
