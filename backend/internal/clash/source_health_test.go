package clash

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestSourceTrafficEvidence(t *testing.T) {
	now := time.Now()
	for _, raw := range []string{"", "total=1", "upload=-1; download=0; total=4", "upload=0;download=0;total=999999999999999999999", "upload=0;upload=1;download=0;total=2"} {
		require.Nil(t, parseSourceTraffic(raw, now), raw)
	}
	traffic := parseSourceTraffic("upload=5; download=7; total=10; expire=0", now)
	require.NotNil(t, traffic)
	require.Zero(t, traffic.remaining())
	require.Equal(t, "exhausted", (Source{Traffic: traffic}).BlockReason(now))
	traffic.Total = 0
	require.Empty(t, (Source{Traffic: traffic}).BlockReason(now), "zero provider total must not be guessed to be an exhausted finite quota")
	traffic.Total = 100
	traffic.ExpiresAt = now.Add(-time.Second).Unix()
	require.Equal(t, "expired", (Source{Traffic: traffic}).BlockReason(now))
	old := Source{Traffic: &SourceTraffic{Total: 10, Download: 10}, LastSuccessAt: now, Failures: 1}
	observed := mergeSourceObservation(old, Source{FetchError: "connection_failed"})
	require.Equal(t, 2, observed.Failures)
	require.Equal(t, "exhausted", observed.BlockReason(now))
	require.Equal(t, now, observed.LastSuccessAt)
	renewed := mergeSourceObservation(old, Source{Traffic: &SourceTraffic{Total: 100}, LastSuccessAt: now})
	require.Zero(t, renewed.Failures)
	require.Empty(t, renewed.BlockReason(now))
}
func TestSourceIdentityAndRuntimeMarker(t *testing.T) {
	s := State{URL: "https://one.example/private?token=secret\nhttps://two.example/private?token=other", Revision: 1, Nodes: []Node{{Name: "订阅1 · A", Enabled: true}, {Name: "订阅2 · B", Enabled: true}}}
	before := marker(s)
	normalizeSources(&s)
	require.Len(t, s.Sources, 2)
	require.Equal(t, s.Sources[1].ID, s.Nodes[1].SourceID)
	require.NotContains(t, s.Sources[0].URLHint, "secret")
	require.Equal(t, before, marker(s), "telemetry and source identity must never reload Mihomo")
	require.NotEmpty(t, s.Nodes[0].SourceID, "marker must not mutate the caller's nodes")
	s.Sources[0].Failures = 100
	require.Equal(t, before, marker(s))
	require.Equal(t, 45*time.Second, ProbeFreshness(3))
	require.Equal(t, 165*time.Second, ProbeFreshness(350))
	require.Equal(t, 4*time.Minute, ProbeFreshness(10000))
}
func TestFetchSourceHeadersAndRedactedErrors(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "bad") {
			http.Error(w, "private payload", 403)
			return
		}
		w.Header().Set("subscription-userinfo", "upload=1;download=2;total=3")
		fmt.Fprint(w, "proxies:\n - {name: A, type: http, server: example.com, port: 80}\n")
	}))
	defer feed.Close()
	nodes, sources, err := fetchSubscriptionDetails(context.Background(), feed.Client(), feed.URL+"/good?token=private")
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "exhausted", sources[0].BlockReason(time.Now()))
	require.Equal(t, sources[0].ID, nodes[0].SourceID)
	_, sources, err = fetchSubscriptionDetails(context.Background(), feed.Client(), feed.URL+"/bad?token=private")
	require.NoError(t, err, "individual failures are returned as source outcomes")
	require.Equal(t, "http_error", sources[0].FetchError)
	encoded, err := json.Marshal(sources)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "token")
	require.NotContains(t, string(encoded), "private payload")

}
func sourceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PROXY_FAILOVER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set disposable PostgreSQL DSN")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("sources_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close(); root.Exec("DROP SCHEMA " + schema + " CASCADE"); root.Close() })
	_, err = db.Exec(`CREATE TABLE clash_subscription(id int PRIMARY KEY,document jsonb,updated_at timestamptz DEFAULT NOW());INSERT INTO clash_subscription(id,document) VALUES(1,'{}')`)
	require.NoError(t, err)
	return db
}
func TestSourceMonitorPreservesConfigAndKnownBlock(t *testing.T) {
	db := sourceTestDB(t)
	var deny atomic.Bool
	var quota atomic.Value
	quota.Store("upload=1;download=9;total=10")
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deny.Load() {
			http.Error(w, "no", 502)
			return
		}
		w.Header().Set("subscription-userinfo", quota.Load().(string))
		fmt.Fprint(w, "proxies:\n - {name: A, type: http, server: example.com, port: 80}\n")
	}))
	defer feed.Close()
	s := State{URL: feed.URL + "/secret", Revision: 7, UpdatedAt: time.Now().UTC(), Nodes: []Node{{Name: "A", ProxyID: 17, Enabled: true, Password: "listener-secret"}}}
	raw, _ := json.Marshal(s)
	_, err := db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	m := NewManager(db)
	m.fetchClient = feed.Client()
	require.NoError(t, m.refreshSources(context.Background()))
	got, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.EqualValues(t, 7, got.Revision)
	require.Equal(t, s.UpdatedAt, got.UpdatedAt)
	require.Equal(t, "listener-secret", got.Nodes[0].Password)
	routes, err := LoadProxySources(context.Background(), db)
	require.NoError(t, err)
	require.True(t, routes[17].Blocked)
	age := func() {
		t.Helper()
		past, _ := json.Marshal(time.Now().Add(-20 * time.Minute))
		_, e := db.Exec(`UPDATE clash_subscription SET document=jsonb_set(document,'{sources,0,checked_at}',$1::jsonb)`, string(past))
		require.NoError(t, e)
	}
	deny.Store(true)
	age()
	require.NoError(t, m.refreshSources(context.Background()))
	routes, err = LoadProxySources(context.Background(), db)
	require.NoError(t, err)
	require.True(t, routes[17].Blocked, "failed refresh cannot prove recovery")
	deny.Store(false)
	quota.Store("upload=0;download=0;total=100")
	age()
	require.NoError(t, m.refreshSources(context.Background()))
	routes, err = LoadProxySources(context.Background(), db)
	require.NoError(t, err)
	require.False(t, routes[17].Blocked)
}

