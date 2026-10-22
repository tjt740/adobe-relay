package proxyfailover

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVerificationPendingReclaimsInactiveSlotWithoutResuming(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	m := NewManager(db)
	_, err := db.Exec(`UPDATE proxies SET status='inactive' WHERE id<>1;
 INSERT INTO accounts(id,proxy_id,status,schedulable,error_message) VALUES(3,1,'error',false,'invalid credentials');
 INSERT INTO accounts(id,status,schedulable,error_message,credentials,extra) VALUES
 (1155,'error',false,'Okad credentials pending verification','{"cookie":"synthetic-cookie"}',
 '{"okad":{"resume":true,"verification":"proxy_pending","version":3},"okad_manual_pause":false}');`)
	require.NoError(t, err)
	snapshot := func() string {
		var raw string
		require.NoError(t, db.QueryRow(`SELECT row_to_json(t)::text FROM (SELECT status,schedulable,error_message,proxy_auto_paused,expires_at,extra,credentials FROM accounts WHERE id=1155) t`).Scan(&raw))
		return raw
	}
	before := snapshot()
	_, err = m.SaveAllocation(ctx, AllocationPolicy{Enabled: true})
	require.NoError(t, err)
	view, err := m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, view.VerificationWaiting)
	m.probe = func(_ context.Context, p proxy) health {
		if p.ID != 1 {
			return health{Status: "unhealthy"}
		}
		return health{Status: "healthy", LatencyMS: 20}
	}
	require.NoError(t, m.check(ctx))
	var pid int64
	require.NoError(t, db.QueryRow(`SELECT COALESCE(proxy_id,0) FROM accounts WHERE id=1155`).Scan(&pid))
	require.EqualValues(t, 1, pid, "a pending verification must reclaim the inactive binding instead of waiting forever")
	require.Equal(t, before, snapshot(), "allocation must not clear credential verification, manual intent or scheduling gates")
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE id IN (1,2) AND proxy_id=1 AND status='active' AND schedulable`).Scan(&pid))
	require.EqualValues(t, 2, pid, "working business routes keep their capacity")
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE id=3 AND proxy_id IS NULL AND status='error' AND NOT schedulable AND error_message='invalid credentials'`).Scan(&pid))
	require.EqualValues(t, 1, pid, "the displaced account keeps its original error and pause")
	require.NoError(t, m.check(ctx))
	require.Equal(t, before, snapshot(), "a later round must not treat the priority binding as permission to resume")
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM accounts WHERE proxy_id=1`).Scan(&pid))
	require.EqualValues(t, 3, pid)
	view, err = m.GetAllocation(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, view.VerificationPriorityAccounts)
	require.Zero(t, view.VerificationWaiting)
	require.Zero(t, view.ReclaimableSlots, "a route reserved for pending verification must not be advertised as reclaimable")
}

func TestVerificationPriorityCannotDisplaceBusinessOrUseUnhealthyNodes(t *testing.T) {
	now := time.Now()
	p := proxy{ID: 1, Status: "active"}
	h := health{Status: "healthy", fingerprint: p.fingerprint(), CheckedAt: now}
	accounts := []allocationAccount{
		{ID: 1, Current: 1, Eligible: true},
		{ID: 2, Current: 1, Eligible: true},
		{ID: 3, Current: 1, Eligible: true},
		{ID: 1155, Eligible: true, BindingOnly: true, VerificationPriority: true},
	}
	ps, hs := map[int64]proxy{1: p}, map[int64]health{1: h}
	next := planAllocation(accounts, ps, hs, now)
	require.Zero(t, next[1155], "existing business routes take precedence over verification")
	for _, id := range []int64{1, 2, 3} {
		require.EqualValues(t, 1, next[id])
	}
	accounts = accounts[3:]
	for _, state := range []string{"unhealthy", "unknown"} {
		h.Status = state
		hs[1] = h
		require.Zero(t, planAllocation(accounts, ps, hs, now)[1155])
	}
	h.Status, h.CheckedAt = "healthy", now.Add(-time.Hour)
	hs[1] = h
	require.Zero(t, planAllocation(accounts, ps, hs, now)[1155], "priority cannot bypass fresh connectivity evidence")
}

func TestVerificationPriorityRequiresRecoverableOwnedVerification(t *testing.T) {
	cases := []struct {
		name, verification, status, message, platform, accountType, cookie string
		manual, expired, schedulable, want                                 bool
	}{
		{name: "waiting for proxy", verification: "proxy_pending", want: true},
		{name: "new cookie awaiting first verification", verification: "pending", want: true},
		{name: "transient verification retry", verification: "verification_retry", want: true},
		{name: "legacy schedulable pending error", verification: "proxy_pending", schedulable: true, want: true},
		{name: "connection check awaiting credentials", verification: "proxy_pending", message: "Okad credentials pending connection test", want: true},
		{name: "manual pause", verification: "proxy_pending", manual: true},
		{name: "expired", verification: "proxy_pending", expired: true},
		{name: "no recovery intent", verification: "proxy_pending"},
		{name: "disabled", verification: "proxy_pending", status: "disabled"},
		{name: "authentication rejected", verification: "auth_failed"},
		{name: "identity rejected", verification: "identity_mismatch"},
		{name: "quota rejected", verification: "quota_blocked"},
		{name: "entitlement rejected", verification: "entitlement_blocked"},
		{name: "unrelated error", verification: "proxy_pending", message: "Authentication failed (401)"},
		{name: "other platform", verification: "proxy_pending", platform: "openai"},
		{name: "other account type", verification: "proxy_pending", accountType: "api_key"},
		{name: "missing cookie", verification: "proxy_pending", cookie: " "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			ctx := context.Background()
			status, message, platform, accountType, cookie := tc.status, tc.message, tc.platform, tc.accountType, tc.cookie
			if status == "" {
				status = "error"
			}
			if message == "" {
				message = "Okad credentials pending verification"
			}
			if platform == "" {
				platform = "adobe"
			}
			if accountType == "" {
				accountType = "oauth"
			}
			if cookie == "" {
				cookie = "synthetic-cookie"
			}
			resume := tc.name != "no recovery intent"
			extra, err := json.Marshal(map[string]any{"okad": map[string]any{"resume": resume, "verification": tc.verification}, "okad_manual_pause": tc.manual})
			require.NoError(t, err)
			credentials, err := json.Marshal(map[string]any{"cookie": cookie})
			require.NoError(t, err)
			_, err = db.Exec(`INSERT INTO accounts(id,proxy_id,status,schedulable,error_message) VALUES(3,1,'error',false,'invalid credentials')`)
			require.NoError(t, err)
			_, err = db.Exec(`INSERT INTO accounts(id,status,schedulable,error_message,platform,type,credentials,extra,expires_at)
 VALUES(1155,$1,$2,$3,$4,$5,$6,$7,CASE WHEN $8 THEN NOW()-INTERVAL '1 day' ELSE NULL END)`, status, tc.schedulable, message, platform, accountType, string(credentials), string(extra), tc.expired)
			require.NoError(t, err)
			accounts, err := loadAllocationAccounts(ctx, db)
			require.NoError(t, err)
			now := time.Now()
			p := proxy{ID: 1, Status: "active"}
			next := planAllocation(accounts, map[int64]proxy{1: p}, map[int64]health{1: {Status: "healthy", fingerprint: p.fingerprint(), CheckedAt: now}}, now)
			require.Equal(t, tc.want, next[1155] != 0)
		})
	}
}
