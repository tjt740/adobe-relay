//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type okadCheckRepo struct {
	*connectionTestResultRepo
	queued   []string
	finished []string
	jobs     []OkadConnectionTestJob
}

func (r *okadCheckRepo) QueueOkadConnectionTest(_ context.Context, _ *Account, key string, verified bool) error {
	r.queued = append(r.queued, key)
	return nil
}
func (r *okadCheckRepo) ClaimOkadConnectionTest(context.Context) (*OkadConnectionTestJob, error) {
	return nil, nil
}
func (r *okadCheckRepo) FinishOkadConnectionTest(_ context.Context, job OkadConnectionTestJob, state string) error {
	r.finished = append(r.finished, state)
	r.jobs = append(r.jobs, job)
	return nil
}

func TestOkadManualCompletionCarriesActualStart(t *testing.T) {
	a := adobeTestConnAccount(map[string]any{"cookie": "fixture"})
	a.Extra = map[string]any{"okad_connection_test": map[string]any{"gated": true, "key": OkadConnectionTestKey(a)}}
	r := &okadCheckRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}}
	started := time.Now().Add(-time.Minute)
	ctx := context.WithValue(context.Background(), accountTestStartedAtKey{}, started)
	(&AccountTestService{accountRepo: r}).recordConnectionTestResult(ctx, a, adobe.NewUpstreamTemporaryError("busy", 408, adobe.ErrorTypeStatus))
	require.Len(t, r.jobs, 1)
	require.True(t, r.jobs[0].Manual)
	require.Equal(t, started, r.jobs[0].StartedAt)
	require.Equal(t, []string{"failed"}, r.finished)
}

func TestOkadAutomaticConnectionTestUsesAccountTestAndRecordsFailure(t *testing.T) {
	a := adobeTestConnAccount(map[string]any{"cookie": "fixture-cookie", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
	repo := &okadCheckRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}}
	calls := 0
	withStubbedAdobeTestClient(t, adobe.NewClient(adobe.ClientConfig{Transport: &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) {
		calls++
		return adobeJSONResponse(t, 401, map[string]any{"error": "expired"}, nil), nil
	}}}))
	svc := &AccountTestService{accountRepo: repo}
	svc.runOkadConnectionTest(context.Background(), repo, OkadConnectionTestJob{AccountID: a.ID, Key: OkadConnectionTestKey(a)})
	require.Equal(t, 1, calls)
	require.Equal(t, []int{401}, repo.codes)
	require.Equal(t, []int{401}, repo.alertCodes)
	require.Equal(t, []string{"failed"}, repo.finished)
	require.Empty(t, repo.attempts, "automatic detection cannot spend a login attempt")
	// A queued snapshot replaced by another Cookie makes no upstream request.
	svc.runOkadConnectionTest(context.Background(), repo, OkadConnectionTestJob{AccountID: a.ID, Key: "outdated-cookie"})
	require.Equal(t, 1, calls)
	require.Len(t, repo.codes, 1)
}

func TestOkadAutomaticConnectionTestRecordsSuccessfulGeneration(t *testing.T) {
	a := adobeTestConnAccount(map[string]any{"cookie": "fixture-cookie", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
	repo := &okadCheckRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}}
	api := &adobeFakeTransport{}
	withStubbedAdobeTestClient(t, adobeSubmitPollDownload(t, api, []byte("PNGDATA")))
	svc := &AccountTestService{accountRepo: repo}
	svc.runOkadConnectionTest(context.Background(), repo, OkadConnectionTestJob{AccountID: a.ID, Key: OkadConnectionTestKey(a)})
	require.Equal(t, []int{200}, repo.codes)
	require.Equal(t, []bool{true}, repo.successes)
	require.Equal(t, []string{"success"}, repo.finished)
	bodies := adobeSubmitBodies(t, api)
	require.Len(t, bodies, 1)
	require.Equal(t, "gpt-image-2.5-flare", bodies[0]["modelVersion"])
	require.Empty(t, repo.alertCodes)
	require.Empty(t, repo.attempts)
}

func TestOkadNativeCreateQueuesOnlyMarkedAdobeCookies(t *testing.T) {
	for _, tc := range []struct {
		name, platform, cookie  string
		marked, pending, queued bool
	}{
		{"legacy push", PlatformAdobe, "new-cookie", true, false, true},
		{"reliable initial", PlatformAdobe, "new-cookie", true, true, true},
		{"manual create", PlatformAdobe, "new-cookie", false, false, false},
		{"token only", PlatformAdobe, "", true, false, false},
		{"other platform", PlatformOpenAI, "cookie", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			extra := map[string]any{}
			if tc.marked {
				extra["adobeteam_external_email"] = "one@example.test"
			}
			if tc.pending {
				extra["okad_initial_verification"] = true
			}
			a, err := buildAccountForCreate(&CreateAccountInput{Platform: tc.platform, Type: AccountTypeOAuth, Credentials: map[string]any{"cookie": tc.cookie}}, extra)
			require.NoError(t, err)
			state, ok := a.Extra["okad_connection_test"].(map[string]any)
			require.Equal(t, tc.queued, ok)
			if ok {
				require.Equal(t, "pending", state["state"])
				require.Equal(t, tc.pending, state["require_verification"])
				require.Equal(t, OkadConnectionTestKey(a), state["key"])
			}
		})
	}
}

func TestOkadAutomaticTestDeadlineRecords408ButShutdownDoesNot(t *testing.T) {
	a := adobeTestConnAccount(nil)
	repo := &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}
	svc := &AccountTestService{accountRepo: repo}
	base := context.WithValue(context.Background(), okadConnectionTestContextKey{}, OkadConnectionTestJob{AccountID: a.ID})
	deadline, stop := context.WithDeadline(base, time.Now().Add(-time.Second))
	defer stop()
	svc.recordConnectionTestResult(deadline, a, context.DeadlineExceeded)
	require.Equal(t, []int{408}, repo.codes)
	require.Equal(t, []int{408}, repo.alertCodes)
	canceled, cancel := context.WithCancel(base)
	cancel()
	svc.recordConnectionTestResult(canceled, a, context.Canceled)
	require.Len(t, repo.codes, 1)
}
