package clash

// Opt-in: CLASH_TEST_DATABASE_URL must point at a disposable PostgreSQL server.
// CLASH_TEST_CONTROLLER_URL / CLASH_TEST_SECRET configure a disposable Mihomo.
// Publish its ports 24000-24020 unchanged on localhost. Tests create an isolated SQL
// schema and replace the complete Mihomo config. Optional CLASH_TEST_SUBSCRIPTION_FILE
// verifies a private real-world YAML fixture without checking it into the repository.
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestMihomoLifecycleIntegration(t *testing.T) {
	dsn := os.Getenv("CLASH_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CLASH_TEST_DATABASE_URL for isolated PostgreSQL + Mihomo integration")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer root.Close()
	schema := fmt.Sprintf("clash_test_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	defer root.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE proxies(id BIGSERIAL PRIMARY KEY,name varchar(100),protocol text,host text,port int,username text,password text,status text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
 CREATE TABLE accounts(id BIGSERIAL PRIMARY KEY,proxy_id bigint,extra jsonb,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
 CREATE TABLE scheduler_outbox(id BIGSERIAL PRIMARY KEY,event_type text,payload jsonb);`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../migrations/240_clash_subscription.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	// Two real outbound HTTP proxies return distinct markers, proving per-node routing.
	upstream := func(label string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, label) }))
	}
	a, b := upstream("exit-a"), upstream("exit-b")
	defer a.Close()
	defer b.Close()
	portOf := func(raw string) int { u, _ := url.Parse(raw); p, _ := strconv.Atoi(u.Port()); return p }
	yamlBody := func(aPort int) string {
		return fmt.Sprintf("proxies:\n - {name: A, type: http, server: host.docker.internal, port: %d}\n - {name: B, type: http, server: host.docker.internal, port: %d}\n", aPort, portOf(b.URL))
	}
	var mu sync.Mutex
	subscription := yamlBody(portOf(a.URL))
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprint(w, subscription)
	}))
	defer feed.Close()
	m := NewManager(db)
	m.controller = os.Getenv("CLASH_TEST_CONTROLLER_URL")
	m.secret = os.Getenv("CLASH_TEST_SECRET")
	m.host = "127.0.0.1"
	m.portStart = 24000
	m.fetchClient = feed.Client()
	ctx := context.Background()
	preview, err := m.Preview(ctx, feed.URL)
	require.NoError(t, err)
	require.Len(t, preview.Nodes, 2)
	v, err := m.Import(ctx, ImportRequest{URL: feed.URL, Names: []string{"A", "B"}, Revision: 0})
	require.NoError(t, err)
	require.True(t, v.Ready)
	original, err := loadState(ctx, db, false)
	require.NoError(t, err)
	id := original.Nodes[0].ProxyID
	managed, err := m.IsManaged(ctx, id)
	require.NoError(t, err)
	require.True(t, managed)
	managed, err = m.IsManaged(ctx, 999999)
	require.NoError(t, err)
	require.False(t, managed)
	_, err = db.Exec(`INSERT INTO accounts(proxy_id,extra) VALUES($1,'{"upstream_billing_probe":{"stale":true}}')`, id)
	require.NoError(t, err)
	requestThrough := func(n Node) (string, error) {
		p := &url.URL{Scheme: "http", Host: net.JoinHostPort(m.host, strconv.Itoa(n.Port)), User: url.UserPassword("clash", n.Password)}
		tr := &http.Transport{Proxy: http.ProxyURL(p)}
		defer tr.CloseIdleConnections()
		client := &http.Client{Transport: tr, Timeout: 3 * time.Second}
		res, err := client.Get("http://example.invalid/clash-test")
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		return string(body), err
	}
	body, err := requestThrough(original.Nodes[0])
	require.NoError(t, err)
	require.Equal(t, "exit-a", body)
	body, err = requestThrough(original.Nodes[1])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	wrongPassword := original.Nodes[0]
	wrongPassword.Password = "wrong"
	body, err = requestThrough(wrongPassword)
	require.True(t, err != nil || !strings.Contains(body, "exit-"), "node entry must require authentication")
	// Replace the subscription URL and A's destination, preserving its ID and account binding.
	mu.Lock()
	subscription = yamlBody(portOf(b.URL))
	mu.Unlock()
	v, err = m.Import(ctx, ImportRequest{URL: feed.URL + "/replacement", Names: []string{"A"}, Revision: 1})
	require.NoError(t, err)
	require.Equal(t, id, v.Nodes[0].ProxyID)
	current, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, original.Nodes[0].Port, current.Nodes[0].Port)
	body, err = requestThrough(current.Nodes[0])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	body, err = requestThrough(original.Nodes[1])
	require.True(t, err != nil || !strings.Contains(body, "exit-"), "retired node must fail closed")
	var extra []byte
	var accountProxy int64
	require.NoError(t, db.QueryRow("SELECT proxy_id,extra FROM accounts").Scan(&accountProxy, &extra))
	require.Equal(t, id, accountProxy)
	require.NotContains(t, string(extra), "stale")
	var events int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM scheduler_outbox").Scan(&events))
	require.Greater(t, events, 0)
	_, err = m.Import(ctx, ImportRequest{Names: []string{"A"}, Revision: 1})
	require.ErrorContains(t, err, "重新预览")
	// An invalid inner-core configuration must leave the old database and route intact.
	mu.Lock()
	subscription = "proxies: [{name: A, type: ss, server: example.com, port: 443, cipher: invalid-cipher, password: test}]"
	mu.Unlock()
	_, err = m.Import(ctx, ImportRequest{Names: []string{"A"}, Revision: 2})
	require.Error(t, err)
	unchanged, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), unchanged.Revision)
	body, err = requestThrough(current.Nodes[0])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	// A core restart loses its in-memory document. A fresh application manager restores it.
	require.NoError(t, m.apply(ctx, State{}))
	require.False(t, m.matches(ctx, current))
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { m.Run(runCtx); close(done) }()
	require.Eventually(t, func() bool { return m.matches(ctx, current) }, 5*time.Second, 100*time.Millisecond)
	stop()
	<-done
	body, err = requestThrough(current.Nodes[0])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	// A private supplied fixture can be parsed and validated by the real core without disclosure.
	if path := os.Getenv("CLASH_TEST_SUBSCRIPTION_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		nodes, err := parseSubscription(raw)
		require.NoError(t, err)
		t.Logf("real subscription parsed: %d nodes", len(nodes))
		sample := State{Revision: 99, Nodes: []Node{nodes[0]}}
		sample.Nodes[0].ProxyID = 1000
		sample.Nodes[0].Port = 24010
		sample.Nodes[0].Password = "fixture-secret"
		sample.Nodes[0].Enabled = true
		require.NoError(t, m.apply(ctx, sample))
		require.True(t, m.matches(ctx, sample))
		// The caller can opt into a small public liveness check through the supplied node.
		if os.Getenv("CLASH_TEST_LIVE") == "1" {
			p := &url.URL{Scheme: "http", Host: "127.0.0.1:24010", User: url.UserPassword("clash", "fixture-secret")}
			tr := &http.Transport{Proxy: http.ProxyURL(p)}
			defer tr.CloseIdleConnections()
			client := &http.Client{Transport: tr, Timeout: 20 * time.Second}
			res, e := client.Get("https://www.gstatic.com/generate_204")
			require.NoError(t, e)
			res.Body.Close()
			require.Equal(t, 204, res.StatusCode)
			t.Log("real subscription node HTTPS liveness: HTTP 204")
		}
		require.NoError(t, m.apply(ctx, current))
	}
	// Disabling every node keeps IDs but cannot fall through to DIRECT.
	mu.Lock()
	subscription = yamlBody(portOf(a.URL))
	mu.Unlock()
	v, err = m.Import(ctx, ImportRequest{Names: []string{}, Revision: 2})
	require.NoError(t, err)
	for _, n := range v.Nodes {
		require.False(t, n.Enabled)
	}
	// Concurrent imports must not allocate duplicates or clobber one another.
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := m.Import(ctx, ImportRequest{Names: []string{"A", "B"}, Revision: 3})
			outcomes <- e
		}()
	}
	wg.Wait()
	close(outcomes)
	successes, conflicts := 0, 0
	for e := range outcomes {
		if e == nil {
			successes++
		} else if strings.Contains(e.Error(), "重新预览") {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	restored, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, original.Nodes[1].ProxyID, restored.Nodes[1].ProxyID)
	body, err = requestThrough(restored.Nodes[1])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	var total int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM proxies").Scan(&total))
	require.Equal(t, 2, total)
	viewJSON, err := json.Marshal(v)
	require.NoError(t, err)
	require.NotContains(t, string(viewJSON), "password")
}
