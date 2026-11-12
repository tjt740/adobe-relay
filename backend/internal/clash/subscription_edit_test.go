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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNotesIdentityValidationAndRuntimeMarker(t *testing.T) {
	old := State{URL: "https://a.example/sub\nhttps://b.example/sub", Revision: 3}
	before := marker(old)
	old.Notes, old.Priorities, old.SettingsRevision = updatedSettings(old, strings.Fields(old.URL), map[string]string{"https://a.example/sub": " 主力线路 "}, map[string]int{"https://a.example/sub": 100})
	require.Equal(t, before, marker(old), "notes must not trigger a reconcile reload")
	require.Equal(t, "主力线路", notesByURL(old)["https://a.example/sub"])
	notes, priorities, rev := updatedSettings(old, []string{"https://b.example/sub", "https://a.example/sub"}, nil, nil)
	require.Equal(t, old.Priorities, priorities)
	require.Equal(t, old.Notes, notes)
	require.Equal(t, old.SettingsRevision, rev)
	notes, priorities, rev = updatedSettings(old, strings.Fields(old.URL), map[string]string{"https://a.example/sub": ""}, map[string]int{"https://a.example/sub": 0})
	require.Empty(t, priorities)
	require.Empty(t, notes)
	require.Equal(t, old.SettingsRevision+1, rev)
	require.NoError(t, validateNotes(map[string]string{"url": strings.Repeat("中", 200)}))
	require.Error(t, validateNotes(map[string]string{"url": strings.Repeat("中", 201)}))
	require.Error(t, validateNotes(map[string]string{"url": "one\ntwo"}))
	require.Error(t, checkSettingsRevision(old, map[string]string{}, nil, nil))
}

func TestNotesSaveDoesNotFetchReloadOrChangeImportTime(t *testing.T) {
	db := sourceTestDB(t)
	previous := State{URL: "https://a.example/sub", Revision: 3, UpdatedAt: time.Now().UTC().Truncate(time.Second)}
	normalizeSources(&previous)
	raw, _ := json.Marshal(previous)
	_, err := db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	m := NewManager(db)
	// No runtime or subscription client is configured. Metadata saves still work.
	zero := int64(0)
	v, err := m.UpdateSettings(context.Background(), SettingsRequest{Revision: 3, SettingsRevision: &zero, Notes: map[string]string{previous.URL: "主力"}, Priorities: map[string]int{previous.URL: 100}})
	require.NoError(t, err)
	require.Equal(t, "主力", v.Notes[previous.URL])
	require.Equal(t, 100, v.Priorities[previous.URL])
	require.EqualValues(t, 1, v.SettingsRevision)
	current, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Equal(t, previous.Revision, current.Revision)
	require.Equal(t, previous.UpdatedAt, current.UpdatedAt)
	require.Equal(t, previous.Nodes, current.Nodes)
	require.Equal(t, previous.Sources, current.Sources)
	require.Equal(t, marker(previous), marker(current))
	_, err = m.UpdateSettings(context.Background(), SettingsRequest{Revision: 3, SettingsRevision: &zero, Notes: map[string]string{previous.URL: "stale"}})
	require.ErrorContains(t, err, "设置已被其他操作更新")
	one := int64(1)
	_, err = m.UpdateSettings(context.Background(), SettingsRequest{Revision: 2, SettingsRevision: &one, Notes: map[string]string{previous.URL: "stale"}})
	require.ErrorContains(t, err, "链接已变化")
	_, err = m.UpdateSettings(context.Background(), SettingsRequest{Revision: 3, SettingsRevision: &one, Notes: map[string]string{"https://unknown.example": "new"}})
	require.ErrorContains(t, err, "随导入")
	v, err = m.UpdateSettings(context.Background(), SettingsRequest{Revision: 3, SettingsRevision: &one, Notes: map[string]string{previous.URL: ""}})
	require.NoError(t, err)
	require.Empty(t, v.Notes[previous.URL])
	require.Equal(t, 100, v.Priorities[previous.URL], "omitted priorities must be preserved")
	require.Error(t, validatePriorities(map[string]int{previous.URL: -1}))
	require.Error(t, validatePriorities(map[string]int{previous.URL: 101}))
}

