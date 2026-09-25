package proxyfailover

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestChooseOnlyFreshHealthyBackups(t *testing.T) {
	now := time.Now()
	ps := map[int64]proxy{1: {ID: 1, Status: "active"}, 2: {ID: 2, Status: "active"}, 3: {ID: 3, Status: "active"}}
	hs := map[int64]health{}
	for id, p := range ps {
		hs[id] = health{ProxyID: id, Status: "healthy", CheckedAt: now, fingerprint: p.fingerprint()}
	}
	p := Policy{PrimaryProxyID: 1, BackupProxyIDs: []int64{2, 3}, CurrentProxyID: 1}
	require.Zero(t, choose(p, ps, hs, now), "healthy primary must remain pinned")
	h := hs[1]
	h.Status = "unhealthy"
	h.Failures = 1
	hs[1] = h
	require.Zero(t, choose(p, ps, hs, now), "single failure must not switch")
	h.Failures = 2
	hs[1] = h
	require.EqualValues(t, 2, choose(p, ps, hs, now))
	b := hs[2]
	b.CheckedAt = now.Add(-time.Minute)
	hs[2] = b
	require.EqualValues(t, 3, choose(p, ps, hs, now), "skip stale measurements")
	c := ps[3]
	c.UpdatedAt = now
	ps[3] = c
	require.Zero(t, choose(p, ps, hs, now), "edited proxy must be rechecked")
	b.CheckedAt = now
	hs[2] = b
	c = ps[2]
	c.ExpiresAt = sql.NullTime{Time: now.Add(-time.Second), Valid: true}
	ps[2] = c
	require.Zero(t, choose(p, ps, hs, now), "expired backups never qualify")
	p.CurrentProxyID = 3
	hs[3] = health{Status: "healthy", CheckedAt: now, fingerprint: ps[3].fingerprint()}
	require.Zero(t, choose(p, ps, hs, now), "never fail back while current route is healthy")
}

