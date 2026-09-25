// Package proxyfailover monitors Adobe connectivity without sending credentials or
// submitting generation jobs. It only changes the route for subsequent requests.
package proxyfailover

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const probeURL = "https://firefly-3p.ff.adobe.io/v2/3p-images/generate-async"

func probeAdobe(ctx context.Context, p proxy) health {
	result := health{Status: "unhealthy", Message: "connection_failed"}
	if !p.available(time.Now()) {
		result.Message = "proxy_unavailable"
		return result
	}
	u := &url.URL{Scheme: p.Protocol, Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port))}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	// A proxy is mandatory; never fall back to environment proxies or direct access.
	if p.Host == "" || p.Port <= 0 {
		return result
	}
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(),
		tlsclient.WithClientProfile(profiles.MappedTLSClients[adobe.DefaultIdentity.TLSProfile]),
		tlsclient.WithTimeoutSeconds(6), tlsclient.WithProxyUrl(u.String()),
		tlsclient.WithDisableHttp3(), tlsclient.WithNotFollowRedirects(), tlsclient.WithCatchPanics())
	if err != nil {
		return result
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := fhttp.NewRequestWithContext(ctx, "HEAD", probeURL, nil)
	if err != nil {
		return result
	}
	req.Header.Set("User-Agent", adobe.DefaultIdentity.UserAgent)
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return result
	}
	defer resp.Body.Close()
	result.LatencyMS = time.Since(start).Milliseconds()
	result.Status, result.Message = classifyStatus(resp.StatusCode)
	return result
}

func classifyStatus(code int) (string, string) {
	switch {
	case code >= 200 && code < 300, code == 401, code == 404, code == 405:
		return "healthy", "adobe_reachable"
	case code == 403, code == 407:
		return "unhealthy", "access_denied"
	default:
		// Rate limits and server outages are not evidence that an individual IP is dead.
		return "unknown", "upstream_unavailable"
	}
}