func TestSourceHealthViewIsReadOnlyAndExplainsGroupCapacity(t *testing.T) {
	db := sourceTestDB(t)
	now := time.Now().UTC()
	s := State{URL: "https://provider.example/private?token=secret", Nodes: []Node{{Name: "A", ProxyID: 1, Enabled: true}}}
	normalizeSources(&s)
	s.Sources[0].Traffic = &SourceTraffic{Total: 10, Download: 10, ObservedAt: now}
	s.Sources[0].CheckedAt = now
	raw, _ := json.Marshal(s)
	_, err := db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE proxies(id bigint,deleted_at timestamptz,status text,expires_at timestamptz,updated_at timestamptz);
 CREATE TABLE proxy_failover_health(proxy_id bigint,status text,failures int,checked_at timestamptz);
 CREATE TABLE accounts(id bigint,proxy_id bigint,status text,schedulable bool,proxy_auto_paused bool,deleted_at timestamptz,expires_at timestamptz,auto_pause_on_expired bool,temp_unschedulable_until timestamptz,overload_until timestamptz,rate_limit_reset_at timestamptz);
 CREATE TABLE groups(id bigint PRIMARY KEY,name text,status text,platform text,deleted_at timestamptz);
 CREATE TABLE account_groups(account_id bigint,group_id bigint);
 INSERT INTO proxies VALUES(1,NULL,'active',NULL,NOW()-INTERVAL '1 minute');
 INSERT INTO proxy_failover_health VALUES(1,'unhealthy',2,NOW());
 INSERT INTO groups VALUES(7,'test','active','adobe',NULL);
 INSERT INTO accounts(id,proxy_id,status,schedulable,proxy_auto_paused) VALUES(1,1,'active',false,false),(2,NULL,'error',false,true);
 INSERT INTO account_groups VALUES(1,7),(2,7);`)
	require.NoError(t, err)
	m := NewManager(db)
	v, err := m.Health(context.Background())
	require.NoError(t, err)
	require.Len(t, v.Sources, 1)
	require.Equal(t, "exhausted", v.Sources[0].Status)
	require.Equal(t, 1, v.Sources[0].Failed)
	require.EqualValues(t, 0, *v.Sources[0].RemainingBytes)
	require.Len(t, v.Groups, 1)
	require.Equal(t, 0, v.Groups[0].Available)
	require.Equal(t, 0, v.Groups[0].Enabled)
	require.Equal(t, 1, v.Groups[0].WaitingProxy)
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "token=secret")
	after, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.EqualValues(t, 0, after.Revision)
}

func TestUnknownCountersAndMissingExpiryCannotClearKnownBlock(t *testing.T) {
	now := time.Now()
	old := Source{Traffic: &SourceTraffic{Total: 10, Download: 10, ObservedAt: now}}
	observed := mergeSourceObservation(old, Source{Traffic: &SourceTraffic{Total: 0, ObservedAt: now}})
	require.Equal(t, "exhausted", observed.BlockReason(now))
	old = Source{Traffic: &SourceTraffic{Total: 100, ExpiresAt: now.Add(-time.Hour).Unix(), ObservedAt: now}}
	observed = mergeSourceObservation(old, Source{Traffic: &SourceTraffic{Total: 100, ObservedAt: now}})
	require.Equal(t, "expired", observed.BlockReason(now))
	observed = mergeSourceObservation(old, Source{Traffic: &SourceTraffic{Total: 100, ExpiresAt: now.Add(time.Hour).Unix(), ObservedAt: now}})
	require.Empty(t, observed.BlockReason(now))
	onlyExpiry := parseSourceTraffic("expire=1", now)
	require.NotNil(t, onlyExpiry)
	require.Equal(t, "expired", (Source{Traffic: onlyExpiry}).BlockReason(now))
	require.Nil(t, parseSourceTraffic("upload=0;download=0;total=100;expire=9223372036854775807", now))
}
