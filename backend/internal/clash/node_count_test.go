package clash

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func largeYAMLSubscription(count int) string {
	var body strings.Builder
	body.WriteString("proxies:\n")
	for i := range count {
		fmt.Fprintf(&body, " - {name: Node-%d, type: http, server: example.com, port: 80}\n", i)
	}
	return body.String()
}

func TestParseSubscriptionAbove512Nodes(t *testing.T) {
	const count = 1200
	var uris strings.Builder
	for i := range count {
		fmt.Fprintf(&uris, "trojan://test@example.com:443#Node-%d\n", i)
	}
	for name, body := range map[string]string{
		"yaml":   largeYAMLSubscription(count),
		"base64": base64.StdEncoding.EncodeToString([]byte(uris.String())),
	} {
		t.Run(name, func(t *testing.T) {
			nodes, err := parseSubscription([]byte(body))
			require.NoError(t, err)
			require.Len(t, nodes, count)
			require.Equal(t, "Node-1199", nodes[count-1].Name)
		})
	}
}

func TestMergeSubscriptionsAbove512Nodes(t *testing.T) {
	const perSource = 400
	body := largeYAMLSubscription(perSource)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer feed.Close()
	urls := []string{feed.URL + "/a", feed.URL + "/b", feed.URL + "/c"}
	nodes, sources, err := fetchSubscriptionDetails(context.Background(), feed.Client(), strings.Join(urls, "\n"))
	require.NoError(t, err)
	require.Len(t, sources, len(urls))
	require.Len(t, nodes, perSource*len(urls))
	for i, source := range sources {
		require.Empty(t, source.FetchError)
		require.Equal(t, perSource, source.NodeCount)
		for j := range perSource {
			node := nodes[i*perSource+j]
			require.Equal(t, source.ID, node.SourceID)
			require.Equal(t, fmt.Sprintf("Node-%d", j), node.OriginalName)
			require.Equal(t, fmt.Sprintf("订阅%d · Node-%d", i+1, j), node.Name)
		}
	}
}

func TestImportAbove512NodesPreservesFailedSources(t *testing.T) {
	db := sourceTestDB(t)
	_, err := db.Exec(`CREATE TABLE proxies(id BIGSERIAL PRIMARY KEY,name text,protocol text,host text,port int,username text,password text,status text,backup_proxy_id bigint,fallback_mode text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
CREATE TABLE accounts(id BIGSERIAL PRIMARY KEY,proxy_id bigint,proxy_fallback_origin_id bigint,extra jsonb,status text,schedulable bool,error_message text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
CREATE TABLE scheduler_outbox(event_type text,payload jsonb);`)
	require.NoError(t, err)
	const count = 520
	var allFail atomic.Bool
	body := largeYAMLSubscription(count)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allFail.Load() || r.URL.Path == "/bad" {
			http.Error(w, "offline", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, body)
	}))
	defer feed.Close()
	// Only TCP readiness and the submitted config are simulated; this does not
	// claim to test real Mihomo forwarding or production subscription providers.
	listeners := map[int]net.Listener{}
	var applies atomic.Int32
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Payload string }
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var config struct{ Listeners []struct{ Port int } }
		if json.Unmarshal([]byte(request.Payload), &config) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, entry := range config.Listeners {
			if listeners[entry.Port] != nil {
				continue
			}
			listener, e := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(entry.Port))
			if e != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			listeners[entry.Port] = listener
			go func() {
				for {
					conn, e := listener.Accept()
					if e != nil {
						return
					}
					conn.Close()
				}
			}()
		}
		applies.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer func() {
		controller.Close()
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	m := NewManager(db)
	m.controller, m.secret, m.host, m.portStart = controller.URL, "test-only", "127.0.0.1", 32500
	m.fetchClient = feed.Client()
	urls := feed.URL + "/good\n" + feed.URL + "/bad"
	previous := State{URL: urls, Revision: 1, Nodes: []Node{
		{Name: "订阅2 · Retained", ProxyID: 1, Port: 32500, Password: "keep-enabled", Enabled: true, Config: map[string]any{"name": "订阅2 · Retained", "type": "http", "server": "example.com", "port": 80}},
		{Name: "订阅2 · Disabled", ProxyID: 2, Port: 32501, Password: "keep-disabled", Enabled: false, Config: map[string]any{"name": "订阅2 · Disabled", "type": "http", "server": "example.com", "port": 80}},
	}}
	normalizeSources(&previous)
	raw, err := json.Marshal(previous)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO proxies(id,status) VALUES(1,'active'),(2,'inactive'); SELECT setval(pg_get_serial_sequence('proxies','id'),2);
INSERT INTO accounts(proxy_id,status,schedulable,extra) VALUES(1,'active',false,'{"manual":"preserve"}');`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	previous, err = loadState(context.Background(), db, false)
	require.NoError(t, err)
	preview, err := m.Preview(context.Background(), urls)
	require.NoError(t, err)
	require.Len(t, preview.Nodes, count+1)
	selected := make([]string, 0, count)
	for _, node := range preview.Nodes {
		if !node.Retained {
			selected = append(selected, node.Name)
		}
	}
	require.Len(t, selected, count)
	v, err := m.Import(context.Background(), ImportRequest{URL: urls, Names: selected, Revision: 1})
	require.NoError(t, err)
	require.Len(t, v.Nodes, count+1, "failed enabled node must survive even when not selected")
	current, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Len(t, current.Nodes, count+2)
	require.Equal(t, previous.Nodes[1], current.Nodes[0], "disabled failed-source node must remain disabled with its original credentials")
	ports := map[int]bool{}
	for _, node := range current.Nodes {
		require.False(t, ports[node.Port], "listener ports must remain unique")
		ports[node.Port] = true
		if node.ProxyID == 1 {
			require.Equal(t, "keep-enabled", node.Password)
			require.Equal(t, 32500, node.Port)
		}
	}
	var boundID int64
	var schedulable bool
	var extra string
	require.NoError(t, db.QueryRow(`SELECT proxy_id,schedulable,extra::text FROM accounts WHERE id=1`).Scan(&boundID, &schedulable, &extra))
	require.EqualValues(t, 1, boundID)
	require.False(t, schedulable)
	require.Contains(t, extra, "preserve")
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM proxies WHERE id=2`).Scan(&status))
	require.Equal(t, "inactive", status)
	// An all-failed refresh must remain a no-op for a pool larger than 512.
	allFail.Store(true)
	_, err = m.Import(context.Background(), ImportRequest{URL: urls, Revision: v.Revision})
	require.ErrorContains(t, err, "所有订阅均未成功获取")
	after, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Equal(t, current, after)
	require.EqualValues(t, 1, applies.Load())
}
