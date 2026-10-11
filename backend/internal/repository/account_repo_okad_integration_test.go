package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOkadVersionedDeliveryPostgres(t *testing.T) {
	db := newOkadRecoveryPostgres(t)
	repo := &accountRepository{sql: db}
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO accounts(id,platform,type,status,error_message,schedulable,credentials,extra,updated_at) VALUES(1,'adobe','oauth','active','',true,'{"email":"one@test.example","cookie":"old","model_mapping":{"x":"keep"}}','{}',NOW())`)
	require.NoError(t, err)
	read := func() *service.Account {
		a := &service.Account{ID: 1, Platform: "adobe", Type: "oauth"}
		var creds, extra []byte
		require.NoError(t, db.QueryRow(`SELECT credentials,extra,status,COALESCE(error_message,''),schedulable,updated_at,proxy_id FROM accounts WHERE id=1`).Scan(&creds, &extra, &a.Status, &a.ErrorMessage, &a.Schedulable, &a.UpdatedAt, &a.ProxyID))
		require.NoError(t, json.Unmarshal(creds, &a.Credentials))
		require.NoError(t, json.Unmarshal(extra, &a.Extra))
		return a
	}
	apply := func(version int64, cookie string) (bool, error) {
		return repo.ApplyOkadCredential(ctx, read(), service.OkadCredentialUpdate{Email: "one@test.example", Issuer: "okad-test", SourceKey: "external:1", Version: version, Cookie: cookie})
	}
	applied, err := apply(2, "new")
	require.NoError(t, err)
	require.True(t, applied)
	a := read()
	require.False(t, a.Schedulable)
	require.Equal(t, service.OkadVerificationPending, a.ErrorMessage)
	require.Contains(t, a.Credentials, "model_mapping")
	for _, version := range []int64{1, 2} {
		applied, err = apply(version, "stale")
		require.NoError(t, err)
		require.False(t, applied)
	}
	applied, err = repo.CompleteOkadVerification(ctx, a, map[string]any{"access_token": "verified"}, "verified", "verified")
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, read().Schedulable)
	require.Equal(t, "new", read().GetCredential("cookie"))
	// Transient upstream timeouts must not take a previously healthy account out
	// of service. An invalid credential must, and its cached token must be removed.
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "verification_retry", "timeout")
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, read().Schedulable)
	require.Equal(t, "verified", read().GetCredential("access_token"))
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "auth_failed", "Authentication failed (401): Adobe rejected the new cookie")
	require.NoError(t, err)
	require.True(t, applied)
	require.False(t, read().Schedulable)
	require.Empty(t, read().GetCredential("access_token"))
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "verification_retry", "timeout")
	require.NoError(t, err)
	require.True(t, applied)
	require.False(t, read().Schedulable)
	applied, err = repo.CompleteOkadVerification(ctx, read(), map[string]any{"access_token": "renewed"}, "verified", "verified")
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, read().Schedulable)
	// Revalidating an already active account must quarantine a definitive failure.
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "quota_blocked", "no credits")
	require.NoError(t, err)
	require.True(t, applied)
	require.False(t, read().Schedulable)
	require.Equal(t, service.OkadVerificationPending, read().ErrorMessage)
	applied, err = repo.CompleteOkadVerification(ctx, read(), map[string]any{"access_token": "refreshed"}, "verified", "verified")
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, read().Schedulable)
	applied, err = apply(3, "newer")
	require.NoError(t, err)
	require.True(t, applied)
	snapshot := read()
	require.NoError(t, repo.SetSchedulable(ctx, 1, false))
	applied, err = repo.CompleteOkadVerification(ctx, snapshot, nil, "verified", "verified")
	require.NoError(t, err)
	require.False(t, applied)
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "verified", "verified")
	require.NoError(t, err)
	require.True(t, applied)
	require.False(t, read().Schedulable)
	_, err = db.Exec(`UPDATE accounts SET status='inactive',schedulable=false,updated_at=NOW() WHERE id=1`)
	require.NoError(t, err)
	applied, err = apply(4, "latest")
	require.NoError(t, err)
	require.True(t, applied)
	applied, err = repo.CompleteOkadVerification(ctx, read(), nil, "verified", "verified")
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, "inactive", read().Status)
	require.False(t, read().Schedulable)
	p := service.OkadCredentialUpdate{Email: "one@test.example", Issuer: "other", SourceKey: "pool:1", Version: 100, Cookie: "wrong"}
	applied, err = repo.ApplyOkadCredential(ctx, read(), p)
	require.NoError(t, err)
	require.False(t, applied)
	items, err := repo.ListOkadInventory(ctx, 0, 200, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	raw, err := json.Marshal(items)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "latest")
	require.NotContains(t, string(raw), "access_token")
}
