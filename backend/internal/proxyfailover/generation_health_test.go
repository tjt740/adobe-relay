package proxyfailover

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/clash"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAdobeRouteReviewSkipsExhaustedAndDistinctSourceSiblings(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	state := clash.State{URL: "https://one.example/feed https://two.example/feed https://three.example/feed",
		Sources: []clash.Source{
			{ID: "one", Traffic: &clash.SourceTraffic{Total: 10, Download: 10, ObservedAt: time.Now()}},
			{ID: "two", Traffic: &clash.SourceTraffic{Total: 100, ObservedAt: time.Now()}},
			{ID: "three", Traffic: &clash.SourceTraffic{Total: 100, ObservedAt: time.Now()}},
		}, Nodes: []clash.Node{
			{Name: "one", ProxyID: 1, Enabled: true, SourceID: "one"},
			{Name: "two-a", ProxyID: 2, Enabled: true, SourceID: "two"},
			{Name: "two-b", ProxyID: 3, Enabled: true, SourceID: "two"},
			{Name: "three", ProxyID: 4, Enabled: true, SourceID: "three"},
		}}
	// Populate canonical IDs using the same documented SHA-256 prefix convention.
	for i, raw := range []string{"https://one.example/feed", "https://two.example/feed", "https://three.example/feed"} {
		old := state.Sources[i].ID
		state.Sources[i].ID = testSourceID(raw)
		for j := range state.Nodes {
			if state.Nodes[j].SourceID == old {
				state.Nodes[j].SourceID = state.Sources[i].ID
			}
		}
	}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE proxies SET status='active'`)
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	var revision time.Time
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=1`).Scan(&revision))
	source, ready, err := m.RouteReady(ctx, 1, &revision)
	require.NoError(t, err)
	require.Equal(t, state.Sources[0].ID, source)
	require.False(t, ready)
	first, err := m.AcquireTestProxy(WithTestRouteOptions(ctx, TestRouteOptions{ExcludeSources: []string{"proxy:force-review"}}), 10, "model-a")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotEqual(t, state.Sources[0].ID, first.SourceID, "exhausted feeds cost no Adobe submissions")
	require.NoError(t, m.FinishTestProxy(ctx, *first, false, false))
	second, err := m.AcquireTestProxy(WithTestRouteOptions(ctx, TestRouteOptions{ExcludeSources: []string{first.SourceID}, RequireClash: true}), 10, "model-a")
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.SourceID, second.SourceID)
	require.NoError(t, m.FinishTestProxy(ctx, *second, false, false))
	_, err = m.AcquireTestProxy(WithTestRouteOptions(ctx, TestRouteOptions{ExcludeSources: []string{first.SourceID, second.SourceID}, RequireClash: true}), 10, "model-a")
	require.ErrorIs(t, err, ErrNoTestProxy)
	assertRouting(t, db, 10, 0, false)
}

func TestAdobeGenerationCircuitExpiresWithoutSuccessfulTest(t *testing.T) {
	db, m := quarantinedFixture(t)
	ctx := context.Background()
	var revision time.Time
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=1`).Scan(&revision))
	record := func(account int64, scope string, success bool, started, checked time.Time) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx.Rollback()
		code := 408
		if success {
			code = 200
		}
		_, err = RecordGeneration(ctx, tx, 1, revision, "source-snapshot", account, "cookie", "", scope, success, code, "submit", !success, started, checked)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	for i := 0; i < 3; i++ {
		now := time.Now()
		record(1, "model-a", false, now, now)
	}
	ps, err := loadProxies(ctx, db)
	require.NoError(t, err)
	require.False(t, ps[1].generationBlocked, "one bad account cannot quarantine a node")
	for _, id := range []int64{2, 3} {
		now := time.Now()
		record(id, "model-a", false, now, now)
	}
	ps, err = loadProxies(ctx, db)
	require.NoError(t, err)
	require.True(t, ps[1].generationBlocked)
	require.True(t, allocationRouteAvailable(ps[1], time.Now(), false), "other platforms keep their network routing")
	require.NoError(t, m.check(ctx), "HEAD success cannot clear a generation circuit")
	_, ready, err := m.RouteReady(ctx, 1, &revision)
	require.NoError(t, err)
	require.False(t, ready)
	old := time.Now().Add(-time.Hour)
	record(1, "model-a", true, old, time.Now())
	now := time.Now()
	record(1, "model-b", true, now, now)
	_, ready, err = m.RouteReady(ctx, 1, &revision)
	require.NoError(t, err)
	require.False(t, ready, "old or different-model success cannot erase the incident")
	_, err = db.Exec(`UPDATE adobe_route_health SET retry_after=NOW()-INTERVAL '1 second' WHERE scope='model-a';DELETE FROM accounts WHERE id IN(1,2);
 INSERT INTO accounts(id,status,schedulable,credentials,extra) VALUES(11,'error',false,'{"cookie":"second"}','{}');
 INSERT INTO adobe_proxy_408_blocks(account_id,scope,failed_at) VALUES(11,'model-a',NOW())`)
	require.NoError(t, err)
	_, ready, err = m.RouteReady(ctx, 1, &revision)
	require.NoError(t, err)
	require.True(t, ready, "ordinary generation resumes after the bounded cooldown without test proof")
	ps, err = loadProxies(ctx, db)
	require.NoError(t, err)
	require.True(t, ps[1].available(time.Now()))
	require.NoError(t, m.check(ctx))
	var blocked bool
	require.NoError(t, db.QueryRow(`SELECT retry_after IS NOT NULL FROM adobe_route_health WHERE proxy_id=1 AND scope='model-a'`).Scan(&blocked))
	require.True(t, blocked, "timeout evidence remains stored even though the cooldown expired")

}

func testSourceID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:12])
}
