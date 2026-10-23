//go:build unit

package service

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/proxyfailover"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type routeReviewRepo struct {
	*leasedCheckRepo
	t            *testing.T
	routes       []string
	next         int
	observations []AdobeHealthObservation
}

func (r *routeReviewRepo) AcquireAdobeProxyForTest(ctx context.Context, a *Account, _ string) (*Account, *AdobeProxyTestLease, error) {
	opts := proxyfailover.TestRouteOptionsFromContext(ctx)
	require.Len(r.t, opts.ExcludeSources, r.next)
	if r.next > 0 {
		require.Equal(r.t, r.routes[:r.next], opts.ExcludeSources)
	}
	if r.next == len(r.routes) {
		return a, nil, proxyfailover.ErrNoTestProxy
	}
	pid := int64(100 + r.next)
	r.next++
	a.ProxyID = &pid
	a.Proxy = &Proxy{ID: pid, Host: "fixture", Port: 8080, Protocol: "http", Status: StatusActive, UpdatedAt: time.Now()}
	return a, &AdobeProxyTestLease{AccountID: a.ID, Owner: fmt.Sprint(pid)}, nil
}
func (r *routeReviewRepo) AdobeTestRoute(_ context.Context, a *Account) (string, bool, error) {
	return r.routes[r.next-1], true, nil
}
func (r *routeReviewRepo) RecordAdobeHealth(_ context.Context, _ *Account, o AdobeHealthObservation) error {
	r.observations = append(r.observations, o)
	return nil
}
func (r *routeReviewRepo) FinishAdobeProxyTest(_ context.Context, l AdobeProxyTestLease, success, _ bool) error {
	r.events = append(r.events, "release")
	r.finishedLease = append(r.finishedLease, success)
	if !success {
		a := r.accounts[l.AccountID]
		a.Proxy = nil
		a.ProxyID = nil
	}
	return nil
}

func TestAdobeRouteReviewThreeSourcesAndFinalSSE(t *testing.T) {
	for _, tc := range []struct {
		name             string
		routes, failures int
		success          bool
	}{
		{"third source succeeds", 3, 2, true}, {"three genuine failures", 3, 3, false}, {"insufficient sources", 1, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adobeTestConnAccount(map[string]any{"cookie": "fixed-cookie", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
			a.Extra = map[string]any{"okad_manual_pause": true}
			r := &routeReviewRepo{leasedCheckRepo: newLeasedCheckRepo(a), t: t}
			for i := 0; i < tc.routes; i++ {
				r.routes = append(r.routes, fmt.Sprintf("%024d", i+1))
			}
			api := &adobeFakeTransport{}
			client := adobeSubmitPollDownload(t, api, []byte("PNG"))
			success := api.handler
			submits := 0
			api.handler = func(req *adobe.Request, n int) (*adobe.Response, error) {
				if req.URL == adobe.ImageSubmitURL {
					submits++
					if submits <= tc.failures {
						return &adobe.Response{StatusCode: 408, Body: []byte(`{"error_code":"timeout_error"}`)}, nil
					}
				}
				return success(req, n)
			}
			withStubbedAdobeTestClient(t, client)
			rec, err := runAdobeTestConnWithRepo(t, r, a, "gpt-image-2", "")
			require.Equal(t, tc.success, err == nil)
			require.Equal(t, tc.routes, submits, "no nested SDK retries or repeated source")
			require.Len(t, r.observations, tc.routes)
			for i, o := range r.observations {
				require.Equal(t, r.routes[i], o.SourceID)
			}
			require.True(t, a.Extra["okad_manual_pause"].(bool))
			require.Equal(t, "fixed-cookie", a.GetCredential("cookie"))
			if tc.success {
				require.NotContains(t, rec.Body.String(), `"type":"error"`)
				require.Contains(t, rec.Body.String(), `"success":true`)
			} else {
				require.Contains(t, rec.Body.String(), `"type":"error"`)
				if tc.routes < 3 {
					require.Contains(t, rec.Body.String(), "检测条件不足")
				}
			}
		})
	}
}

func TestAdobeRouteReviewNeverReplaysAcceptedOrUncertainSubmission(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response *adobe.Response
		err      error
	}{
		{"uncertain transport", nil, context.DeadlineExceeded},
		{"uncertain gateway", &adobe.Response{StatusCode: 502}, nil},
		{"accepted error", &adobe.Response{StatusCode: 408, Headers: map[string]string{"x-override-status-link": "https://firefly-3p.ff.adobe.io/jobs/accepted"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adobeTestConnAccount(map[string]any{"cookie": "fixture", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
			r := &routeReviewRepo{leasedCheckRepo: newLeasedCheckRepo(a), t: t, routes: []string{fmt.Sprintf("%024d", 1), fmt.Sprintf("%024d", 2)}}
			api := &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) { return tc.response, tc.err }}
			withStubbedAdobeTestClient(t, adobe.NewClient(adobe.ClientConfig{Transport: api}))
			_, err := runAdobeTestConnWithRepo(t, r, a, "gpt-image-2", "")
			require.Error(t, err)
			require.Equal(t, 1, r.next)
			require.Len(t, api.calls, 1)
		})
	}
}

func TestAdobeRouteReviewAutomaticJobCompletesBeforeFinalLeaseRelease(t *testing.T) {
	a := adobeTestConnAccount(map[string]any{"cookie": "fixture", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
	r := &routeReviewRepo{leasedCheckRepo: newLeasedCheckRepo(a), t: t, routes: []string{fmt.Sprintf("%024d", 1), fmt.Sprintf("%024d", 2)}}
	api := &adobeFakeTransport{}
	client := adobeSubmitPollDownload(t, api, []byte("PNG"))
	success := api.handler
	api.handler = func(req *adobe.Request, n int) (*adobe.Response, error) {
		if req.URL == adobe.ImageSubmitURL && n == 0 {
			return &adobe.Response{StatusCode: 408}, nil
		}
		return success(req, n)
	}
	withStubbedAdobeTestClient(t, client)
	svc := &AccountTestService{accountRepo: r}
	svc.runOkadConnectionTest(context.Background(), r, OkadConnectionTestJob{AccountID: a.ID, Key: OkadConnectionTestKey(a)})
	require.Equal(t, []string{"success"}, r.finished)
	require.Equal(t, []bool{false, true}, r.finishedLease)
	require.Equal(t, []string{"job_finish", "release"}, r.events[len(r.events)-2:])
	require.Empty(t, r.attempts, "review cannot reserve any extra Okad login")
}
