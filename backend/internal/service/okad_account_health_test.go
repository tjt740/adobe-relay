package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOkadAccountHealthProjection(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		message string
		code    int
	}{
		{"Authentication failed (401): secret-cookie", 401},
		{"OAuth 401 (no refresh_token): secret-token", 401},
		{"Token refresh failed: adobe cookie is no longer valid", 401},
		{`{"status_code":408,"message":"private"}`, 408},
		{"HTTP 403: private", 403},
		{"upstream status code: 503 private", 503},
		{"an unclassified private error", 0},
	} {
		h := ProjectOkadAccountHealth(OkadAccountHealth{}, tc.message, nil, nil, nil, now)
		require.Equal(t, "error", h.State)
		require.Equal(t, tc.code, h.Errors[0].Code)
		raw, err := json.Marshal(h)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "private")
		require.NotContains(t, string(raw), "secret")
	}
	for _, message := range []string{"", OkadVerificationPending, OkadConnectionTestPending} {
		h := ProjectOkadAccountHealth(OkadAccountHealth{}, message, nil, nil, nil, now)
		require.Equal(t, "unknown", h.State)
	}
	for _, state := range []string{"pending", "running"} {
		h := ProjectOkadAccountHealth(OkadAccountHealth{LastSuccessAt: &now}, "", map[string]any{"state": state}, nil, nil, now)
		require.Equal(t, "unknown", h.State, "old success must not bless a new Cookie")
	}
	h := ProjectOkadAccountHealth(OkadAccountHealth{LastSuccessAt: &now}, "", map[string]any{"last_check_success": false, "last_check_status_code": float64(408)}, nil, nil, now)
	require.Equal(t, "recovered", h.State, "a stale test flag must not override successful evidence")
	h = ProjectOkadAccountHealth(OkadAccountHealth{}, "", map[string]any{"last_check_success": false, "last_check_status_code": float64(408)}, nil, nil, now)
	require.Equal(t, 408, h.Errors[0].Code)
	future, past := now.Add(time.Minute), now.Add(-time.Minute)
	h = ProjectOkadAccountHealth(OkadAccountHealth{LastSuccessAt: &now, Errors: []OkadAccountFailure{{Code: 408}}}, "HTTP 401 private", nil, &future, &past, now)
	require.Equal(t, "error", h.State)
	require.Len(t, h.Errors, 3)
	require.Equal(t, 429, h.Errors[2].Code)
}
