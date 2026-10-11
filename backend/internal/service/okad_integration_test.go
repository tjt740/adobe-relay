package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type okadProbeFake struct {
	email    string
	balances int
}

func (f *okadProbeFake) RefreshAccessTokenFromCookie(context.Context, any, adobe.RefreshOptions) (*adobe.RefreshResult, error) {
	return &adobe.RefreshResult{AccessToken: "fresh-token", ExpiresIn: 3600}, nil
}
func (f *okadProbeFake) FetchAccountInfo(context.Context, string) (*adobe.AccountInfo, error) {
	return &adobe.AccountInfo{Email: f.email}, nil
}
func (f *okadProbeFake) FetchCreditsBalance(context.Context, string) (*adobe.CreditsBalance, error) {
	f.balances++
	return &adobe.CreditsBalance{}, nil
}

func TestOkadProbeChecksAdobeIdentityBeforeBalance(t *testing.T) {
	a := &Account{Credentials: map[string]any{"email": "one@test.example", "cookie": "cookie"}}
	f := &okadProbeFake{email: "other@test.example"}
	fields, _, err := probeOkadCredential(context.Background(), a, f)
	require.ErrorIs(t, err, errOkadIdentityMismatch)
	require.Nil(t, fields)
	require.Zero(t, f.balances)
	f.email = "ONE@test.example"
	fields, _, err = probeOkadCredential(context.Background(), a, f)
	require.NoError(t, err)
	require.Equal(t, "fresh-token", fields["access_token"])
	require.NotContains(t, fields, "cookie")
	require.Equal(t, 1, f.balances)
}

func TestOkadManagedResponseRequiresMatchingJobAndEmail(t *testing.T) {
	for _, tc := range []struct{ name, email, id, code string }{
		{"valid", "one@test.example", "request-1", ""},
		{"wrong job", "one@test.example", "other", "protocol"},
		{"wrong identity", "other@test.example", "request-1", "identity_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "local-key", r.Header.Get("X-API-Key"))
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "request-1", body["request_id"])
				_ = json.NewEncoder(w).Encode(map[string]any{"managed": true, "ok": true, "state": "done", "email": tc.email, "job_id": tc.id})
			}))
			defer server.Close()
			client := &OkadCookieRefreshClient{url: server.URL, apiKey: "local-key", client: server.Client(), timeout: time.Second}
			err := client.RefreshManaged(context.Background(), &Account{Credentials: map[string]any{"email": "one@test.example"}}, "request-1")
			if tc.code == "" {
				require.NoError(t, err)
			} else {
				var typed *OkadRecoveryError
				require.ErrorAs(t, err, &typed)
				require.Equal(t, tc.code, typed.Code)
			}
		})
	}
}