func TestProbeClassification(t *testing.T) {
	for _, code := range []int{200, 204, 401, 404, 405} {
		status, _ := classifyStatus(code)
		require.Equal(t, "healthy", status)
	}
	for _, code := range []int{403, 407} {
		status, _ := classifyStatus(code)
		require.Equal(t, "unhealthy", status)
	}
	for _, code := range []int{301, 429, 500, 502, 503} {
		status, _ := classifyStatus(code)
		require.Equal(t, "unknown", status)
	}
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "CONNECT", r.Method)
		w.WriteHeader(407)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	h := probeAdobe(context.Background(), proxy{Status: "active", Protocol: "http", Host: u.Hostname(), Port: port})
	require.Equal(t, "unhealthy", h.Status)
	require.Positive(t, calls.Load())
	h = probeAdobe(context.Background(), proxy{Status: "inactive", Protocol: "http", Host: u.Hostname(), Port: port})
	require.Equal(t, "proxy_unavailable", h.Message)
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PROXY_FAILOVER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PROXY_FAILOVER_TEST_DATABASE_URL to a disposable PostgreSQL instance")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("failover_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() { root.Exec("DROP SCHEMA " + schema + " CASCADE"); root.Close() })
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE proxies(id BIGINT PRIMARY KEY,protocol text DEFAULT 'http',host text DEFAULT 'localhost',port int DEFAULT 8080,username text,password text,status text DEFAULT 'active',expires_at timestamptz,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
 CREATE TABLE accounts(id BIGINT PRIMARY KEY,proxy_id bigint,platform text DEFAULT 'adobe',type text DEFAULT 'oauth',status text DEFAULT 'active',schedulable bool DEFAULT true,expires_at timestamptz,deleted_at timestamptz,updated_at timestamptz DEFAULT NOW(),proxy_fallback_origin_id bigint);
 CREATE TABLE scheduler_outbox(id BIGSERIAL PRIMARY KEY,event_type text,payload jsonb);
 INSERT INTO proxies(id) VALUES(1),(2),(3),(4);
 INSERT INTO accounts(id,proxy_id) VALUES(1,1),(2,1);`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/241_account_proxy_failover.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return db
}

func TestFailoverIntegration(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	policy := Policy{Enabled: true, PrimaryProxyID: 1, BackupProxyIDs: []int64{2, 3}, CurrentProxyID: 1}
	_, err := m.Save(ctx, 1, policy)
	require.NoError(t, err)
	_, err = m.Save(ctx, 2, policy)
	require.NoError(t, err)
	var calls atomic.Int64
	m.probe = func(_ context.Context, p proxy) health {
		calls.Add(1)
		if p.ID != 3 {
			return health{Status: "unhealthy", Message: "connection_failed"}
		}
		return health{Status: "healthy", LatencyMS: 70}
	}
	require.NoError(t, m.check(ctx))
	require.EqualValues(t, 3, calls.Load(), "shared proxies checked once per round")
	v, err := m.Get(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, v.CurrentProxyID)
	require.NoError(t, m.check(ctx))
	require.EqualValues(t, 3, calls.Load(), "cannot accumulate failures by repeated calls")
	age := func() {
		_, err := db.Exec("UPDATE proxy_failover_health SET checked_at=NOW()-INTERVAL '31 seconds'")
		require.NoError(t, err)
	}
	age()
	require.NoError(t, m.check(ctx))
	v, err = m.Get(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 3, v.CurrentProxyID)
	require.Len(t, v.Events, 1)
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM scheduler_outbox").Scan(&count))
	require.Equal(t, 2, count)
	require.Equal(t, "healthy", v.State)
	m.probe = func(_ context.Context, p proxy) health { return health{Status: "healthy"} }
	age()
	require.NoError(t, m.check(ctx))
	v, err = m.Get(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 3, v.CurrentProxyID, "primary recovery must not trigger failback")
	// Manual proxy changes outside the pool are respected.
	_, err = db.Exec("UPDATE accounts SET proxy_id=4 WHERE id=1")
	require.NoError(t, err)
	v, err = m.Get(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "paused", v.State)
	_, err = m.Save(ctx, 1, policy)
	require.ErrorIs(t, err, ErrConflict)
	// All nodes failing must never clear proxy_id or route direct.
	m.probe = func(_ context.Context, p proxy) health { return health{Status: "unhealthy"} }
	age()
	require.NoError(t, m.check(ctx))
	age()
	require.NoError(t, m.check(ctx))
	v, err = m.Get(ctx, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, v.CurrentProxyID)
	require.Equal(t, "no_backup", v.State)
	// Policies use optimistic concurrency, including when disabled.
	v.Enabled = false
	_, err = m.Save(ctx, 2, v.Policy)
	require.NoError(t, err)
	_, err = m.Save(ctx, 2, v.Policy)
	require.ErrorIs(t, err, ErrConflict)
}

func TestConcurrentChangesAndAtomicOutbox(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	_, err := m.Save(ctx, 1, Policy{Enabled: true, PrimaryProxyID: 1, BackupProxyIDs: []int64{2}, CurrentProxyID: 1})
	require.NoError(t, err)
	m.probe = func(_ context.Context, p proxy) health {
		if p.ID == 1 {
			return health{Status: "unhealthy"}
		}
		return health{Status: "healthy"}
	}
	require.NoError(t, m.check(ctx))
	_, err = db.Exec("UPDATE proxy_failover_health SET checked_at=NOW()-INTERVAL '31 seconds'")
	require.NoError(t, err)
	original := m.probe
	var once sync.Once
	m.probe = func(ctx context.Context, p proxy) health {
		once.Do(func() {
			_, e := db.Exec("UPDATE proxies SET updated_at=NOW(),port=8081 WHERE id=2")
			require.NoError(t, e)
		})
		return original(ctx, p)
	}
	require.NoError(t, m.check(ctx))
	v, err := m.Get(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, v.CurrentProxyID, "configuration changed during probe")
	m.probe = original
	// Make outbox insertion fail; account, history, and health must roll back together.
	_, err = db.Exec("ALTER TABLE scheduler_outbox ADD CONSTRAINT reject_event CHECK(event_type='reject')")
	require.NoError(t, err)
	require.Error(t, m.check(ctx))
	v, err = m.Get(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, v.CurrentProxyID)
	require.Empty(t, v.Events)
	_, err = db.Exec("ALTER TABLE scheduler_outbox DROP CONSTRAINT reject_event")
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	v, err = m.Get(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, v.CurrentProxyID)
	// A second replica cannot probe while leadership is held elsewhere.
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec("SELECT pg_advisory_xact_lock(241092601)")
	require.NoError(t, err)
	m.probe = func(context.Context, proxy) health { t.Error("duplicate replica probe"); return health{} }
	require.NoError(t, m.check(ctx))
}

// Opt-in diagnostic: run inside the deployment network to inspect real nodes.
// Targets are host:port only; account cookies and generation endpoints are never POSTed.
func TestLiveAdobeProbe(t *testing.T) {
	targets := os.Getenv("PROXY_FAILOVER_LIVE_TARGETS")
	if targets == "" {
		t.Skip("set PROXY_FAILOVER_LIVE_TARGETS for live HEAD connectivity checks")
	}
	for _, target := range strings.Split(targets, ",") {
		u, err := url.Parse("http://" + target)
		require.NoError(t, err)
		port, err := strconv.Atoi(u.Port())
		require.NoError(t, err)
		h := probeAdobe(context.Background(), proxy{Protocol: "http", Host: u.Hostname(), Port: port, Status: "active", Username: os.Getenv("PROXY_FAILOVER_LIVE_USER"), Password: os.Getenv("PROXY_FAILOVER_LIVE_PASSWORD")})
		t.Logf("%s: %s, %d ms (%s)", target, h.Status, h.LatencyMS, h.Message)
	}
}
