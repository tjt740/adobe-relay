//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

func TestAccountConnectionTestStatusCode(t *testing.T) {
	for _, tc := range []struct {
		message string
		code    int
	}{
		{"", 200}, {"Adobe credentials rejected (401) — expired", 401},
		{"API returned 403: forbidden", 403}, {"HTTP error! status: 408", 408},
		{"Grok Responses API returned 429: busy", 429}, {"status code=502", 502},
		{"Adobe generation timed out after 1m30s", 408}, {"context deadline exceeded", 408},
		{"connection refused", 0}, {"account 401 has no cookie", 0},
	} {
		t.Run(tc.message, func(t *testing.T) {
			var err error
			if tc.message != "" {
				err = errors.New(tc.message)
			}
			require.Equal(t, tc.code, connectionTestStatusCode(err))
		})
	}
}

func TestAccountConnectionTestAdobeEndpointPreservesCookieOnToken401(t *testing.T) {
	account := adobeTestConnAccount(map[string]any{"cookie": "fixture-cookie", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
	repo := &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{account.ID: account}}}
	api := &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) {
		return adobeJSONResponse(t, 401, map[string]any{"error": "expired"}, nil), nil
	}}
	withStubbedAdobeTestClient(t, adobe.NewClient(adobe.ClientConfig{Transport: api}))
	response, err := runAdobeTestConnWithRepo(t, repo, account, "", "")
	require.Error(t, err)
	require.Contains(t, response.Body.String(), `"status_code":401`)
	require.Equal(t, []int{401}, repo.codes)
	require.Equal(t, []bool{false}, repo.successes)
	require.Empty(t, account.ErrorMessage)
	require.Empty(t, repo.attempts)
}

type connectionTestResultRepo struct {
	*okadRecoveryRepo
	codes     []int
	successes []bool
}

func (r *connectionTestResultRepo) RecordConnectionTestResult(_ context.Context, _ int64, success bool, code int) error {
	r.codes = append(r.codes, code)
	r.successes = append(r.successes, success)
	return nil
}

func TestAccountConnectionTestRecordsStatusWithoutInferringCookieFailure(t *testing.T) {
	for _, code := range []int{200, 401, 403, 408} {
		a := recoveryFixtureAccount(int64(code))
		a.Status, a.ErrorMessage, a.Schedulable = StatusActive, "", true
		repo := &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}
		svc := &AccountTestService{accountRepo: repo}
		var testErr error
		if code != 200 {
			testErr = errors.New(map[int]string{401: "Adobe credentials rejected (401)", 403: "API returned 403", 408: "API returned 408"}[code])
		}
		svc.recordConnectionTestResult(context.Background(), a, testErr)
		require.Equal(t, []int{code}, repo.codes)
		require.Equal(t, []bool{code == 200}, repo.successes)
		require.Equal(t, StatusActive, a.Status)
		require.Empty(t, a.ErrorMessage)
		require.Empty(t, repo.attempts, "testing must not reserve extra Okad login attempts")
	}
}

func TestAccountConnectionTestCancellationAndConcurrentPause(t *testing.T) {
	a := recoveryFixtureAccount(501)
	a.Status, a.ErrorMessage, a.Schedulable = StatusActive, "", true
	repo := &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}
	svc := &AccountTestService{accountRepo: repo}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.recordConnectionTestResult(ctx, a, errors.New("context canceled"))
	require.Empty(t, repo.codes)
	snapshot, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	a.Schedulable = false
	svc.recordConnectionTestResult(context.Background(), snapshot, errors.New("Adobe credentials rejected (401)"))
	require.Equal(t, StatusActive, a.Status)
	require.False(t, a.Schedulable)
	require.Empty(t, a.ErrorMessage)
}
