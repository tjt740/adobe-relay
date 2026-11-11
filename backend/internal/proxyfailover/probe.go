// Package proxyfailover monitors Adobe connectivity without sending credentials or
// submitting generation jobs. It only changes the route for subsequent requests.
package proxyfailover

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	fhttp "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const probeURL = "https://firefly-3p.ff.adobe.io/v2/3p-images/generate-async"
const probeFallbackURL = "https://firefly.adobe.com/"

type httpsProbeObservation struct {
	Code      int
	Failure   string
	ElapsedMS int64
}

func probeAdobe(ctx context.Context, p proxy) health {
	result := health{Status: "unhealthy", Message: "connection_failed"}
	if !p.networkAvailable(time.Now()) {
		result.Message = "proxy_unavailable"
		return result
	}
	if p.Host == "" || p.Port <= 0 {
		return result
	}
	u := &url.URL{Scheme: p.Protocol, Host: net.JoinHostPort(p.Host, strconv.Itoa(p.Port))}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	// A proxy is mandatory. Certificate verification and no-direct-fallback remain enabled.
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), tlsclient.WithClientProfile(profiles.MappedTLSClients[adobe.DefaultIdentity.TLSProfile]), tlsclient.WithTimeoutSeconds(6), tlsclient.WithProxyUrl(u.String()), tlsclient.WithDisableHttp3(), tlsclient.WithNotFollowRedirects(), tlsclient.WithCatchPanics())
	if err != nil {
		return result
	}
	defer client.CloseIdleConnections()
	check := func(target string) httpsProbeObservation {
		one, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		req, err := fhttp.NewRequestWithContext(one, "HEAD", target, nil)
		if err != nil {
			return httpsProbeObservation{Failure: "request_failed"}
		}
		req.Header.Set("User-Agent", adobe.DefaultIdentity.UserAgent)
		start := time.Now()
		resp, err := client.Do(req)
		o := httpsProbeObservation{ElapsedMS: time.Since(start).Milliseconds()}
		if err != nil || resp == nil {
			o.Failure = probeFailureClass(err)
			return o
		}
		defer resp.Body.Close()
		o.Code = resp.StatusCode
		return o
	}
	primary := check(probeURL)
	// A completed, accepted API HTTPS exchange already proves TLS + HTTP reachability.
	// Use a second Adobe host only on failure: no extra requests on healthy routes.
	if status, _ := classifyStatus(primary.Code); primary.Failure == "" && status == "healthy" {
		return combineHTTPSProbes(primary, httpsProbeObservation{})
	}
	if ctx.Err() != nil {
		return health{Status: "unknown", Message: "probe_canceled"}
	}
	return combineHTTPSProbes(primary, check(probeFallbackURL))
}

func probeFailureClass(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return "timeout"
	}
	var cert x509.UnknownAuthorityError
	if errors.As(err, &cert) {
		return "tls_certificate"
	}
	// Persist a fixed category only; raw proxy errors may contain userinfo or credentials.
	if err != nil {
		s := strings.ToLower(err.Error())
		if strings.Contains(s, "tls") || strings.Contains(s, "ssl") || strings.Contains(s, "certificate") {
			return "tls_failed"
		}
		if strings.Contains(s, "407") {
			return "proxy_authentication"
		}
	}
	return "connection_failed"
}
func combineHTTPSProbes(primary, secondary httpsProbeObservation) health {
	label := func(o httpsProbeObservation) string {
		if o.Failure != "" {
			return o.Failure
		}
		return strconv.Itoa(o.Code)
	}
	result := health{Status: "unknown", LatencyMS: primary.ElapsedMS, Message: fmt.Sprintf("https_api=%s;https_web=%s", label(primary), label(secondary))}
	if primary.Failure == "" {
		result.Status, _ = classifyStatus(primary.Code)
		if primary.Code == 403 {
			result.Message = "adobe_access_denied;" + result.Message
		}
		return result
	}
	// Only independent transport failures justify treating the entire route as down.
	if primary.Failure != "canceled" && secondary.Failure != "" && secondary.Failure != "canceled" {
		result.Status = "unhealthy"
		result.Message = "https_transport_failed;" + result.Message
	}
	return result
}
func classifyStatus(code int) (string, string) {
	switch {
	case code >= 200 && code < 300, code == 401, code == 404, code == 405:
		return "healthy", "adobe_reachable"
	case code == 407:
		return "unhealthy", "proxy_authentication"
	case code == 403:
		return "unknown", "adobe_access_denied"
	default:
		return "unknown", "upstream_unavailable"
	}
}
