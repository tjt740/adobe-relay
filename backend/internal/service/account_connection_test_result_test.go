//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
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
	require.Equal(t, []int{401}, repo.alertCodes)
	require.Equal(t, []bool{false}, repo.successes)
	require.Empty(t, account.ErrorMessage)
	require.Empty(t, repo.attempts)
}

type connectionTestResultRepo struct {
	*okadRecoveryRepo
	codes         []int
	successes     []bool
	alertCodes    []int
	alertReasons  []string
	alertTimeouts []bool
}

func (r *connectionTestResultRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}

func (r *connectionTestResultRepo) RecordAdobeConnectionTestError(_ context.Context, _ int64, code int, reason string, timeout bool) error {
	r.alertCodes = append(r.alertCodes, code)
	r.alertReasons = append(r.alertReasons, reason)
	r.alertTimeouts = append(r.alertTimeouts, timeout)
	return nil
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
	svc.recordConnectionTestResult(context.Background(), a, fmt.Errorf("test stopped: %w", context.Canceled))
	require.Empty(t, repo.codes)
	require.Empty(t, repo.alertCodes)
	snapshot, err := repo.GetByID(context.Background(), a.ID)
	require.NoError(t, err)
	a.Schedulable = false
	svc.recordConnectionTestResult(context.Background(), snapshot, errors.New("Adobe credentials rejected (401)"))
	require.Equal(t, StatusActive, a.Status)
	require.False(t, a.Schedulable)
	require.Empty(t, a.ErrorMessage)
}

func TestAccountConnectionTestAdobeAlertSourcesAndClassification(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		code    int
		reason  string
		timeout bool
	}{
		{"success", nil, 0, "", false},
		{"auth", adobe.NewAuthError("rejected", 401), 401, "adobe_auth", false},
		{"quota", adobe.NewQuotaExhaustedError("quota", 401), 401, "adobe_quota_exhausted", false},
		{"entitlement", adobe.NewNotEntitledError("plan", 401, ""), 401, "adobe_not_entitled", false},
		{"upstream 408", &adobe.RequestError{Message: "overloaded", StatusCode: 408}, 408, "adobe_request", false},
		{"text 408", errors.New("API returned 408"), 408, "", false},
		{"generation timeout", errors.New("Adobe generation timed out after 1m30s"), 408, "", true},
		{"deadline", context.DeadlineExceeded, 408, "", true},
		{"unrelated 403", adobe.NewAuthError("forbidden", 403), 0, "", false},
		{"no response", errors.New("connection refused"), 0, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := recoveryFixtureAccount(1)
			repo := &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{1: a}}}
			svc := &AccountTestService{accountRepo: repo}
			svc.recordConnectionTestResult(context.Background(), a, tc.err)
			if tc.code == 0 {
				require.Empty(t, repo.alertCodes)
			} else {
				require.Equal(t, []int{tc.code}, repo.alertCodes)
				require.Equal(t, []string{tc.reason}, repo.alertReasons)
				require.Equal(t, []bool{tc.timeout}, repo.alertTimeouts)
			}
			require.Empty(t, repo.attempts, "alert observation cannot spend a login attempt")
			a.Platform = PlatformOpenAI
			repo.alertCodes = nil
			svc.recordConnectionTestResult(context.Background(), a, tc.err)
			require.Empty(t, repo.alertCodes, "exclude non-Adobe tests")
		})
	}
}

type successfulAdobeTestRepo struct {
	*connectionTestResultRepo
	recovered int
	started   time.Time
}

func (r *successfulAdobeTestRepo) RecoverAdobeAfterSuccessfulTestIfCurrent(_ context.Context, _ *Account, started time.Time) (bool, error) {
	r.recovered++
	r.started = started
	return true, nil
}
func TestAccountConnectionTestSharedAdobeRecovery(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		for _, success := range []bool{false, true} {
			a := recoveryFixtureAccount(1)
			repo := &successfulAdobeTestRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{1: a}}}}
			svc := &AccountTestService{accountRepo: repo}
			start := time.Now().Add(-time.Second)
			ctx := context.WithValue(context.Background(), accountTestStartedAtKey{}, start)
			if automatic {
				ctx = context.WithValue(ctx, okadConnectionTestContextKey{}, OkadConnectionTestJob{AccountID: 1})
			}
			var err error
			if !success {
				err = errors.New("API returned 408")
			}
			svc.recordConnectionTestResult(ctx, a, err)
			if success {
				require.Equal(t, 1, repo.recovered)
				require.Equal(t, start, repo.started)
			} else {
				require.Zero(t, repo.recovered)
			}
		}
	}
}
