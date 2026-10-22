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

type leasedCheckRepo struct {
	*okadCheckRepo
	events        []string
	scopes        []string
	acquireErr    error
	finishedLease []bool
}

func (r *leasedCheckRepo) AcquireAdobeProxyForTest(_ context.Context, a *Account, scope string) (*Account, *AdobeProxyTestLease, error) {
	r.events = append(r.events, "acquire")
	r.scopes = append(r.scopes, scope)
	if r.acquireErr != nil {
		return a, nil, r.acquireErr
	}
	pid := int64(99)
	a.ProxyID = &pid
	a.Proxy = &Proxy{ID: pid, Status: StatusActive, UpdatedAt: time.Now()}
	a.Schedulable = false
	r.accounts[a.ID] = a
	return a, &AdobeProxyTestLease{AccountID: a.ID, Owner: "fixture-owner"}, nil
}
func (r *leasedCheckRepo) FinishAdobeProxyTest(ctx context.Context, _ AdobeProxyTestLease, success, _ bool) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.events = append(r.events, "release")
	r.finishedLease = append(r.finishedLease, success)
	return nil
}
func (r *leasedCheckRepo) RecordAdobeHealth(_ context.Context, a *Account, _ AdobeHealthObservation) error {
	if a.ProxyID == nil || *a.ProxyID != 99 {
		return errors.New("health must use the borrowed route")
	}
	r.events = append(r.events, "health")
	return nil
}
func (r *leasedCheckRepo) RecoverAdobeAfterSuccessfulTestIfCurrent(context.Context, *Account, time.Time) (bool, error) {
	r.events = append(r.events, "recovery")
	return false, nil
}
func (r *leasedCheckRepo) FinishOkadConnectionTest(ctx context.Context, j OkadConnectionTestJob, state string) error {
	r.events = append(r.events, "job_finish")
	return r.okadCheckRepo.FinishOkadConnectionTest(ctx, j, state)
}
func newLeasedCheckRepo(a *Account) *leasedCheckRepo {
	return &leasedCheckRepo{okadCheckRepo: &okadCheckRepo{connectionTestResultRepo: &connectionTestResultRepo{okadRecoveryRepo: &okadRecoveryRepo{accounts: map[int64]*Account{a.ID: a}}}}}
}

func TestAdobeProxyLeaseWrapsRealGenerationAndPersistsBeforePromotion(t *testing.T) {
	a := adobeTestConnAccount(map[string]any{"cookie": "fixture", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
	r := newLeasedCheckRepo(a)
	withStubbedAdobeTestClient(t, adobeSubmitPollDownload(t, &adobeFakeTransport{}, []byte("PNGDATA")))
	_, err := runAdobeTestConnWithRepo(t, r, a, "gpt-image-2.5-flare", "")
	require.NoError(t, err)
	require.Equal(t, []string{"acquire", "health", "recovery", "release"}, r.events)
	require.Equal(t, []string{"firefly-gpt-image-2-5-flare"}, r.scopes)
	require.Equal(t, []bool{true}, r.finishedLease)
	require.Equal(t, []int{200}, r.codes)
}

func TestAdobeProxyLeaseFailuresReleaseAndNoCapacityDoesNotSubmit(t *testing.T) {
	for _, tc := range []struct {
		name                string
		acquireErr, testErr error
	}{
		{"upstream failure", nil, adobe.NewUpstreamTemporaryError("busy", 408, adobe.ErrorTypeStatus)},
		{"canceled generation", nil, context.Canceled},
		{"no capacity", errors.New("暂无可用代理供检测"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adobeTestConnAccount(map[string]any{"cookie": "fixture", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
			r := newLeasedCheckRepo(a)
			r.acquireErr = tc.acquireErr
			calls := 0
			withStubbedAdobeTestClient(t, adobe.NewClient(adobe.ClientConfig{Transport: &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) { calls++; return nil, tc.testErr }}}))
			_, err := runAdobeTestConnWithRepo(t, r, a, "", "")
			require.Error(t, err)
			if tc.acquireErr != nil {
				require.Zero(t, calls)
				require.Empty(t, r.finishedLease)
				require.Empty(t, r.codes)
			} else {
				require.Positive(t, calls)
				require.Equal(t, []bool{false}, r.finishedLease)
				require.Equal(t, "release", r.events[len(r.events)-1])
			}
		})
	}
}

func TestAdobeProxyLeaseAutomaticWorkerBorrowsOnceAndFinishesJobBeforeRelease(t *testing.T) {
	a := adobeTestConnAccount(map[string]any{"cookie": "fixture", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
	r := newLeasedCheckRepo(a)
	withStubbedAdobeTestClient(t, adobeSubmitPollDownload(t, &adobeFakeTransport{}, []byte("PNGDATA")))
	s := &AccountTestService{accountRepo: r}
	s.runOkadConnectionTest(context.Background(), r, OkadConnectionTestJob{AccountID: a.ID, Key: OkadConnectionTestKey(a)})
	require.Len(t, r.scopes, 1, "the nested account test must reuse the worker's lease")
	require.Equal(t, []bool{true}, r.finishedLease)
	require.Equal(t, []string{"success"}, r.finished)
	require.Equal(t, []string{"acquire", "health", "recovery", "job_finish", "release"}, r.events)
}

func TestAdobeProxyLeaseCleanupSurvivesCanceledRequest(t *testing.T) {
	a := adobeTestConnAccount(nil)
	r := newLeasedCheckRepo(a)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	finishAdobeTestProxy(ctx, r, &AdobeProxyTestLease{AccountID: a.ID}, false, false)
	require.Equal(t, []bool{false}, r.finishedLease)
}

type credentialOnlyLeaseRepo struct {
	*okadProxyPendingRepo
	released bool
	promoted bool
}

func (r *credentialOnlyLeaseRepo) AcquireAdobeProxyForTest(_ context.Context, a *Account, _ string) (*Account, *AdobeProxyTestLease, error) {
	return a, &AdobeProxyTestLease{AccountID: a.ID}, nil
}
func (r *credentialOnlyLeaseRepo) FinishAdobeProxyTest(_ context.Context, _ AdobeProxyTestLease, success, _ bool) error {
	r.released = true
	r.promoted = success
	return nil
}
func TestAdobeProxyLeaseCredentialOnlyVerificationAlwaysReleases(t *testing.T) {
	a := &Account{ID: 1, Platform: PlatformAdobe, Type: AccountTypeOAuth, Extra: map[string]any{"okad": map[string]any{"version": 1}}}
	r := &credentialOnlyLeaseRepo{okadProxyPendingRepo: &okadProxyPendingRepo{a: a}}
	_, err := (&AccountUsageService{accountRepo: r}).VerifyOkadCredential(context.Background(), a)
	require.NoError(t, err)
	require.True(t, r.released)
	require.False(t, r.promoted)
}
