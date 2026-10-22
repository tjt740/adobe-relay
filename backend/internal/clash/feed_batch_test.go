package clash

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManyFeedsHaveIndependentOutcomesAndSharedConcurrency(t *testing.T) {
	var active, peak, calls atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); current > old && !peak.CompareAndSwap(old, current); old = peak.Load() {
		}
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		if r.URL.Path == "/3" {
			http.Error(w, "secret-provider-body", 503)
			return
		}
		fmt.Fprint(w, "proxies: [{name: Same, type: http, server: example.com, port: 80}]")
	}))
	defer feed.Close()
	urls := []string{}
	for i := range 24 {
		urls = append(urls, fmt.Sprintf("%s/%d?token=private", feed.URL, i))
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes, sources, err := fetchSubscriptionDetails(context.Background(), feed.Client(), strings.Join(urls, "\n"))
			require.NoError(t, err)
			require.Len(t, sources, 24)
			require.Len(t, nodes, 23)
			require.Equal(t, "http_error", sources[3].FetchError)
			require.Equal(t, "订阅24 · Same", nodes[len(nodes)-1].Name)
			encoded, _ := json.Marshal(sources)
			require.NotContains(t, string(encoded), "private")
			require.NotContains(t, string(encoded), "secret-provider-body")
		}()
	}
	wg.Wait()
	require.EqualValues(t, 48, calls.Load())
	require.Greater(t, peak.Load(), int32(1))
	require.LessOrEqual(t, peak.Load(), int32(sourceConcurrency))
}

func TestFailedAndReorderedFeedsKeepNodeIdentity(t *testing.T) {
	previous := State{URL: "https://one.example/sub\nhttps://two.example/sub", Nodes: []Node{
		{Name: "订阅1 · Same", ProxyID: 17, Port: 20000, Password: "listener-a", Enabled: true},
		{Name: "订阅2 · Same", ProxyID: 28, Port: 20001, Password: "listener-b", Enabled: true},
	}}
	normalizeSources(&previous)
	fresh := Node{Name: "订阅1 · Same", OriginalName: "Same", SourceID: previous.Sources[1].ID, Config: map[string]any{"name": "订阅1 · Same"}}
	sources := []Source{{ID: previous.Sources[1].ID}, {ID: previous.Sources[0].ID, FetchError: "http_error"}}
	nodes, retained, err := preserveFailedSources(previous, []Node{fresh}, sources)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, "订阅2 · Same", nodes[0].Name, "URL order must not exchange node identities")
	require.Equal(t, "订阅1 · Same", nodes[1].Name)
	require.EqualValues(t, 17, nodes[1].ProxyID)
	require.Equal(t, "listener-a", nodes[1].Password)
	require.True(t, retained[nodes[1].Name])
	require.Equal(t, "订阅1 · Same", fresh.Config["name"], "candidate renaming must not mutate caller configs")
}

func TestFeedQueueCancellationAndDuplicateURLs(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer feed.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	results := 0
	jobs := make([]feedJob, 200)
	for i := range jobs {
		jobs[i] = feedJob{url: feed.URL, index: i + 1}
	}
	fetchFeeds(ctx, feed.Client(), jobs, func(feedResult) { results++ })
	require.Zero(t, results, "a cancelled batch cannot classify waiting feeds as failures")
	require.Empty(t, sourceFetchSlots, "all network permits must be released")
	urls, err := subscriptionURLs("https://one.example/sub\nhttps://one.example/sub\nhttps://two.example/sub")
	require.NoError(t, err)
	require.Len(t, urls, 2)
	_, err = subscriptionURLs(strings.Repeat("a", (64<<10)+1))
	require.Error(t, err)
}

