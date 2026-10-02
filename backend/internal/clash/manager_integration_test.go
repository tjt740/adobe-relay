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
 CREATE TABLE accounts(id BIGSERIAL PRIMARY KEY,proxy_id bigint REFERENCES proxies(id) ON DELETE SET NULL,extra jsonb,status text DEFAULT 'active',schedulable boolean DEFAULT true,error_message text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
 CREATE TABLE scheduler_outbox(id BIGSERIAL PRIMARY KEY,event_type text,payload jsonb);`)
	require.NoError(t, err)
	for _, name := range []string{"149_proxy_expiry_fallback.sql", "240_clash_subscription.sql", "241_account_proxy_failover.sql"} {
		migration, err := os.ReadFile("../../migrations/" + name)
		require.NoError(t, err)
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}
	// Even manual proxies that resemble a managed endpoint must survive cleanup.
	var manualID int64
	require.NoError(t, db.QueryRow(`INSERT INTO proxies(name,protocol,host,port,username,password,status) VALUES('Clash · Manual','http','mihomo',24900,'clash','manual-secret','active') RETURNING id`).Scan(&manualID))
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
	// Each isolated schema starts its ID sequence from scratch. Clear the
	// disposable runtime so prior runs cannot own the same ports under other IDs.
	require.NoError(t, m.apply(ctx, State{}))
	t.Cleanup(func() { require.NoError(t, m.apply(context.Background(), State{})) })
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
	_, err = db.Exec(`INSERT INTO accounts(proxy_id,extra) VALUES($1,'{"upstream_billing_probe":{"stale":true}}'),($2,'{"keep":true}'),($3,'{"manual":true}')`, id, original.Nodes[1].ProxyID, manualID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_proxy_failover(account_id,enabled,primary_proxy_id,backup_proxy_ids) VALUES(1,true,$1,jsonb_build_array($2::bigint,$3::bigint)),(2,true,$2,'[]'),(3,true,$3,jsonb_build_array($2::bigint,$1::bigint))`, id, original.Nodes[1].ProxyID, manualID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE proxies SET backup_proxy_id=$1,fallback_mode='proxy' WHERE id=$2`, original.Nodes[1].ProxyID, manualID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET proxy_fallback_origin_id=$1 WHERE id=3`, original.Nodes[1].ProxyID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO proxy_failover_health(proxy_id,fingerprint,status,checked_at) VALUES($1,'old','healthy',NOW())`, original.Nodes[1].ProxyID)
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
	require.Len(t, v.Nodes, 1)
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
	require.NoError(t, db.QueryRow("SELECT proxy_id,extra FROM accounts WHERE id=1").Scan(&accountProxy, &extra))
	require.Equal(t, id, accountProxy)
	require.NotContains(t, string(extra), "stale")
	var total int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM proxies").Scan(&total))
	require.Equal(t, 2, total, "only current A and the manual proxy remain")
	var paused bool
	require.NoError(t, db.QueryRow(`SELECT proxy_id IS NULL AND NOT schedulable AND status='error' AND error_message LIKE '%Clash%' FROM accounts WHERE id=2`).Scan(&paused))
	require.True(t, paused, "removing a bound proxy must pause its account, never route direct")
	var manualOK bool
	require.NoError(t, db.QueryRow(`SELECT a.proxy_id=$1 AND a.schedulable AND a.status='active' AND a.proxy_fallback_origin_id IS NULL AND p.backup_proxy_id IS NULL AND p.fallback_mode='none' AND p.password='manual-secret' FROM accounts a JOIN proxies p ON p.id=a.proxy_id WHERE a.id=3`, manualID).Scan(&manualOK))
	require.True(t, manualOK)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM account_proxy_failover WHERE primary_proxy_id=$1 OR backup_proxy_ids @> jsonb_build_array($1::bigint)`, original.Nodes[1].ProxyID).Scan(&total))
	require.Zero(t, total)
	var backups []byte
	require.NoError(t, db.QueryRow(`SELECT backup_proxy_ids FROM account_proxy_failover WHERE account_id=3`).Scan(&backups))
	require.JSONEq(t, fmt.Sprintf("[%d]", id), string(backups))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM proxy_failover_health`).Scan(&total))
	require.Zero(t, total)
	var events int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM scheduler_outbox").Scan(&events))
	require.Greater(t, events, 0)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM scheduler_outbox WHERE payload->'account_ids' @> '[2]'::jsonb`).Scan(&events))
	require.Positive(t, events, "retired accounts must be evicted from scheduler caches")
	_, err = m.Import(ctx, ImportRequest{Names: []string{"A"}, Revision: 1})
	require.ErrorContains(t, err, "重新预览")
	// An invalid inner-core configuration must leave the old database and route intact.
	mu.Lock()
	subscription = "proxies: [{name: B, type: ss, server: example.com, port: 443, cipher: invalid-cipher, password: test}]"
	mu.Unlock()
	_, err = m.Import(ctx, ImportRequest{Names: []string{"B"}, Revision: 2})
	require.Error(t, err)
	unchanged, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), unchanged.Revision)
	require.NoError(t, db.QueryRow(`SELECT proxy_id=$1 AND schedulable AND status='active' FROM accounts WHERE id=1`, id).Scan(&manualOK))
	require.True(t, manualOK, "failed apply rolls back deletion and account changes")
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM proxies WHERE id=$1`, id).Scan(&total))
	require.Equal(t, 1, total)
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
	// Removing every node leaves only manual proxies and no subscription records.
	mu.Lock()
	subscription = yamlBody(portOf(a.URL))
	mu.Unlock()
	v, err = m.Import(ctx, ImportRequest{Names: []string{}, Revision: 2})
	require.NoError(t, err)
	require.Empty(t, v.Nodes)
	empty, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Empty(t, empty.Nodes)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM proxies").Scan(&total))
	require.Equal(t, 1, total)
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
	require.NotEqual(t, original.Nodes[1].ProxyID, restored.Nodes[1].ProxyID)
	require.Equal(t, original.Nodes[1].Port, restored.Nodes[1].Port, "retired ports can be reused")
	require.NotEqual(t, original.Nodes[1].Password, restored.Nodes[1].Password)
	body, err = requestThrough(restored.Nodes[1])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM proxies").Scan(&total))
	require.Equal(t, 3, total)
	body, err = requestThrough(original.Nodes[1])
	require.True(t, err != nil || !strings.Contains(body, "exit-"), "old credentials cannot use a recycled port")
	// Old releases retained disabled nodes forever. Reconciliation prunes these
	// without fetching the subscription and without changing active/manual nodes.
	var legacyID int64
	require.NoError(t, db.QueryRow(`INSERT INTO proxies(name,protocol,host,port,username,password,status) VALUES('Clash · Retired','http','mihomo',24010,'clash','legacy-secret','inactive') RETURNING id`).Scan(&legacyID))
	legacy := restored
	legacy.Nodes = append(legacy.Nodes, Node{Name: "Retired", ProxyID: legacyID, Port: 24010, Password: "legacy-secret", Enabled: false})
	legacyJSON, err := json.Marshal(legacy)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(legacyJSON))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO accounts(proxy_id) VALUES($1)`, legacyID)
	require.NoError(t, err)
	require.NoError(t, m.apply(ctx, legacy))
	require.NoError(t, m.reconcile(ctx))
	cleaned, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, restored.Nodes, cleaned.Nodes)
	require.Equal(t, restored.Revision+1, cleaned.Revision)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM proxies").Scan(&total))
	require.Equal(t, 3, total)
	require.NoError(t, db.QueryRow(`SELECT proxy_id IS NULL AND NOT schedulable AND status='error' FROM accounts WHERE id=4`).Scan(&paused))
	require.True(t, paused)
	require.NoError(t, m.reconcile(ctx))
	idempotent, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, cleaned.Revision, idempotent.Revision)
	// Subscription order must not let a new node steal a retained node's port.
	mu.Lock()
	subscription = fmt.Sprintf("proxies:\n - {name: C, type: http, server: host.docker.internal, port: %d}\n - {name: A, type: http, server: host.docker.internal, port: %d}\n - {name: B, type: http, server: host.docker.internal, port: %d}\n", portOf(a.URL), portOf(a.URL), portOf(b.URL))
	mu.Unlock()
	_, err = m.Import(ctx, ImportRequest{Names: []string{"C", "A", "B"}, Revision: cleaned.Revision})
	require.NoError(t, err)
	expanded, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Equal(t, restored.Nodes[0].Port, expanded.Nodes[1].Port)
	require.Equal(t, restored.Nodes[1].Port, expanded.Nodes[2].Port)
	require.NotEqual(t, expanded.Nodes[0].Port, expanded.Nodes[1].Port)
	require.NotEqual(t, expanded.Nodes[0].Port, expanded.Nodes[2].Port)
	// Replacing every node in a single reload must not collide with listeners
	// Mihomo has not closed yet. The next import may reuse those released ports.
	mu.Lock()
	subscription = fmt.Sprintf("proxies: [{name: D, type: http, server: host.docker.internal, port: %d}]", portOf(b.URL))
	mu.Unlock()
	_, err = m.Import(ctx, ImportRequest{Names: []string{"D"}, Revision: expanded.Revision})
	require.NoError(t, err)
	replaced, err := loadState(ctx, db, false)
	require.NoError(t, err)
	require.Len(t, replaced.Nodes, 1)
	for _, n := range expanded.Nodes {
		require.NotEqual(t, n.Port, replaced.Nodes[0].Port)
	}
	body, err = requestThrough(replaced.Nodes[0])
	require.NoError(t, err)
	require.Equal(t, "exit-b", body)
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM proxies").Scan(&total))
	require.Equal(t, 2, total, "replacement keeps only D and the manual proxy")
	viewJSON, err := json.Marshal(v)
	require.NoError(t, err)
	require.NotContains(t, string(viewJSON), "password")
}
