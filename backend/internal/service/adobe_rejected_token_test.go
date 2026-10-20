//go:build unit

package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

type rejectedTokenRepo struct{ *adobeGatewayCredsRepo }

func (r *rejectedTokenRepo) InvalidateAdobeAccessTokenIfUnchanged(_ context.Context, id int64, token string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.account == nil || r.account.ID != id || r.account.GetCredential("access_token") != token {
		return false, nil
	}
	c := shallowCopyMap(r.account.Credentials)
	delete(c, "access_token")
	r.account.Credentials = c
	return true, nil
}
func TestAdobeRejectedTokenRenewsFromEffectiveSession(t *testing.T) {
	old := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(time.Hour).Unix()})
	fresh := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(2 * time.Hour).Unix()})
	a := &Account{ID: 77, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": old}, Extra: map[string]any{}}
	a.Credentials = MergeCredentials(a.Credentials, adobeRefreshedSessionFields(a, &adobe.RefreshResult{AccessToken: old, Cookie: "ims_sid=runtime", CookieUpdated: true}))
	repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
	var calls atomic.Int32
	refresher := newAdobeTestRefresher(t, func(req *adobe.Request, _ int) (*adobe.Response, error) {
		calls.Add(1)
		require.Equal(t, "ims_sid=runtime", req.Headers["cookie"])
		b, _ := json.Marshal(map[string]any{"access_token": fresh})
		return &adobe.Response{StatusCode: 200, Body: b, SetCookies: []string{"ims_sid=next; Domain=.services.adobe.com; Path=/; Secure"}}, nil
	})
	p := newAdobeTokenProvider(repo, nil, refresher)
	token, err := p.recoverRejectedToken(context.Background(), a, old, true)
	require.NoError(t, err)
	require.Equal(t, fresh, token)
	require.EqualValues(t, 1, calls.Load())
	require.Equal(t, "ims_sid=source", repo.storedCredential("cookie"))
	require.Equal(t, "ims_sid=next", repo.storedCredential(adobeSessionCookieKey))
	// A stale rejection cannot clear the newer bearer or cause another IMS call.
	token, err = p.recoverRejectedToken(context.Background(), a, old, true)
	require.NoError(t, err)
	require.Equal(t, fresh, token)
	require.EqualValues(t, 1, calls.Load())
}

func TestAdobeRejectedTokenCannotCrossManualPauseDuringRenewal(t *testing.T) {
	old := adobeTestToken(t, map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
	fresh := adobeTestToken(t, map[string]any{"exp": time.Now().Add(2 * time.Hour).Unix()})
	a := &Account{ID: 78, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": old}}
	repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
	repo.beforeCAS = func(current *Account) { current.Schedulable = false }
	p := newAdobeTokenProvider(repo, nil, newAdobeTestRefresher(t, adobeIMSTokenHandler(fresh, nil)))
	_, err := p.recoverRejectedToken(context.Background(), a, old, true)
	require.ErrorIs(t, err, errOAuthRefreshAccountStateChanged)
}

func TestAdobeRejectedTokenIdentityMismatchNotPersisted(t *testing.T) {
	old := adobeTestToken(t, map[string]any{"user_id": "expected-org", "exp": time.Now().Add(time.Hour).Unix()})
	other := adobeTestToken(t, map[string]any{"user_id": "other-org", "exp": time.Now().Add(2 * time.Hour).Unix()})
	a := &Account{ID: 79, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": old}}
	repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
	p := newAdobeTokenProvider(repo, nil, newAdobeTestRefresher(t, adobeIMSTokenHandler(other, nil)))
	_, err := p.recoverRejectedToken(context.Background(), a, old, true)
	require.Error(t, err)
	require.NotEqual(t, other, repo.storedCredential("access_token"))
	require.Empty(t, repo.storedCredential(adobeSessionCookieKey))
}
