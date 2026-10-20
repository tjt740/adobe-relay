//go:build unit

package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAdobeRuntimeSessionBoundToSourceCookie(t *testing.T) {
	a := &Account{Credentials: map[string]any{"cookie": "imported", "model_mapping": map[string]any{"x": "keep"}}}
	fields := adobeRefreshedSessionFields(a, &adobe.RefreshResult{AccessToken: "next-token", Cookie: "renewed", CookieUpdated: true})
	a.Credentials = MergeCredentials(a.Credentials, fields)
	require.Equal(t, "imported", a.GetCredential("cookie"), "Okad source revision remains immutable")
	require.Equal(t, "renewed", adobeEffectiveCookie(a))
	require.Equal(t, "renewed", adobeTokenCredentialFields(a.Credentials)[adobeSessionCookieKey])
	a.Credentials["cookie"] = "operator-replacement"
	require.Equal(t, "operator-replacement", adobeEffectiveCookie(a), "never apply a renewal to another source cookie")
	require.NotContains(t, adobeSessionRenewalOnly(fields), "access_token", "later quota failure may retain renewal but must not publish unverified token")
}

func TestAdobeDeletedSessionDoesNotFallBackToObsoleteImport(t *testing.T) {
	a := &Account{Credentials: map[string]any{"cookie": "imported"}}
	a.Credentials = MergeCredentials(a.Credentials, adobeRefreshedSessionFields(a, &adobe.RefreshResult{Cookie: "", CookieUpdated: true}))
	require.Empty(t, adobeEffectiveCookie(a))
	require.False(t, NewAdobeTokenRefresher().CanRefresh(a))
}

type renewalProbe struct {
	okadProbeFake
	seen string
}

func (p *renewalProbe) RefreshAccessTokenFromCookie(_ context.Context, c any, _ adobe.RefreshOptions) (*adobe.RefreshResult, error) {
	p.seen = c.(string)
	return &adobe.RefreshResult{AccessToken: "new-token", Cookie: "renewed-b", CookieUpdated: true}, nil
}
func (p *renewalProbe) FetchCreditsBalance(context.Context, string) (*adobe.CreditsBalance, error) {
	return nil, adobe.NewRequestError("temporary quota failure")
}
func TestOkadProbeRetainsRenewalAfterQuotaFailure(t *testing.T) {
	a := &Account{Credentials: map[string]any{"email": "one@example.test", "cookie": "imported"}}
	a.Credentials = MergeCredentials(a.Credentials, adobeRefreshedSessionFields(a, &adobe.RefreshResult{Cookie: "renewed-a", CookieUpdated: true}))
	probe := &renewalProbe{okadProbeFake: okadProbeFake{email: "one@example.test"}}
	fields, _, err := probeOkadCredential(context.Background(), a, probe)
	require.Error(t, err)
	require.Equal(t, "credits", okadVerificationStage(err))
	require.Equal(t, "renewed-a", probe.seen)
	require.Equal(t, "renewed-b", fields[adobeSessionCookieKey])
	require.NotContains(t, fields, "access_token")
	probe.email = "other@example.test"
	fields, _, err = probeOkadCredential(context.Background(), a, probe)
	require.ErrorIs(t, err, errOkadIdentityMismatch)
	require.Nil(t, fields)
}
