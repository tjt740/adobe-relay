package clash

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthDetailsMatchActiveCooldownAndPreserveManualExclusions(t *testing.T) {
	db := sourceTestDB(t)
	_, err := db.Exec(`CREATE TABLE proxies(id bigint PRIMARY KEY,deleted_at timestamptz,status text,expires_at timestamptz,updated_at timestamptz);
 CREATE TABLE adobe_route_exclusions(proxy_id bigint PRIMARY KEY,proxy_updated_at timestamptz,created_at timestamptz DEFAULT NOW(),reason text);
 CREATE TABLE adobe_route_health(proxy_id bigint,proxy_updated_at timestamptz,retry_after timestamptz,failed_at timestamptz,scope text);
 CREATE TABLE proxy_failover_health(proxy_id bigint,status text,failures int,checked_at timestamptz);
 CREATE TABLE accounts(id bigint,proxy_id bigint,status text,schedulable bool,proxy_auto_paused bool,deleted_at timestamptz,expires_at timestamptz,auto_pause_on_expired bool,temp_unschedulable_until timestamptz,overload_until timestamptz,rate_limit_reset_at timestamptz);
 CREATE TABLE groups(id bigint PRIMARY KEY,name text,status text,platform text,deleted_at timestamptz);
 CREATE TABLE account_groups(account_id bigint,group_id bigint);
 INSERT INTO proxies SELECT i,NULL,'active',NULL,NOW()-INTERVAL '1 hour' FROM generate_series(1,7) AS i;
 UPDATE proxies SET deleted_at=NOW() WHERE id=7;
 INSERT INTO adobe_route_health SELECT id,updated_at,NOW()+INTERVAL '4 minutes',NOW()-INTERVAL '1 minute','model-a' FROM proxies WHERE id IN(1,4,6,7);
 INSERT INTO adobe_route_health SELECT id,updated_at,NOW()+INTERVAL '3 minutes',NOW()-INTERVAL '2 minutes','model-b' FROM proxies WHERE id=1;
 INSERT INTO adobe_route_health SELECT id,updated_at,NOW()-INTERVAL '1 minute',NOW()-INTERVAL '6 minutes','model-a' FROM proxies WHERE id=2;
 INSERT INTO adobe_route_health SELECT id,updated_at-INTERVAL '1 second',NOW()+INTERVAL '4 minutes',NOW()-INTERVAL '1 minute','model-a' FROM proxies WHERE id=5;
 INSERT INTO adobe_route_exclusions(proxy_id,proxy_updated_at,reason) SELECT id,updated_at-INTERVAL '1 day','private operator note' FROM proxies WHERE id IN(3,4);
 INSERT INTO accounts(id,proxy_id) VALUES(1,1),(2,1),(3,3);
 `)
	require.NoError(t, err)
	s := State{URL: "https://provider.example/private?token=secret", Revision: 11}
	for i := int64(1); i <= 7; i++ {
		s.Nodes = append(s.Nodes, Node{Name: "Example node", ProxyID: i, Enabled: i != 6,
			Password: "listener-secret", Config: map[string]any{"server": "192.0.2.10", "port": 443, "password": "upstream-secret"}})
	}
	// Duplicate imported references must not inflate the pause count.
	s.Nodes = append(s.Nodes, s.Nodes[0])
	normalizeSources(&s)
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE clash_subscription SET document=$1`, string(raw))
	require.NoError(t, err)

	// Enforce read-only SQL, in addition to checking the state afterward.
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`SET default_transaction_read_only=on`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(`SET default_transaction_read_only=off`) })
	v, err := NewManager(db).Health(context.Background())
	require.NoError(t, err)
	require.Len(t, v.Sources, 1)
	src := v.Sources[0]
	require.Equal(t, []int64{1, 3, 4, 6}, src.BlockedProxyIDs)
	require.Equal(t, 4, src.GenerationBlocked)
	require.Len(t, src.BlockedNodes, 4)
	require.Equal(t, src.GenerationBlocked, len(src.BlockedNodes))
	auto := src.BlockedNodes[0]
	require.False(t, auto.Manual)
	require.Equal(t, 2, auto.BoundAccounts)
	require.Equal(t, "192.0.2.10", auto.Server)
	require.Equal(t, 443, auto.ServerPort)
	require.NotNil(t, auto.FailedAt)
	require.NotNil(t, auto.RetryAfter)
	require.True(t, auto.RetryAfter.After(time.Now().Add(3*time.Minute)))
	require.Nil(t, auto.ExcludedAt)
	manual := src.BlockedNodes[1]
	require.True(t, manual.Manual, "operator exclusions survive proxy revision changes")
	require.NotNil(t, manual.ExcludedAt)
	require.Nil(t, manual.RetryAfter)
	require.True(t, src.BlockedNodes[2].Manual)
	require.NotNil(t, src.BlockedNodes[2].RetryAfter, "both causes must be visible")
	require.False(t, src.BlockedNodes[3].Enabled, "retained disabled nodes must be identified")
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	for _, secret := range []string{"token=secret", "listener-secret", "upstream-secret", "private operator note"} {
		require.NotContains(t, string(encoded), secret)
	}
	after, err := loadState(context.Background(), db, false)
	require.NoError(t, err)
	require.EqualValues(t, 11, after.Revision)
	var historical, manualCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_route_health WHERE proxy_id=2`).Scan(&historical))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM adobe_route_exclusions`).Scan(&manualCount))
	require.Equal(t, 1, historical, "expired cooldown history is not removed")
	require.Equal(t, 2, manualCount, "operator state is not changed")
}
