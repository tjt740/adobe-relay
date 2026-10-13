package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type okadFailingProbe struct {
	fail  string
	calls []string
}

func (f *okadFailingProbe) call(stage string) error {
	f.calls = append(f.calls, stage)
	if f.fail == stage {
		return adobe.NewAuthError("upstream contained Cookie: secret-session; token=private-token", 401)
	}
	return nil
}
func (f *okadFailingProbe) RefreshAccessTokenFromCookie(context.Context, any, adobe.RefreshOptions) (*adobe.RefreshResult, error) {
	return &adobe.RefreshResult{AccessToken: "private-token"}, f.call("token")
}
func (f *okadFailingProbe) FetchAccountInfo(context.Context, string) (*adobe.AccountInfo, error) {
	return &adobe.AccountInfo{Email: "one@example.test"}, f.call("identity")
}
func (f *okadFailingProbe) FetchCreditsBalance(context.Context, string) (*adobe.CreditsBalance, error) {
	return &adobe.CreditsBalance{}, f.call("credits")
}

func TestOkadVerificationSeparatesTokenIdentityAndCreditsFailures(t *testing.T) {
	for i, stage := range []string{"token", "identity", "credits"} {
		t.Run(stage, func(t *testing.T) {
			f := &okadFailingProbe{fail: stage}
			a := &Account{Credentials: map[string]any{"email": "one@example.test", "cookie": "session"}}
			fields, balance, err := probeOkadCredential(context.Background(), a, f)
			require.Error(t, err)
			require.Nil(t, fields)
			require.Nil(t, balance)
			require.Len(t, f.calls, i+1, "a rejected stage must not continue to later probes")
			require.Equal(t, stage, okadVerificationStage(err))
			var auth *adobe.AuthError
			require.True(t, errors.As(err, &auth), "stage wrapping must preserve auth classification")
			require.Equal(t, 401, auth.StatusCode)
			message := okadAuthVerificationMessage(err)
			require.Contains(t, message, "Authentication failed (401):")
			require.Contains(t, message, stage)
			require.NotContains(t, message, "secret-session")
			require.NotContains(t, message, "private-token")
		})
	}
}