func TestAppendValidatesDeduplicatedCombinedURLLimitBeforeFetching(t *testing.T) {
	var fetched atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched.Add(1)
		fmt.Fprint(w, "proxies: [{name: New, type: http, server: example.com, port: 80}]")
	}))
	defer feed.Close()
	m := &Manager{fetchClient: feed.Client()}
	previous := State{URL: "https://old.example/sub?token=" + strings.Repeat("a", 40<<10)}
	saved, nodes, _, _, err := m.fetchNewSubscriptions(context.Background(), previous, previous.URL+"\n"+feed.URL)
	require.NoError(t, err)
	require.Equal(t, previous.URL+"\n"+feed.URL, saved)
	require.Len(t, nodes, 1)
	require.EqualValues(t, 1, fetched.Load())
	_, _, _, _, err = m.fetchNewSubscriptions(context.Background(), previous, feed.URL+"?token="+strings.Repeat("b", 30<<10))
	require.ErrorContains(t, err, "64 KiB")
	require.EqualValues(t, 1, fetched.Load(), "combined size must be checked before any fetch")
}

func TestAppendImportsOnlyNewFeedsAndPreservesExistingRecords(t *testing.T) {
	db := sourceTestDB(t)
	_, err := db.Exec(`CREATE TABLE proxies(id BIGSERIAL PRIMARY KEY,name text,protocol text,host text,port int,username text,password text,status text,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
 CREATE TABLE adobe_route_exclusions(proxy_id bigint PRIMARY KEY);
 CREATE TABLE accounts(id BIGSERIAL PRIMARY KEY,proxy_id bigint,extra jsonb,status text,schedulable bool,updated_at timestamptz DEFAULT NOW(),deleted_at timestamptz);
 CREATE TABLE scheduler_outbox(event_type text,payload jsonb);`)
	require.NoError(t, err)
	var oldFetches, newFetches, badFetches, applies atomic.Int32
	var recoverBad, failRuntime atomic.Bool
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/old":
			oldFetches.Add(1)
			http.Error(w, "old must not be fetched", 503)
			return
		case "/bad":
			badFetches.Add(1)
			if !recoverBad.Load() {
				http.Error(w, "offline", 502)
				return
			}
		default:
			newFetches.Add(1)
		}
		fmt.Fprint(w, "proxies: [{name: Shared, type: http, server: example.com, port: 80}]")
	}))
	defer feed.Close()
	listeners := map[int]net.Listener{}
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applies.Add(1)
		if failRuntime.Load() {
			w.WriteHeader(500)
			return
		}
		var req struct{ Payload string }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		var config struct{ Listeners []struct{ Port int } }
		require.NoError(t, json.Unmarshal([]byte(req.Payload), &config))
		for _, entry := range config.Listeners {
			if listeners[entry.Port] != nil {
				continue
			}
			listener, e := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(entry.Port))
			if e != nil {
				t.Errorf("listen: %v", e)
				w.WriteHeader(500)
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
		w.WriteHeader(204)
	}))
	defer func() {
		controller.Close()
		for _, l := range listeners {
			l.Close()
		}
	}()
	m := NewManager(db)
	m.controller, m.secret, m.host, m.portStart = controller.URL, "test-only", "127.0.0.1", 28560
	m.fetchClient = feed.Client()
	oldURL, newURL, badURL := feed.URL+"/old", feed.URL+"/new", feed.URL+"/bad"
	previous := State{URL: oldURL, Revision: 8, SettingsRevision: 2, Notes: map[string]string{sourceID(oldURL): "原订阅"}, Nodes: []Node{
		{Name: "订阅2 · Shared", OriginalName: "订阅2 · Shared", SourceID: sourceID(oldURL), ProxyID: 1, Port: 28560, Password: "keep-secret", Enabled: true, Config: map[string]any{"name": "订阅2 · Shared", "type": "http", "server": "old.example", "port": 80}},
		{Name: "Disabled", OriginalName: "Disabled", SourceID: sourceID(oldURL), ProxyID: 2, Port: 28561, Password: "disabled-secret", Enabled: false, Config: map[string]any{"name": "Disabled", "type": "http", "server": "old.example", "port": 80}},
	}}
	normalizeSources(&previous)
	previous.Sources[0].FetchError = "http_error"
	previous.Sources[0].Failures = 7
	previous.Sources[0].Traffic = &SourceTraffic{Total: 10, Download: 10}
	raw, _ := json.Marshal(previous)
	_, err = db.Exec(`INSERT INTO proxies(id,name,status) VALUES(1,'old','inactive'),(2,'disabled','inactive'); SELECT setval(pg_get_serial_sequence('proxies','id'),2);
 INSERT INTO adobe_route_exclusions(proxy_id) VALUES(1);
 INSERT INTO accounts(proxy_id,extra,status,schedulable) VALUES(1,'{"upstream_billing_probe":{"keep":true}}','active',false);`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	previous, err = loadState(context.Background(), db, false)
	require.NoError(t, err)
	snapshotRows := func() string {
		var result string
		require.NoError(t, db.QueryRow(`SELECT jsonb_build_object('proxies',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM proxies p WHERE id<=2),'accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM accounts a),'events',(SELECT jsonb_agg(to_jsonb(e)) FROM scheduler_outbox e))::text`).Scan(&result))
		return result
	}
	before := snapshotRows()
	rev := int64(2)
	v, err := m.Import(context.Background(), ImportRequest{Mode: "append", URL: oldURL + "\n" + newURL + "\n" + badURL + "\n" + newURL, Revision: 8, Notes: map[string]string{newURL: "新增", badURL: "待重试"}, Priorities: map[string]int{newURL: 50, badURL: 100}, SettingsRevision: &rev})
	require.NoError(t, err)
	require.Zero(t, oldFetches.Load())
	require.EqualValues(t, 1, newFetches.Load())
	require.EqualValues(t, 1, badFetches.Load())
	require.Equal(t, oldURL+"\n"+newURL, v.URL)
	require.Len(t, v.SkippedSources, 1)
	require.Equal(t, "新增", v.Notes[newURL])
	require.Equal(t, 50, v.Priorities[newURL])
	require.NotContains(t, v.Priorities, badURL)
	require.NotContains(t, v.Notes, badURL)
	current, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Equal(t, previous.Nodes, current.Nodes[:2], "original nodes, credentials, ports and disabled state must be untouched")
	require.Equal(t, previous.Sources[0], current.Sources[0], "append cannot refresh failure or quota evidence for old sources")
	require.NotEqual(t, current.Nodes[0].Name, current.Nodes[2].Name, "new source must not collide with an old name")
	require.Equal(t, before, snapshotRows(), "no writes to old proxies, bound account extras, manual pause or outbox")
	require.EqualValues(t, 1, applies.Load())
	_, err = m.Import(context.Background(), ImportRequest{Mode: "append", URL: newURL, Revision: v.Revision})
	require.ErrorContains(t, err, "没有新增")
	require.EqualValues(t, 1, newFetches.Load())
	_, err = m.Import(context.Background(), ImportRequest{Mode: "append", URL: badURL, Revision: 8})
	require.ErrorContains(t, err, "重新预览")
	require.EqualValues(t, 1, badFetches.Load())
	_, err = m.Import(context.Background(), ImportRequest{Mode: "append", URL: badURL, Revision: v.Revision})
	require.ErrorContains(t, err, "新增订阅均未成功")
	unchanged, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Equal(t, current, unchanged)
	recoverBad.Store(true)
	v, err = m.Import(context.Background(), ImportRequest{Mode: "append", URL: badURL, Revision: v.Revision})
	require.NoError(t, err)
	require.Contains(t, v.URL, badURL)
	require.Zero(t, oldFetches.Load())
	require.Equal(t, before, snapshotRows())
	// A runtime failure rolls back both the added proxy and document.
	current, err = loadState(context.Background(), db, false)
	require.NoError(t, err)
	failRuntime.Store(true)
	_, err = m.Import(context.Background(), ImportRequest{Mode: "append", URL: feed.URL + "/fail-runtime", Revision: v.Revision})
	require.Error(t, err)
	unchanged, err = loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.Equal(t, current, unchanged)
	require.Equal(t, before, snapshotRows())
}
