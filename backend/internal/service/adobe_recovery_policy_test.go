//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type adobeRecoveryPolicyRepo struct {
	*connectionTestResultRepo
	invalidations int
	cooldowns     int
}

func (r *adobeRecoveryPolicyRepo) InvalidateAdobeAccessTokenIfUnchanged(_ context.Context, id int64, token string) (bool, error) {
	a := r.accounts[id]
	if a.GetCredential("access_token") != token {
		return false, nil
	}
	r.invalidations++
	delete(a.Credentials, "access_token")
	return true, nil
}
func (r *adobeRecoveryPolicyRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	r.cooldowns++
	return nil
}

func (r *adobeRecoveryPolicyRepo) UpdateAdobeTokenIfCookieUnchanged(_ context.Context, id int64, cookie string, fields map[string]any) (bool, error) {
	a := r.accounts[id]
	if a == nil || a.GetCredential("cookie") != cookie {
		return false, nil
	}
	a.Credentials = MergeCredentials(a.Credentials, fields)
	return true, nil
}

func TestAdobeRecoveryPolicyAll401Entrances(t *testing.T) {
	for _, tc := range []struct {
		name, access, body   string
		invalidate, cooldown int
	}{
		{"token", "", `{"error":"expired"}`, 1, 0},
		{"quota_header", "taste_exhausted", `{}`, 0, 1},
		{"quota_body", "", `{"error_code":"taste_exhausted"}`, 0, 1},
		{"entitlement_body", "", `{"error_code":"model_not_entitled"}`, 0, 0},
		{"entitlement_header", "user_not_entitled", `{}`, 0, 0},
	} {
		for _, path := range []string{"generic", "account_test"} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				wantInvalidations := tc.invalidate
				a := adobeTestConnAccount(map[string]any{"cookie": "old-cookie", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
				r := &adobeRecoveryPolicyRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}}
				if path == "generic" {
					s := NewRateLimitService(r, nil, &config.Config{}, nil, nil)
					s.okadRecovery = newTestOkadRecovery(t, r.okadRecoveryRepo, func(http.ResponseWriter, *http.Request) { t.Error("401 alone must never login") })
					s.HandleUpstreamError(context.Background(), a, 401, http.Header{"X-Access-Error": []string{tc.access}}, []byte(tc.body))
				} else {
					fresh := adobeTestJWT(t, time.Now().Add(2*time.Hour))
					exchanges := 0
					api := &adobeFakeTransport{handler: func(request *adobe.Request, _ int) (*adobe.Response, error) {
						if strings.Contains(request.URL, "/check/v6/token") {
							exchanges++
							body, _ := json.Marshal(map[string]any{"access_token": fresh})
							return &adobe.Response{StatusCode: 200, Body: body}, nil
						}
						return &adobe.Response{StatusCode: 401, Headers: map[string]string{"x-access-error": tc.access}, Body: []byte(tc.body)}, nil
					}}
					withStubbedAdobeTestClient(t, adobe.NewClient(adobe.ClientConfig{Transport: api}))
					response, err := runAdobeTestConnWithRepo(t, r, a, "", "")
					require.Error(t, err)
					require.Contains(t, response.Body.String(), `"status_code":401`)
					require.Equal(t, []int{401}, r.codes)
					if tc.name == "token" {
						require.Equal(t, 1, exchanges)
						wantInvalidations = 2
					} else {
						require.Zero(t, exchanges)
					}
				}
				require.Equal(t, wantInvalidations, r.invalidations)
				require.Equal(t, tc.cooldown, r.cooldowns)
				require.Equal(t, "old-cookie", a.GetCredential("cookie"))
				require.Equal(t, StatusActive, a.Status)
				require.Empty(t, a.ErrorMessage)
				require.Empty(t, r.attempts)
			})
		}
	}
}

func TestAdobeRecoveryPolicy408DoesNotLoginOrInvalidate(t *testing.T) {
	a := recoveryFixtureAccount(912)
	a.Status, a.Schedulable, a.ErrorMessage = StatusActive, true, ""
	r := &adobeRecoveryPolicyRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}}
	s := &AccountTestService{accountRepo: r}
	s.recordConnectionTestResult(context.Background(), a, adobe.NewUpstreamTemporaryError("system under load", 408, adobe.ErrorTypeStatus))
	require.Equal(t, []int{408}, r.codes)
	require.Zero(t, r.invalidations)
	require.Zero(t, r.cooldowns)
	require.Empty(t, r.attempts)
	require.Equal(t, StatusActive, a.Status)
}
