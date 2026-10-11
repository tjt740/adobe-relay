package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type okadProxyPendingRepo struct {
	AccountRepository
	a    *Account
	code string
}

func (r *okadProxyPendingRepo) OkadVerificationProxyReady(context.Context, *Account) (bool, error) {
	return false, nil
}
func (r *okadProxyPendingRepo) ApplyOkadCredential(context.Context, *Account, OkadCredentialUpdate) (bool, error) {
	panic("unexpected write")
}
func (r *okadProxyPendingRepo) ListOkadInventory(context.Context, int64, int, *time.Time) ([]OkadInventoryItem, error) {
	return nil, nil
}
func (r *okadProxyPendingRepo) CompleteOkadVerification(_ context.Context, _ *Account, fields map[string]any, code, message string) (bool, error) {
	if len(fields) != 0 {
		panic("proxy wait must not acquire tokens")
	}
	r.code = code
	return true, nil
}
func (r *okadProxyPendingRepo) GetByID(context.Context, int64) (*Account, error) { return r.a, nil }

func TestOkadVerificationWaitsForProxyWithoutDirectRequests(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformAdobe, Type: AccountTypeOAuth, Extra: map[string]any{"okad": map[string]any{"version": 1}}}
	repo := &okadProxyPendingRepo{a: a}
	previous := adobeUsageClients
	adobeUsageClients = &adobeClientCache{newClient: func(string) *adobe.Client {
		t.Fatal("must not create an upstream client before proxy is ready")
		return nil
	}}
	t.Cleanup(func() { adobeUsageClients = previous })
	_, err := (&AccountUsageService{accountRepo: repo}).VerifyOkadCredential(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, "proxy_pending", repo.code)
}
