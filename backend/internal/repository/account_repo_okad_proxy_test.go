package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestOkadProxyReadinessAndPreservationPostgres(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	a := &service.Account{ID: 1}
	ready, err := repo.OkadVerificationProxyReady(ctx, a)
	require.NoError(t, err)
	require.True(t, ready, "explicit DIRECT remains usable when allocation is disabled")
	_, err = db.Exec(`UPDATE proxy_auto_allocation SET enabled=true`)
	require.NoError(t, err)
	ready, err = repo.OkadVerificationProxyReady(ctx, a)
	require.NoError(t, err)
	require.False(t, ready)
	id := int64(9)
	a.ProxyID = &id
	ready, err = repo.OkadVerificationProxyReady(ctx, a)
	require.NoError(t, err)
	require.False(t, ready, "missing loaded proxy must not fall back to DIRECT")
	a.Proxy = &service.Proxy{ID: id, Status: service.StatusActive}
	ready, err = repo.OkadVerificationProxyReady(ctx, a)
	require.NoError(t, err)
	require.True(t, ready)
	past := time.Now().Add(-time.Minute)
	a.Proxy.ExpiresAt = &past
	ready, err = repo.OkadVerificationProxyReady(ctx, a)
	require.NoError(t, err)
	require.False(t, ready)
	_, err = db.Exec(`ALTER TABLE accounts ADD COLUMN proxy_fallback_origin_id bigint, ADD COLUMN name text, ADD COLUMN group_ids bigint[], ADD COLUMN priority integer, ADD COLUMN concurrency integer, ADD COLUMN rate_multiplier numeric;
 INSERT INTO accounts(id,platform,type,status,schedulable,credentials,extra,updated_at,proxy_id,proxy_fallback_origin_id,name,group_ids,priority,concurrency,rate_multiplier)
 VALUES(1,'adobe','oauth','error',false,'{"email":"one@test.example","cookie":"old","model_mapping":{"model":"keep"}}','{}',NOW(),9,8,'kept-name','{3,4}',6,7,2.5)`)
	require.NoError(t, err)
	var c, e []byte
	a.Platform, a.Type = service.PlatformAdobe, service.AccountTypeOAuth
	require.NoError(t, db.QueryRow(`SELECT credentials,extra,status,schedulable,updated_at FROM accounts WHERE id=1`).Scan(&c, &e, &a.Status, &a.Schedulable, &a.UpdatedAt))
	require.NoError(t, json.Unmarshal(c, &a.Credentials))
	require.NoError(t, json.Unmarshal(e, &a.Extra))
	ok, err := repo.ApplyOkadCredential(ctx, a, service.OkadCredentialUpdate{Email: "one@test.example", Issuer: "fixture", SourceKey: "external:1", Version: 1, Cookie: "new"})
	require.NoError(t, err)
	require.True(t, ok)
	var preserved bool
	require.NoError(t, db.QueryRow(`SELECT proxy_id=9 AND proxy_fallback_origin_id=8 AND name='kept-name' AND group_ids=ARRAY[3,4]::bigint[] AND priority=6 AND concurrency=7 AND rate_multiplier=2.5 AND credentials->'model_mapping'->>'model'='keep' FROM accounts WHERE id=1`).Scan(&preserved))
	require.True(t, preserved, "credential synchronization must preserve routing, group, rate and model configuration")
	_, err = db.Exec(`INSERT INTO proxies(id) VALUES(9)`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT credentials,extra,status,schedulable,updated_at FROM accounts WHERE id=1`).Scan(&c, &e, &a.Status, &a.Schedulable, &a.UpdatedAt))
	require.NoError(t, json.Unmarshal(c, &a.Credentials))
	require.NoError(t, json.Unmarshal(e, &a.Extra))
	require.NoError(t, db.QueryRow(`SELECT updated_at FROM proxies WHERE id=9`).Scan(&a.Proxy.UpdatedAt))
	_, err = db.Exec(`UPDATE proxies SET updated_at=updated_at+INTERVAL '1 second' WHERE id=9`)
	require.NoError(t, err)
	ok, err = repo.CompleteOkadVerification(ctx, a, map[string]any{"access_token": "stale-proxy-token"}, "verified", "old route")
	require.NoError(t, err)
	require.False(t, ok, "editing an IP node without changing its ID invalidates the old probe")
}
