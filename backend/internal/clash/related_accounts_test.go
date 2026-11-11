package clash

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRelatedAccountsCurrentImpactAndRecordedDepartures(t *testing.T) {
	db := sourceTestDB(t)
	_, err := db.Exec(`CREATE TABLE proxies(id bigint PRIMARY KEY,name text,deleted_at timestamptz,status text,expires_at timestamptz,updated_at timestamptz);
 CREATE TABLE accounts(id bigint PRIMARY KEY,name text,platform text DEFAULT 'adobe',proxy_id bigint,status text DEFAULT 'active',
 schedulable bool DEFAULT true,proxy_auto_paused bool DEFAULT false,extra jsonb DEFAULT '{}',deleted_at timestamptz,
 expires_at timestamptz,auto_pause_on_expired bool DEFAULT true,temp_unschedulable_until timestamptz,overload_until timestamptz,rate_limit_reset_at timestamptz,
 credentials jsonb DEFAULT '{"token":"private-account-token"}',notes text DEFAULT 'private note');
 CREATE TABLE proxy_auto_allocation(id int,enabled bool);
 CREATE TABLE account_proxy_failover(account_id bigint,enabled bool);
 CREATE TABLE proxy_failover_events(id bigserial PRIMARY KEY,account_id bigint,from_proxy_id bigint,to_proxy_id bigint,created_at timestamptz);
 CREATE TABLE adobe_route_exclusions(proxy_id bigint);
 CREATE TABLE adobe_route_health(proxy_id bigint,proxy_updated_at timestamptz,retry_after timestamptz);
 CREATE TABLE proxy_failover_health(proxy_id bigint,status text,failures int,checked_at timestamptz);
 INSERT INTO proxy_auto_allocation VALUES(1,true);
 INSERT INTO proxies SELECT i,'Node '||i,NULL,'active',NULL,NOW()-INTERVAL '1 hour' FROM generate_series(1,4) AS i;
 UPDATE proxies SET deleted_at=NOW() WHERE id=4;
 INSERT INTO proxy_failover_health SELECT id,'healthy',0,NOW() FROM proxies;
 UPDATE proxy_failover_health SET status='unhealthy',failures=2 WHERE proxy_id=3;
 INSERT INTO adobe_route_health SELECT id,updated_at,NOW()+INTERVAL '5 minutes' FROM proxies WHERE id=1;
 INSERT INTO accounts(id,name,proxy_id) VALUES(1,'Auto account',1),(2,'Fixed account',1),(3,'Account error',1),
 (4,'Moved twice',3),(5,'Waiting account',NULL),(6,'Returned account',1),(7,'Other platform',1),
 (8,'Old history',2),(9,'Deleted account',2),(10,'Direct account',NULL),(11,'Moved account',2),(12,'Unrelated account',2),
 (13,'Deleted destination',4),(14,'Future record',2);
 UPDATE accounts SET extra='{"manual_proxy_enabled":true}' WHERE id IN(2,10);
 UPDATE accounts SET status='error',schedulable=false WHERE id=3;
 UPDATE accounts SET status='error',schedulable=false,proxy_auto_paused=true WHERE id=5;
 UPDATE accounts SET platform='openai' WHERE id=7;
 UPDATE accounts SET deleted_at=NOW() WHERE id=9;
 INSERT INTO proxy_failover_events(account_id,from_proxy_id,to_proxy_id,created_at)
 VALUES(4,1,2,NOW()-INTERVAL '20 minutes'),(4,2,3,NOW()-INTERVAL '10 minutes'),
 (5,1,0,NOW()-INTERVAL '20 minutes'),(6,1,2,NOW()-INTERVAL '20 minutes'),(6,2,1,NOW()-INTERVAL '10 minutes'),
 (8,1,2,NOW()-INTERVAL '25 hours'),(9,1,2,NOW()-INTERVAL '20 minutes'),(10,1,0,NOW()-INTERVAL '20 minutes'),
 (11,1,3,NOW()-INTERVAL '30 minutes'),(11,1,2,NOW()-INTERVAL '20 minutes'),
 (13,1,4,NOW()-INTERVAL '20 minutes'),(14,1,2,NOW()+INTERVAL '1 hour');
 `)
	require.NoError(t, err)
	state := State{URL: "https://provider.example/private?token=secret", Nodes: []Node{{ProxyID: 2, Enabled: true}}}
	normalizeSources(&state)
	state.Sources[0].Traffic = &SourceTraffic{Total: 100, Download: 100}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`SET default_transaction_read_only=on`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(`SET default_transaction_read_only=off`) })
	m := NewManager(db)
	query := RelatedAccountsQuery{Page: 1, PageSize: 50, Relation: "all"}
	view, err := m.RelatedAccounts(context.Background(), 1, query)
	require.NoError(t, err)
	require.Equal(t, 5, view.CurrentCount)
	require.Equal(t, 5, view.DepartedCount)
	require.Equal(t, 10, view.Total)
	require.Len(t, view.Items, 10)
	items := map[int64]ProxyRelatedAccount{}
	for _, item := range view.Items {
		items[item.ID] = item
	}
	require.Equal(t, "automatic", items[1].Mode)
	require.Equal(t, "affected", items[1].Impact)
	require.Equal(t, "generation_paused", items[1].RouteState)
	require.Equal(t, "manual", items[2].Mode)
	require.Equal(t, "error", items[3].AccountState, "unschedulable errors must not be mislabeled as manual pauses")
	require.Equal(t, "left_node", items[4].Impact)
	require.EqualValues(t, 3, items[4].CurrentProxy.ID)
	require.EqualValues(t, 2, items[4].LastDeparture.Destination.ID, "historical destination is not the current node")
	require.Equal(t, "unavailable", items[4].RouteState, "moving does not prove recovery")
	require.Equal(t, "waiting_proxy", items[5].AccountState)
	require.Equal(t, "waiting_proxy", items[5].RouteState)
	require.Nil(t, items[5].CurrentProxy)
	require.Nil(t, items[5].LastDeparture.Destination)
	require.True(t, items[6].UsingThisNode)
	require.NotNil(t, items[6].LastDeparture, "a returned account retains its departure evidence")
	require.Equal(t, "affected", items[6].Impact)
	require.Equal(t, "not_blocked", items[7].Impact, "Adobe cooldown must not mark other platforms as blocked")
	require.Equal(t, "manual_direct", items[10].Mode)
	require.Equal(t, "direct", items[10].RouteState)
	require.Equal(t, "subscription_blocked", items[11].RouteState)
	require.EqualValues(t, 2, items[11].LastDeparture.Destination.ID, "keep only the latest recorded departure")
	require.Equal(t, "unavailable", items[13].RouteState)
	for _, id := range []int64{8, 9, 12, 14} {
		require.NotContains(t, items, id)
	}
	encoded, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-account-token")
	require.NotContains(t, string(encoded), "private note")
	require.NotContains(t, string(encoded), "credentials")
	require.NotContains(t, string(encoded), "token=secret")

	query.Relation = "departed"
	query.PageSize = 2
	query.Page = 3
	view, err = m.RelatedAccounts(context.Background(), 1, query)
	require.NoError(t, err)
	require.Equal(t, 5, view.Total)
	require.Len(t, view.Items, 1)
	require.EqualValues(t, 13, view.Items[0].ID)
	query = RelatedAccountsQuery{Page: 1, PageSize: 20, Relation: "current", Search: "fixed"}
	view, err = m.RelatedAccounts(context.Background(), 1, query)
	require.NoError(t, err)
	require.Equal(t, 1, view.Total)
	require.EqualValues(t, 2, view.Items[0].ID)
	require.Equal(t, 5, view.CurrentCount, "headline counts describe all associations, not only search matches")
	query.Search = "%' OR 1=1"
	view, err = m.RelatedAccounts(context.Background(), 1, query)
	require.NoError(t, err)
	require.Empty(t, view.Items)
	_, err = m.RelatedAccounts(context.Background(), 4, query)
	require.ErrorIs(t, err, ErrRelatedProxyNotFound)
	_, err = m.RelatedAccounts(context.Background(), 1, RelatedAccountsQuery{Page: -1, PageSize: 20, Relation: "all"})
	require.ErrorIs(t, err, ErrRelatedAccountsQuery)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM proxy_failover_events`).Scan(&count))
	require.Equal(t, 12, count, "reading details never writes switch events")
}

func TestRelatedAccountEvidenceModesAndGates(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		e                          relatedAccountEvidence
		proxy                      bool
		mode, state, route, impact string
	}{
		{"auto off", relatedAccountEvidence{status: "active", schedulable: true, proxyActive: true, probeFresh: true, probeStatus: "healthy"}, true, "unmanaged", "eligible", "reachable", "not_blocked"},
		{"legacy failover", relatedAccountEvidence{failover: true, status: "active", schedulable: true, proxyActive: true}, true, "failover", "eligible", "unconfirmed", "unconfirmed"},
		{"manual pause", relatedAccountEvidence{manual: true, manuallyPaused: true, status: "active", schedulable: true, proxyActive: true, excluded: true}, true, "manual", "paused", "generation_paused", "affected"},
		{"expired account", relatedAccountEvidence{automatic: true, status: "active", schedulable: true, expired: true, proxyActive: true, probeFresh: true, probeStatus: "healthy"}, true, "automatic", "expired", "reachable", "not_blocked"},
		{"disabled", relatedAccountEvidence{automatic: true, status: "disabled", schedulable: false}, true, "automatic", "disabled", "unavailable", "affected"},
		{"account cooldown", relatedAccountEvidence{automatic: true, status: "active", schedulable: true, accountCooldown: true, proxyActive: true, probeFresh: true, probeStatus: "healthy"}, true, "automatic", "cooldown", "reachable", "not_blocked"},
		{"direct", relatedAccountEvidence{status: "active", schedulable: true}, false, "unmanaged", "eligible", "direct", "left_node"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := ProxyRelatedAccount{Platform: "adobe", UsingThisNode: tc.proxy}
			if tc.proxy {
				item.CurrentProxy = &RelatedProxyRef{ID: 1}
			}
			describeRelatedAccount(&item, tc.e)
			require.Equal(t, tc.mode, item.Mode)
			require.Equal(t, tc.state, item.AccountState)
			require.Equal(t, tc.route, item.RouteState)
			require.Equal(t, tc.impact, item.Impact)
		})
	}
}