func TestSourceCheckDoesNotRequireOrChangeSavedConfiguration(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "proxies: [{name: A, type: http, server: example.com, port: 80}]")
	}))
	defer feed.Close()
	m := NewManager(nil)
	m.fetchClient = feed.Client()
	src, err := m.CheckSource(context.Background(), feed.URL)
	require.NoError(t, err)
	require.Empty(t, src.FetchError)
	require.Equal(t, 1, src.NodeCount)
	_, err = m.CheckSource(context.Background(), feed.URL+"\n"+feed.URL+"/other")
	require.Error(t, err)
}

func TestRefreshSavesCompletedFeedsWhenAnotherExceedsBatchBudget(t *testing.T) {
	db := sourceTestDB(t)
	var idleTransactions atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var count int32
		if err := db.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='idle in transaction'`).Scan(&count); err == nil && count > 0 {
			idleTransactions.Store(count)
		}
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/bad" {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, "proxies: [{name: A, type: http, server: example.com, port: 80}]")
	}))
	defer feed.Close()
	previous := State{URL: feed.URL + "/fast\n" + feed.URL + "/bad\n" + feed.URL + "/slow", Revision: 11}
	normalizeSources(&previous)
	old := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	for i := range previous.Sources {
		previous.Sources[i].CheckedAt = old
	}
	raw, _ := json.Marshal(previous)
	_, err := db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	m := NewManager(db)
	m.fetchClient = feed.Client()
	require.NoError(t, m.refreshSourcesWithin(context.Background(), 150*time.Millisecond))
	current, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.True(t, current.Sources[0].CheckedAt.After(old))
	require.Equal(t, "http_error", current.Sources[1].FetchError)
	require.Equal(t, old, current.Sources[2].CheckedAt, "unfinished feed remains due instead of becoming a false failure")
	require.Zero(t, current.Sources[2].Failures)
	require.Equal(t, previous.Revision, current.Revision)
	require.Equal(t, previous.UpdatedAt, current.UpdatedAt)
	require.Zero(t, idleTransactions.Load(), "network fetching must not leave a SQL transaction open")
}

func TestPartialImportPreservesFailedBindingsAndAllFailureIsNoOp(t *testing.T) {
	db := sourceTestDB(t)
	_, err := db.Exec(`CREATE TABLE proxies(id BIGSERIAL PRIMARY KEY,name text,protocol text,host text,port int,username text,password text,status text,backup_proxy_id bigint,fallback_mode text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
CREATE TABLE accounts(id BIGSERIAL PRIMARY KEY,proxy_id bigint,proxy_fallback_origin_id bigint,extra jsonb,status text,schedulable bool,error_message text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
CREATE TABLE account_proxy_failover(account_id bigint,primary_proxy_id bigint,backup_proxy_ids jsonb,revision int,updated_at timestamptz);
CREATE TABLE scheduler_outbox(event_type text,payload jsonb);`)
	require.NoError(t, err)
	var allFail, transientFail atomic.Bool
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allFail.Load() || r.URL.Path == "/bad" || (transientFail.Load() && r.URL.Path == "/transient") {
			http.Error(w, "offline", 502)
			return
		}
		name := "A"
		if r.URL.Path == "/transient" {
			name = "T"
		}
		fmt.Fprintf(w, "proxies: [{name: %s, type: http, server: example.com, port: 80}]", name)
	}))
	defer feed.Close()
	// Disposable controller and TCP listeners verify SQL/selection preservation.
	// They deliberately do not claim to exercise real Mihomo routing.
	listeners := map[int]net.Listener{}
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Payload string `json:"payload"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			w.WriteHeader(400)
			return
		}
		var config struct {
			Listeners []struct {
				Port int `json:"port"`
			} `json:"listeners"`
		}
		if json.Unmarshal([]byte(request.Payload), &config) != nil {
			w.WriteHeader(400)
			return
		}
		for _, entry := range config.Listeners {
			if listeners[entry.Port] != nil {
				continue
			}
			listener, e := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(entry.Port))
			if e != nil {
				w.WriteHeader(500)
				return
			}
			listeners[entry.Port] = listener
			go func() {
				for {
					c, e := listener.Accept()
					if e != nil {
						return
					}
					c.Close()
				}
			}()
		}
		w.WriteHeader(204)
	}))
	defer func() {
		controller.Close()
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	m := NewManager(db)
	m.controller, m.secret, m.host, m.portStart = controller.URL, "test-only", "127.0.0.1", 28530
	m.fetchClient = feed.Client()
	urls := feed.URL + "/good\n" + feed.URL + "/bad"
	// Initially both saved nodes predate the failing provider.
	previous := State{URL: urls, Revision: 3, Nodes: []Node{
		{Name: "订阅1 · A", ProxyID: 1, Port: 28530, Password: "keep-a", Enabled: true, Config: map[string]any{"name": "订阅1 · A", "type": "http", "server": "example.com", "port": 80}},
		{Name: "订阅2 · B", ProxyID: 2, Port: 28531, Password: "keep-b", Enabled: true, Config: map[string]any{"name": "订阅2 · B", "type": "http", "server": "example.com", "port": 80}},
		{Name: "订阅2 · Disabled", ProxyID: 3, Port: 28532, Password: "keep-disabled", Enabled: false, Config: map[string]any{"name": "订阅2 · Disabled", "type": "http", "server": "example.com", "port": 80}},
	}}
	raw, _ := json.Marshal(previous)
	_, err = db.Exec(`INSERT INTO proxies(id,status) VALUES(1,'active'),(2,'active'),(3,'inactive'); INSERT INTO accounts(proxy_id,status,schedulable,extra) VALUES(2,'active',true,'{"manual":"preserve"}');`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	preview, err := m.Preview(context.Background(), urls)
	require.NoError(t, err)
	require.Len(t, preview.Nodes, 2)
	require.True(t, preview.Nodes[1].Retained)
	// A client cannot accidentally remove a failed source even when names is empty.
	v, err := m.Import(context.Background(), ImportRequest{URL: urls, Names: []string{}, Revision: 3})
	require.NoError(t, err)
	require.Len(t, v.Nodes, 1)
	require.EqualValues(t, 2, v.Nodes[0].ProxyID)
	var proxyID int64
	var schedulable bool
	var extra string
	require.NoError(t, db.QueryRow(`SELECT proxy_id,schedulable,extra::text FROM accounts WHERE id=1`).Scan(&proxyID, &schedulable, &extra))
	require.EqualValues(t, 2, proxyID)
	require.True(t, schedulable)
	require.Contains(t, extra, "preserve")
	allFail.Store(true)
	_, err = m.Import(context.Background(), ImportRequest{URL: urls, Revision: v.Revision})
	require.Error(t, err)
	current, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Equal(t, v.Revision, current.Revision)
	byID := map[int64]Node{}
	for _, node := range current.Nodes {
		byID[node.ProxyID] = node
	}
	require.Equal(t, "keep-b", byID[2].Password)
	require.False(t, byID[3].Enabled, "legacy disabled nodes must not be re-enabled by a failed feed")
	var disabledStatus string
	require.NoError(t, db.QueryRow(`SELECT status FROM proxies WHERE id=3`).Scan(&disabledStatus))
	require.Equal(t, "inactive", disabledStatus)
	allFail.Store(false)
	newURLs := urls + "\n" + feed.URL + "/transient"
	preview, err = m.Preview(context.Background(), newURLs)
	require.NoError(t, err)
	selected := []string{}
	ids := map[string]string{}
	for _, node := range preview.Nodes {
		selected = append(selected, node.Name)
		ids[node.Name] = node.SourceID
	}
	transientFail.Store(true)
	v, err = m.Import(context.Background(), ImportRequest{URL: newURLs, Revision: current.Revision, Names: selected, SourceIDs: ids})
	require.NoError(t, err, "a newly failed feed must not abort unrelated successful imports")
	require.Len(t, v.Nodes, 2)
	require.EqualValues(t, 2, v.Nodes[1].ProxyID)
	require.Equal(t, "订阅2 · B", v.Nodes[1].Name)
}
