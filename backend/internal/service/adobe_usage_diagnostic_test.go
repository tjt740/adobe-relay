//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

func TestAdobeManualUsageDiagnosticRefreshesQuarantinedWithoutResuming(t *testing.T) {
	for _, paused := range []bool{false, true} {
		name := "quarantined"
		if paused {
			name = "operator-paused"
		}
		t.Run(name, func(t *testing.T) {
			old := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(time.Hour).Unix()})
			fresh := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(2 * time.Hour).Unix()})
			a := &Account{ID: 81, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusError, Schedulable: false, ErrorMessage: "Token refresh failed (non-retryable): old session", Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": old}, Extra: map[string]any{"okad_manual_pause": paused}}
			repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
			var exchanges atomic.Int32
			refresher := newAdobeTestRefresher(t, func(req *adobe.Request, n int) (*adobe.Response, error) {
				exchanges.Add(1)
				return adobeIMSTokenHandler(fresh, nil)(req, n)
			})
			calls := 0
			client := adobeAuthCreditsFunc(func(_ context.Context, token string) (*adobe.CreditsBalance, error) {
				calls++
				if calls == 1 {
					return nil, adobe.NewAuthError("rejected", 401)
				}
				require.Equal(t, fresh, token)
				return &adobe.CreditsBalance{Available: int64Ptr(12)}, nil
			})
			svc := newAdobeUsageTestService(t, repo, client)
			svc.adobeTokenProvider = newAdobeTokenProvider(repo, nil, refresher)
			info, err := svc.getAdobeUsage(context.Background(), a, "active", true)
			require.NoError(t, err)
			require.Empty(t, info.ErrorCode)
			require.Equal(t, 2, calls)
			require.EqualValues(t, 1, exchanges.Load())
			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Equal(t, StatusError, repo.account.Status)
			require.False(t, repo.account.Schedulable)
			require.Equal(t, a.ErrorMessage, repo.account.ErrorMessage)
			require.Equal(t, paused, repo.account.Extra["okad_manual_pause"])
		})
	}
}

func TestAdobeUsageDiagnosticDoesNotExpandBackgroundOrDisabledEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, status, source string
		force, expired       bool
	}{
		{"passive", StatusError, "passive", true, false},
		{"automatic", StatusError, "active", false, false},
		{"disabled", StatusDisabled, "active", true, false},
		{"inactive", "inactive", "active", true, false},
		{"expired", StatusError, "active", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := adobeTestToken(t, map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
			a := &Account{ID: 82, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: tc.status, Schedulable: false, Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": token}}
			if tc.expired {
				expired := time.Now().Add(-time.Hour)
				a.ExpiresAt = &expired
			}
			repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
			var exchanges atomic.Int32
			refresher := newAdobeTestRefresher(t, func(*adobe.Request, int) (*adobe.Response, error) {
				exchanges.Add(1)
				t.Error("ineligible check exchanged a token")
				return nil, context.Canceled
			})
			calls := 0
			client := adobeAuthCreditsFunc(func(context.Context, string) (*adobe.CreditsBalance, error) {
				calls++
				return nil, adobe.NewAuthError("rejected", 401)
			})
			svc := newAdobeUsageTestService(t, repo, client)
			svc.adobeTokenProvider = newAdobeTokenProvider(repo, nil, refresher)
			info, err := svc.getAdobeUsage(context.Background(), a, tc.source, tc.force)
			require.NoError(t, err)
			require.NotEmpty(t, info.ErrorCode)
			require.Equal(t, 1, calls)
			require.Zero(t, exchanges.Load())
		})
	}
}

func TestAdobeManualUsageDiagnosticDoesNotJoinBackgroundFlight(t *testing.T) {
	for _, backgroundTimeout := range []bool{false, true} {
		name := "background-auth-rejection"
		if backgroundTimeout {
			name = "late-background-timeout"
		}
		t.Run(name, func(t *testing.T) {
			old := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(time.Hour).Unix()})
			fresh := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(2 * time.Hour).Unix()})
			a := &Account{ID: 83, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusError, Schedulable: false, Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": old}}
			repo := &rejectedTokenRepo{newAdobeGatewayCredsRepo(a)}
			var exchanges, calls atomic.Int32
			refresher := newAdobeTestRefresher(t, func(req *adobe.Request, n int) (*adobe.Response, error) {
				exchanges.Add(1)
				return adobeIMSTokenHandler(fresh, nil)(req, n)
			})
			started, release := make(chan struct{}), make(chan struct{})
			client := adobeAuthCreditsFunc(func(ctx context.Context, token string) (*adobe.CreditsBalance, error) {
				if calls.Add(1) == 1 {
					close(started)
					select {
					case <-release:
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					if backgroundTimeout {
						return nil, context.DeadlineExceeded
					}
				}
				if token == old {
					return nil, adobe.NewAuthError("rejected", 401)
				}
				return &adobe.CreditsBalance{Total: int64Ptr(15), Available: int64Ptr(12)}, nil
			})
			svc := newAdobeUsageTestService(t, repo, client)
			svc.adobeTokenProvider = newAdobeTokenProvider(repo, nil, refresher)
			passiveDone := make(chan struct{})
			go func() {
				defer close(passiveDone)
				_, _ = svc.getAdobeUsage(context.Background(), a, "passive", false)
			}()
			<-started
			type result struct {
				info *UsageInfo
				err  error
			}
			manualDone := make(chan result, 1)
			go func() {
				info, err := svc.getAdobeUsage(context.Background(), a, "active", true)
				manualDone <- result{info, err}
			}()
			var got result
			select {
			case got = <-manualDone:
				close(release)
			case <-time.After(2 * time.Second):
				close(release)
				got = <-manualDone
			}
			<-passiveDone
			require.NoError(t, got.err)
			require.NotNil(t, got.info)
			require.Empty(t, got.info.ErrorCode, "manual revalidation inherited a background query's non-diagnostic refresh policy")
			require.EqualValues(t, 1, exchanges.Load())
			require.Equal(t, fresh, repo.storedCredential("access_token"))
			cached, ok := svc.getCachedAdobeUsage(a.ID)
			require.True(t, ok)
			require.Empty(t, cached.ErrorCode, "an older background failure replaced the successful manual result")
			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Equal(t, StatusError, repo.account.Status)
			require.False(t, repo.account.Schedulable)

		})
	}
}
