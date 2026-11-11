package proxyfailover

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestHTTPSProbeSeparatesRouteFailuresFromAdobeDenials(t *testing.T) {
	for _, tt := range []struct {
		name string
		p, s httpsProbeObservation
		want string
	}{
		{"API success without fallback", httpsProbeObservation{Code: 200}, httpsProbeObservation{}, "healthy"},
		{"unauthenticated API reachable", httpsProbeObservation{Code: 401}, httpsProbeObservation{}, "healthy"},
		{"403 is not credential expiry or dead IP", httpsProbeObservation{Code: 403}, httpsProbeObservation{Code: 200}, "unknown"},
		{"upstream unavailable", httpsProbeObservation{Code: 502}, httpsProbeObservation{Code: 200}, "unknown"},
		{"single target blocked", httpsProbeObservation{Failure: "tls_failed"}, httpsProbeObservation{Code: 200}, "unknown"},
		{"both fail TLS", httpsProbeObservation{Failure: "tls_failed"}, httpsProbeObservation{Failure: "tls_failed"}, "unhealthy"},
		{"timeout both", httpsProbeObservation{Failure: "timeout"}, httpsProbeObservation{Failure: "connection_failed"}, "unhealthy"},
		{"canceled is not route failure", httpsProbeObservation{Failure: "canceled"}, httpsProbeObservation{Failure: "canceled"}, "unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, combineHTTPSProbes(tt.p, tt.s).Status) })
	}
	require.Equal(t, "tls_failed", probeFailureClass(errors.New("tls failed: http://user:private@proxy:123")))
	require.Equal(t, "timeout", probeFailureClass(context.DeadlineExceeded))
}

func TestHTTPSProbeRejectsConnect200WithoutTLS(t *testing.T) {
	var calls atomic.Int64
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "CONNECT", r.Method)
		c, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		defer c.Close()
		_, _ = c.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\nHTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
	}))
	defer proxyServer.Close()
	u, _ := url.Parse(proxyServer.URL)
	port, _ := strconv.Atoi(u.Port())
	h := probeAdobe(context.Background(), proxy{Status: "active", Protocol: "http", Host: u.Hostname(), Port: port})
	require.Equal(t, "unhealthy", h.Status)
	require.EqualValues(t, 2, calls.Load())
	require.Contains(t, h.Message, "https_transport_failed")
}
