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

type adobeAuthCreditsFunc func(context.Context, string) (*adobe.CreditsBalance, error)

func (f adobeAuthCreditsFunc) FetchCreditsBalance(ctx context.Context, token string) (*adobe.CreditsBalance, error) {
	return f(ctx, token)
}

func TestAdobeUsageRevalidateRenewsRejectedTokenOnce(t *testing.T) {
	for _, failsAgain := range []bool{false, true} {
		name := "recovers"
		if failsAgain {
			name = "second-rejection-stops"
		}
		t.Run(name, func(t *testing.T) {
			old := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(time.Hour).Unix()})
			fresh := adobeTestToken(t, map[string]any{"user_id": "org-user", "exp": time.Now().Add(2 * time.Hour).Unix()})
			a := &Account{ID: 80, Platform: PlatformAdobe, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"cookie": "ims_sid=source", "access_token": old}}
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
					require.Equal(t, old, token)
				} else {
					require.Equal(t, fresh, token)
				}
				if calls == 1 || failsAgain {
					return nil, adobe.NewAuthError("rejected", 401)
				}
				return &adobe.CreditsBalance{Total: int64Ptr(15), Available: int64Ptr(12), Used: int64Ptr(3)}, nil
			})
			svc := newAdobeUsageTestService(t, repo, client)
			svc.adobeTokenProvider = newAdobeTokenProvider(repo, nil, refresher)
			info, err := svc.getAdobeUsage(context.Background(), a, "active", true)
			require.NoError(t, err)
			require.Equal(t, 2, calls)
			require.EqualValues(t, 1, exchanges.Load())
			require.Equal(t, "ims_sid=source", repo.storedCredential("cookie"))
			if failsAgain {
				require.True(t, info.NeedsReauth)
				require.Equal(t, errorCodeUnauthenticated, info.ErrorCode)
				require.Empty(t, repo.storedCredential("access_token"))
			} else {
				require.Empty(t, info.ErrorCode)
				require.NotNil(t, info.AdobeCredit)
				require.Equal(t, fresh, repo.storedCredential("access_token"))
			}
		})
	}
}
