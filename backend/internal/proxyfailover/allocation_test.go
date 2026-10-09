package proxyfailover

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAllocationPacksThreeAndKeepsWorkingBindings(t *testing.T) {
	now := time.Now()
	ps := map[int64]proxy{}
	hs := map[int64]health{}
	for _, id := range []int64{1, 2, 3} {
		p := proxy{ID: id, Status: "active"}
		ps[id] = p
		hs[id] = health{ProxyID: id, Status: "healthy", fingerprint: p.fingerprint(), CheckedAt: now, LatencyMS: id * 10}
	}
	accounts := []allocationAccount{{ID: 1, Current: 2, Eligible: true}, {ID: 9, Current: 1}} // manual binding consumes one slot
	for id := int64(2); id <= 8; id++ {
		accounts = append(accounts, allocationAccount{ID: id, Eligible: true, Waiting: true})
	}
	next := planAllocation(accounts, ps, hs, now)
	require.EqualValues(t, 2, next[1], "keep an existing healthy binding")
	require.EqualValues(t, 1, next[2], "fill a partially occupied node first; latency breaks ties")
	used := map[int64]int{1: 1}
	for _, id := range next {
		require.NotZero(t, id)
		used[id]++
	}
	require.Equal(t, map[int64]int{1: 3, 2: 3, 3: 3}, used)
	accounts = append(accounts, allocationAccount{ID: 10, Eligible: true, Waiting: true})
	next = planAllocation(accounts, ps, hs, now)
	require.Zero(t, next[10], "never exceed three when capacity runs out")
	for id, h := range hs {
		h.CheckedAt = now.Add(-time.Minute)
		hs[id] = h
	}
	next = planAllocation(accounts, ps, hs, now)
	require.EqualValues(t, 2, next[1], "do not move a working binding just because a probe is late")
	require.Zero(t, next[2], "new bindings need fresh healthy evidence")
}

func TestAutomaticAllocationLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	exec := func(q string) { t.Helper(); _, err := db.Exec(q); require.NoError(t, err) }
	state := func(id int64, proxyID int64, waiting bool) {
		t.Helper()
		var pid int64
		var paused, schedulable bool
		require.NoError(t, db.QueryRow(`SELECT COALESCE(proxy_id,0),proxy_auto_paused,schedulable FROM accounts WHERE id=$1`, id).Scan(&pid, &paused, &schedulable))
		require.Equal(t, proxyID, pid)
		require.Equal(t, waiting, paused)
		require.Equal(t, !waiting, schedulable)
	}
	_, err := m.Save(ctx, 1, Policy{Enabled: true, PrimaryProxyID: 1, BackupProxyIDs: []int64{2}, CurrentProxyID: 1})
	require.NoError(t, err)
	exec(`INSERT INTO accounts(id) SELECT generate_series(3,8)`)
	policy, err := m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	require.Equal(t, 8, policy.Waiting)
	state(3, 0, true) // no unverified DIRECT account can be scheduled
	var mu sync.Mutex
	failed := map[int64]bool{3: true, 4: true}
	m.probe = func(_ context.Context, p proxy) health {
		mu.Lock()
		defer mu.Unlock()
		if failed[p.ID] {
			return health{Status: "unhealthy"}
		}
		return health{Status: "healthy", LatencyMS: p.ID * 10}
	}
	require.NoError(t, m.check(ctx))
	for _, id := range []int64{1, 2, 3} {
		state(id, 1, false)
	}
	for _, id := range []int64{4, 5, 6} {
		state(id, 2, false)
	}
	state(7, 0, true)
	state(8, 0, true)
	v, err := m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 6, v.Assigned)
	require.Equal(t, 2, v.Waiting)
	require.Equal(t, 0, v.AvailableSlots)
	individual, err := m.Get(ctx, 1)
	require.NoError(t, err)
	require.True(t, individual.AutoManaged)
	require.False(t, individual.Enabled)
	_, err = m.Save(ctx, 1, individual.Policy)
	require.ErrorIs(t, err, ErrAutoManaged)
	// New accounts and manual rebindings wait for capacity validation, even if
	// the requested node already has three accounts.
	exec(`INSERT INTO accounts(id,proxy_id) VALUES(9,1)`)
	state(9, 1, true)
	require.NoError(t, m.check(ctx))
	state(9, 0, true)
	age := func() { exec(`UPDATE proxy_failover_health SET checked_at=NOW()-INTERVAL '31 seconds'`) }
	mu.Lock()
	failed[1] = true
	failed[3] = false
	mu.Unlock()
	age()
	require.NoError(t, m.check(ctx))
	state(1, 1, false) // one failure does not switch
	age()
	require.NoError(t, m.check(ctx))
	// Node 3 was partly filled while node 1 was being confirmed unhealthy.
	v, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 6, v.Assigned)
	require.Equal(t, 3, v.Waiting)
	for _, node := range v.Nodes {
		require.LessOrEqual(t, node.Accounts, 3)
	}
	mu.Lock()
	failed[1] = false
	failed[4] = false
	mu.Unlock()
	age()
	require.NoError(t, m.check(ctx))
	v, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 9, v.Assigned)
	require.Zero(t, v.Waiting)
	// Unrelated credential errors / manual disables must never be undone.
	exec(`UPDATE accounts SET status='error',error_message='credentials expired',schedulable=false WHERE id=1;
 UPDATE accounts SET status='disabled',schedulable=false WHERE id=2`)
	require.NoError(t, m.check(ctx))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE id IN (1,2) AND NOT schedulable AND NOT proxy_auto_paused`).Scan(&count))
	require.Equal(t, 2, count)
	// Subscription retirement transfers its pause to the allocator.
	exec(`UPDATE accounts SET status='error',schedulable=false,proxy_id=NULL,error_message='Clash 订阅节点已移除，请重新选择代理并恢复账号调度' WHERE id=3`)
	state(3, 0, true)
	require.NoError(t, m.check(ctx))
	v, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Zero(t, v.Waiting)
	_, err = m.SaveAllocation(ctx, AllocationPolicy{Enabled: false, Revision: 0})
	require.ErrorIs(t, err, ErrConflict)
	_, err = m.SaveAllocation(ctx, AllocationPolicy{Enabled: false, Revision: policy.Revision})
	require.NoError(t, err)
	exec(`INSERT INTO accounts(id) VALUES(10)`)
	state(10, 0, false) // disabled feature preserves original behavior
}

func TestAllocationConcurrentEditsAndAtomicity(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	policy, err := m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	var once sync.Once
	m.probe = func(_ context.Context, p proxy) health {
		once.Do(func() {
			_, e := db.Exec(`UPDATE proxies SET port=8081,updated_at=clock_timestamp() WHERE id=1;
 INSERT INTO accounts(id) VALUES(3),(4),(5),(6),(7),(8),(9),(10)`)
			require.NoError(t, e)
		})
		return health{Status: "healthy", LatencyMS: p.ID * 10}
	}
	require.NoError(t, m.check(ctx))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_id=1`).Scan(&count))
	require.Zero(t, count, "edited endpoint cannot inherit old health")
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_auto_paused`).Scan(&count))
	require.Equal(t, 1, count, "concurrent imports included before capacity allocation")
	_, err = db.Exec(`DELETE FROM scheduler_outbox; ALTER TABLE scheduler_outbox ADD CONSTRAINT reject_event CHECK(event_type='reject')`)
	require.NoError(t, err)
	require.Error(t, m.check(ctx), "outbox failure rolls back all allocation changes")
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_auto_paused`).Scan(&count))
	require.Equal(t, 1, count)
	_, err = db.Exec(`ALTER TABLE scheduler_outbox DROP CONSTRAINT reject_event`)
	require.NoError(t, err)
	require.NoError(t, m.check(ctx))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_auto_paused`).Scan(&count))
	require.Zero(t, count)
	// Disabling while network checks run must prevent a late allocation commit.
	_, err = db.Exec(`UPDATE proxy_failover_health SET checked_at=NOW()-INTERVAL '31 seconds'; INSERT INTO accounts(id) VALUES(11)`)
	require.NoError(t, err)
	once = sync.Once{}
	m.probe = func(_ context.Context, p proxy) health {
		once.Do(func() {
			_, e := m.SaveAllocation(ctx, AllocationPolicy{Enabled: false, Revision: policy.Revision})
			require.NoError(t, e)
		})
		return health{Status: "healthy"}
	}
	require.NoError(t, m.check(ctx))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE id=11 AND proxy_auto_paused AND proxy_id IS NULL`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestExistingAccountsBindWithoutChangingSchedulingState(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	_, err := db.Exec(`INSERT INTO accounts(id,proxy_id,status,schedulable,error_message,expires_at) VALUES
 (3,1,'error',false,'cookie expired',NULL),
 (4,1,'disabled',false,'manually disabled',NULL),
 (5,1,'active',false,NULL,NULL),
 (6,1,'active',true,NULL,NOW()-INTERVAL '1 day'),
 (7,NULL,'error',true,'invalid credentials',NULL),
 (8,NULL,'error',false,$1,NOW()-INTERVAL '1 day')`, allocationWaitingMessage)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET proxy_auto_paused=true WHERE id=8;
 INSERT INTO accounts(id,proxy_id,platform) VALUES(10,4,'openai');
 INSERT INTO accounts(id,proxy_id,deleted_at) VALUES(11,1,NOW());`)
	require.NoError(t, err)
	snapshot := func() string {
		t.Helper()
		var raw string
		require.NoError(t, db.QueryRow(`SELECT json_agg(s ORDER BY id)::text FROM
 (SELECT id,status,schedulable,error_message,expires_at,proxy_auto_paused FROM accounts WHERE id BETWEEN 3 AND 8) s`).Scan(&raw))
		return raw
	}
	before := snapshot()
	policy, err := m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	require.Equal(t, 8, policy.Accounts, "existing disabled, expired and error accounts are included")
	require.Equal(t, 6, policy.BindingOnlyAccounts)
	var failed bool
	m.probe = func(_ context.Context, p proxy) health {
		if failed {
			return health{Status: "unhealthy"}
		}
		return health{Status: "healthy", LatencyMS: p.ID * 10}
	}
	check := func() {
		t.Helper()
		require.NoError(t, m.check(ctx))
		require.Equal(t, before, snapshot(), "proxy changes must preserve every original scheduling/error/expiry field")
		var other int64
		require.NoError(t, db.QueryRow(`SELECT proxy_id FROM accounts WHERE id=10`).Scan(&other))
		require.EqualValues(t, 4, other, "other platforms are never reallocated")
		require.NoError(t, db.QueryRow(`SELECT proxy_id FROM accounts WHERE id=11`).Scan(&other))
		require.EqualValues(t, 1, other, "deleted accounts are never changed")
	}
	check()
	v, err := m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 8, v.Assigned)
	require.Zero(t, v.Waiting)
	require.Equal(t, 6, v.BindingOnlyAccounts, "an expired allocator-paused account can have an IP without being resumed")
	for _, node := range v.Nodes {
		require.LessOrEqual(t, node.Accounts, 3)
	}
	age := func() {
		_, e := db.Exec(`UPDATE proxy_failover_health SET checked_at=NOW()-INTERVAL '31 seconds'`)
		require.NoError(t, e)
	}
	failed = true
	age()
	check() // first failure keeps existing routes
	age()
	check() // second failure detaches unavailable routes, preserving stopped account states
	v, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 8, v.Waiting)
	require.Zero(t, v.Assigned)
	failed = false
	age()
	check() // capacity recovery binds existing inactive accounts too
	v, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 8, v.Assigned)
	require.Zero(t, v.Waiting)
	// Background rounds are idempotent once all existing accounts have routes.
	var eventsBefore, eventsAfter int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM proxy_failover_events`).Scan(&eventsBefore))
	check()
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM proxy_failover_events`).Scan(&eventsAfter))
	require.Equal(t, eventsBefore, eventsAfter)
}
