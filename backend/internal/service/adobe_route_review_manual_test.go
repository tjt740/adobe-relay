//go:build unit

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type fixedRouteReviewRepo struct {
	*routeReviewRepo
	acquired int
}

func (r *fixedRouteReviewRepo) AcquireAdobeProxyForTest(_ context.Context, a *Account, _ string) (*Account, *AdobeProxyTestLease, error) {
	r.acquired++
	return a, nil, nil
}
func (r *fixedRouteReviewRepo) AdobeTestRoute(context.Context, *Account) (string, bool, error) {
	return "manual-source", true, nil
}
func TestAdobeManualRouteNeverStartsCrossSourceReview(t *testing.T) {
	for _, code := range []int{408, 429} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			a := adobeTestConnAccount(map[string]any{"cookie": "fixed-cookie", "access_token": adobeTestJWT(t, time.Now().Add(time.Hour))})
			pid := int64(9)
			a.ProxyID = &pid
			a.Proxy = &Proxy{ID: pid, Protocol: "http", Host: "fixture", Port: 8080, Status: StatusActive}
			a.Extra = map[string]any{"manual_proxy_enabled": true, "okad_manual_pause": true}
			r := &fixedRouteReviewRepo{routeReviewRepo: &routeReviewRepo{leasedCheckRepo: newLeasedCheckRepo(a), t: t}}
			submits := 0
			transport := &adobeFakeTransport{handler: func(req *adobe.Request, _ int) (*adobe.Response, error) {
				submits++
				return &adobe.Response{StatusCode: code, Body: []byte(`{"error_code":"timeout_error"}`)}, nil
			}}
			withStubbedAdobeTestClient(t, adobe.NewClient(adobe.ClientConfig{Transport: transport, DownloadTransport: transport}))
			rec, err := runAdobeTestConnWithRepo(t, r, a, "gpt-image-2", "")
			require.Error(t, err)
			require.Equal(t, 1, submits)
			require.Equal(t, 1, r.acquired)
			require.Equal(t, &pid, a.ProxyID)
			require.Equal(t, true, a.Extra["okad_manual_pause"])
			require.NotContains(t, rec.Body.String(), "更换不同 Clash")
		})
	}
}
