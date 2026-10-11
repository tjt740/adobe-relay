//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type okadRecoveryRepo struct {
	AccountRepository
	mu         sync.Mutex
	accounts   map[int64]*Account
	beforeSave func(*Account)
}

func (r *okadRecoveryRepo) QueueAdobeCookieRecoveryIfUnchanged(_ context.Context, a *Account) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	latest := r.accounts[a.ID]
	if latest.Status != StatusActive || !latest.Schedulable || latest.GetCredential("cookie") != a.GetCredential("cookie") {
		return false, nil
	}
	latest.Status = StatusError
	latest.Schedulable = false
	latest.ErrorMessage = "Authentication failed (401): Okad recovery pending"
	return true, nil
}

func (r *okadRecoveryRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.accounts[id]
	if a == nil {
		return nil, errors.New("missing")
	}
	clone := *a
	clone.Credentials = shallowCopyMap(a.Credentials)
	return &clone, nil
}
func (r *okadRecoveryRepo) ListWithFilters(_ context.Context, p pagination.PaginationParams, platform, typ, status, search string, group int64, privacy string) ([]Account, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var accounts []Account
	for _, a := range r.accounts {
		if a.Status == status {
			accounts = append(accounts, *a)
		}
	}
	return accounts, &pagination.PaginationResult{Pages: 1}, nil
}
func (r *okadRecoveryRepo) RecoverAdobeCookieIfUnchanged(_ context.Context, a *Account, cookie string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	latest := r.accounts[a.ID]
	if r.beforeSave != nil {
		r.beforeSave(latest)
	}
	if latest.Status != a.Status || latest.GetCredential("cookie") != a.GetCredential("cookie") {
		return false, nil
	}
	latest.Credentials = shallowCopyMap(latest.Credentials)
	latest.Credentials["cookie"] = cookie
	for _, k := range []string{"access_token", "expires_at", "_token_version"} {
		delete(latest.Credentials, k)
	}
	latest.Status = StatusActive
	latest.ErrorMessage = ""
	latest.Schedulable = true
	return true, nil
}
func recoveryFixtureAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusError, ErrorMessage: "OAuth 401 (no refresh_token): Invalid bearer token", Credentials: map[string]any{"email": "fixture@example.test", "cookie": "old", "access_token": "old-token", "expires_at": "old-exp", "_token_version": "old-ver", "model_mapping": map[string]any{"gpt-image-2": "keep"}}}
}
func newTestOkadRecovery(t *testing.T, repo *okadRecoveryRepo, handler http.HandlerFunc) *OkadCookieRecovery {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	s := newOkadCookieRecovery(repo, &config.Config{Gateway: config.GatewayConfig{OkadCookieRefreshURL: server.URL, OkadCookieRefreshAPIKey: "test-key", OkadCookieRefreshTimeoutSeconds: 2}})
	return s
}
func TestOkadRecoverySweepsHistoricalAndIMSFailures(t *testing.T) {
	a := recoveryFixtureAccount(901)
	b := recoveryFixtureAccount(902)
	b.ErrorMessage = "Token refresh failed (non-retryable): adobe cookie is no longer valid, re-export it from the browser: invalid_credentials"
	repo := &okadRecoveryRepo{accounts: map[int64]*Account{901: a, 902: b}}
	var calls atomic.Int32
	s := newTestOkadRecovery(t, repo, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "test-key", r.Header.Get("X-API-Key"))
		var req map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "fixture@example.test", req["email"])
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true,"cookie":"new"}`))
	})
	s.sweep(context.Background())
	require.EqualValues(t, 2, calls.Load())
	for _, a := range repo.accounts {
		require.Equal(t, StatusActive, a.Status)
		require.True(t, a.Schedulable)
		require.Empty(t, a.ErrorMessage)
		require.Equal(t, "new", a.GetCredential("cookie"))
		require.Contains(t, a.Credentials, "model_mapping")
		require.NotContains(t, a.Credentials, "access_token")
		require.NotContains(t, a.Credentials, "expires_at")
	}
	s.sweep(context.Background())
	require.EqualValues(t, 2, calls.Load())
}
func TestOkadRecoverySkipsNonAuthenticationStates(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Account)
	}{
		{"proxy removed", func(a *Account) {
			a.ErrorMessage = "Clash 订阅节点已移除，请重新选择代理并恢复账号调度"
		}},
		{"quota", func(a *Account) { a.ErrorMessage = "Quota exhausted (401)" }},
		{"disabled", func(a *Account) { a.Status = "disabled" }},
		{"different platform", func(a *Account) { a.Platform = PlatformOpenAI }},
		{"missing email", func(a *Account) { delete(a.Credentials, "email") }},
		{"identity mismatch", func(a *Account) { a.Extra = map[string]any{"adobeteam_external_email": "other@example.test"} }},
		{"expired", func(a *Account) { v := time.Now().Add(-time.Hour); a.ExpiresAt = &v }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := recoveryFixtureAccount(903)
			tc.change(a)
			require.False(t, isAdobeCookieRecoveryCandidate(a))
		})
	}
}
func TestOkadRecoveryFailureRetriesWithBackoff(t *testing.T) {
	a := recoveryFixtureAccount(904)
	repo := &okadRecoveryRepo{accounts: map[int64]*Account{904: a}}
	var calls atomic.Int32
	s := newTestOkadRecovery(t, repo, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":false,"message":"sensitive-cookie"}`))
	})
	s.sweep(context.Background())
	s.sweep(context.Background())
	require.EqualValues(t, 1, calls.Load())
	require.Equal(t, StatusError, a.Status)
	require.Equal(t, "old", a.GetCredential("cookie"))
	s.retry[904] = okadRecoveryAttempt{cookie: "old", next: time.Now().Add(-time.Minute)}
	s.sweep(context.Background())
	require.EqualValues(t, 2, calls.Load())
	_, err := s.recover(context.Background(), a)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sensitive-cookie")
}
func TestOkadRecoveryHonorsConcurrentPushOrDisable(t *testing.T) {
	for _, disable := range []bool{false, true} {
		a := recoveryFixtureAccount(905)
		repo := &okadRecoveryRepo{accounts: map[int64]*Account{905: a}, beforeSave: func(a *Account) {
			if disable {
				a.Status = "disabled"
			} else {
				a.Status = StatusActive
				a.Schedulable = true
				a.Credentials["cookie"] = "concurrent"
				a.Credentials["access_token"] = "concurrent-token"
			}
		}}
		s := newTestOkadRecovery(t, repo, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"cookie":"callback"}`))
		})
		recovered, err := s.recover(context.Background(), a)
		require.NoError(t, err)
		require.Equal(t, !disable, recovered)
		if disable {
			require.Equal(t, "disabled", a.Status)
			require.Equal(t, "old", a.GetCredential("cookie"))
		} else {
			require.Equal(t, "concurrent", a.GetCredential("cookie"))
			require.Equal(t, "concurrent-token", a.GetCredential("access_token"))
		}
	}
}
func TestOkadRecoveryRateLimit401UsesCallback(t *testing.T) {
	a := recoveryFixtureAccount(906)
	a.Status = StatusActive
	a.ErrorMessage = ""
	a.Schedulable = true
	repo := &okadRecoveryRepo{accounts: map[int64]*Account{906: a}}
	s := newTestOkadRecovery(t, repo, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"ok":true,"cookie":"fresh"}`)) })
	rate := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rate.okadRecovery = s
	require.True(t, rate.HandleUpstreamError(context.Background(), a, 401, http.Header{}, []byte(`{"error":"Invalid bearer token"}`)))
	require.Equal(t, StatusActive, a.Status)
	require.Equal(t, "fresh", a.GetCredential("cookie"))
}
func TestOkadRecoveryDisabledWithoutConfiguration(t *testing.T) {
	require.Nil(t, newOkadCookieRecovery(&okadRecoveryRepo{}, &config.Config{}))
}

func TestOkadRecoveryFailed401QueuesWithoutOverridingManualPause(t *testing.T) {
	for _, paused := range []bool{false, true} {
		a := recoveryFixtureAccount(907)
		a.Status, a.ErrorMessage, a.Schedulable = StatusActive, "", true
		repo := &okadRecoveryRepo{accounts: map[int64]*Account{907: a}}
		snapshot, err := repo.GetByID(context.Background(), a.ID)
		require.NoError(t, err)
		s := newTestOkadRecovery(t, repo, func(w http.ResponseWriter, r *http.Request) {
			if paused {
				repo.mu.Lock()
				a.Status, a.Schedulable = "disabled", false
				repo.mu.Unlock()
			}
			_, _ = w.Write([]byte(`{"ok":false,"message":"login failed"}`))
		})
		rate := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		rate.okadRecovery = s
		require.True(t, rate.HandleUpstreamError(context.Background(), snapshot, 401, http.Header{}, nil))
		if paused {
			require.Equal(t, "disabled", a.Status)
			require.Empty(t, a.ErrorMessage)
		} else {
			require.True(t, isAdobeCookieRecoveryCandidate(a))
		}
	}
}
