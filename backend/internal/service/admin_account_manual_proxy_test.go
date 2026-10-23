//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestManualProxyEditPreservesAccountHealthAndPause(t *testing.T) {
	repo := &updateAccountOveragesRepoStub{account: &Account{ID: 7, Platform: "adobe", Type: "oauth", Status: StatusError, Schedulable: false, ErrorMessage: "HTTP 408", Extra: map[string]any{"okad_manual_pause": true, "adobe_health": map[string]any{"pending": true}}}}
	svc := &adminServiceImpl{accountRepo: repo}
	enabled := true
	proxy := int64(3)
	account, err := svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{ManualProxyEnabled: &enabled, ProxyID: &proxy})
	require.NoError(t, err)
	require.Equal(t, true, account.Extra["manual_proxy_enabled"])
	require.Equal(t, true, account.Extra["okad_manual_pause"])
	require.Contains(t, account.Extra, "adobe_health")
	require.Equal(t, "HTTP 408", account.ErrorMessage)
	require.False(t, account.Schedulable)
	require.Equal(t, &proxy, account.ProxyID)
	// An old client that replaces extra cannot silently opt back into automation.
	account, err = svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{Extra: map[string]any{"custom": true}})
	require.NoError(t, err)
	require.Equal(t, true, account.Extra["manual_proxy_enabled"])
	enabled = false
	account, err = svc.UpdateAccount(context.Background(), 7, &UpdateAccountInput{ManualProxyEnabled: &enabled})
	require.NoError(t, err)
	require.Equal(t, false, account.Extra["manual_proxy_enabled"])
	require.False(t, account.Schedulable)
}
