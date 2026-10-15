//go:build unit

package adobe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestImageDownloadFallbackKeepsSameJobAndURL(t *testing.T) {
	for _, status := range []int{0, 408, 429, 500, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			mediaURL := "https://cdn/image.png?X-Amz-Signature=original%2Fsignature"
			client, api, primary := pollClient(t, "https://firefly-3p.ff.adobe.io/jobs/original-job", func(int) (*Response, error) {
				return completedWith(t, mediaURL), nil
			})
			primary.handler = func(*Request, int) (*Response, error) {
				if status == 0 {
					return nil, NewUpstreamTemporaryError("tls: first record does not look like a TLS handshake", 0, ErrorTypeNetwork)
				}
				return bytesResponse(status, nil), nil
			}
			fallback := &fakeTransport{handler: func(req *Request, _ int) (*Response, error) {
				require.Equal(t, mediaURL, req.URL)
				require.Equal(t, http.MethodGet, req.Method)
				require.Equal(t, map[string]string{"accept": "*/*"}, req.Headers)
				require.Empty(t, req.Body)
				require.Equal(t, MaxImageDownloadBytes, req.MaxBodyBytes)
				require.Equal(t, defaultSubmitTimeout, req.Timeout)
				return bytesResponse(200, []byte("same image")), nil
			}}
			client.imageDownloadFallback = fallback
			result, err := generateTestImage(client, t)
			require.NoError(t, err)
			require.Equal(t, []byte("same image"), result.Bytes)
			require.Equal(t, mediaURL, result.URL)
			require.Equal(t, 2, result.Timings.DownloadAttempts)
			require.True(t, result.Timings.DownloadFallback)
			require.Len(t, primary.calls, 1)
			require.Len(t, fallback.calls, 1)
			require.Len(t, api.calls, 2, "exactly one submission and one poll")
		})
	}
}

func TestImageDownloadFallbackBudgetAndTerminalErrors(t *testing.T) {
	for _, fallbackStatus := range []int{200, 403, 451, 503} {
		t.Run(fmt.Sprint(fallbackStatus), func(t *testing.T) {
			client, api, primary := pollClient(t, "https://firefly.adobe.io/jobs/job-1", func(int) (*Response, error) {
				return completedWith(t, "https://cdn/image.png"), nil
			})
			primary.handler = func(*Request, int) (*Response, error) { return bytesResponse(503, nil), nil }
			fallback := &fakeTransport{handler: func(*Request, int) (*Response, error) { return bytesResponse(fallbackStatus, []byte("image")), nil }}
			client.imageDownloadFallback = fallback
			_, err := generateTestImage(client, t)
			wantAttempts := 2
			if fallbackStatus == 503 {
				wantAttempts = 3
			}
			if fallbackStatus == 200 {
				require.NoError(t, err)
			} else {
				var op *OperationError
				require.ErrorAs(t, err, &op)
				require.Equal(t, "download", op.Stage)
				require.Equal(t, "job-1", op.JobID)
				require.Equal(t, wantAttempts, op.Attempts)
				require.True(t, op.DownloadFallback)
			}
			require.Len(t, primary.calls, 1)
			require.Len(t, fallback.calls, wantAttempts-1)
			require.Len(t, api.calls, 2, "failed downloads must not create another job")
		})
	}
}

func TestImageDownloadFallbackDoesNotRunAfterSuccessOrPermanentError(t *testing.T) {
	for _, status := range []int{200, 400, 401, 403, 404, 413, 451} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client, _, primary := pollClient(t, "https://firefly.adobe.io/jobs/job-1", func(int) (*Response, error) {
				return completedWith(t, "https://cdn/image.png"), nil
			})
			primary.handler = func(*Request, int) (*Response, error) { return bytesResponse(status, []byte("image")), nil }
			fallback := &fakeTransport{handler: func(*Request, int) (*Response, error) { t.Fatal("unexpected fallback"); return nil, nil }}
			client.imageDownloadFallback = fallback
			result, err := generateTestImage(client, t)
			if status == 200 {
				require.NoError(t, err)
				require.False(t, result.Timings.DownloadFallback)
			} else {
				require.Error(t, err)
			}
			require.Len(t, primary.calls, 1)
			require.Empty(t, fallback.calls)
		})
	}
}

func TestImageDownloadFallbackCancellationAndBlockedURLs(t *testing.T) {
	fallback := &fakeTransport{handler: func(*Request, int) (*Response, error) { t.Fatal("unexpected fallback"); return nil, nil }}
	for _, mediaURL := range []string{"http://cdn/image", "https://127.0.0.1/image", "https://localhost/image"} {
		primary := &fakeTransport{handler: func(*Request, int) (*Response, error) { t.Fatal("unsafe URL requested"); return nil, nil }}
		_, err := testClient(nil, primary).downloadMeasured(context.Background(), mediaURL, 64, time.Second, nil, fallback)
		require.Error(t, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	primary := &fakeTransport{handler: func(*Request, int) (*Response, error) {
		cancel()
		return nil, NewUpstreamTemporaryError("timeout", 0, ErrorTypeTimeout)
	}}
	_, err := testClient(nil, primary).downloadMeasured(ctx, "https://cdn/image", 64, time.Second, nil, fallback)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, fallback.calls)
}

func TestImageDownloadFallbackLeavesVideoUnchanged(t *testing.T) {
	api := &fakeTransport{handler: func(req *Request, _ int) (*Response, error) {
		if req.Method == http.MethodPost {
			return jsonResponse(t, 200, nil, map[string]string{"x-override-status-link": "https://firefly.adobe.io/jobs/video"}), nil
		}
		return jsonResponse(t, 200, map[string]any{"outputs": []any{map[string]any{"video": map[string]any{"presignedUrl": "https://cdn/video.mp4"}}}}, nil), nil
	}}
	primary := &fakeTransport{handler: func(*Request, int) (*Response, error) { return bytesResponse(503, nil), nil }}
	fallback := &fakeTransport{handler: func(*Request, int) (*Response, error) { t.Fatal("video used image fallback"); return nil, nil }}
	client := NewClient(ClientConfig{Transport: api, DownloadTransport: primary, ImageDownloadFallbackTransport: fallback})
	_, err := client.GenerateVideo(context.Background(), GenerateVideoInput{Token: "token", PollInterval: time.Millisecond})
	require.Error(t, err)
	require.Len(t, primary.calls, 3)
	require.Empty(t, fallback.calls)
}

func TestImageDownloadFallbackDefaultAndDirectGuards(t *testing.T) {
	client := NewClient(ClientConfig{ProxyURL: "http://proxy.example:8080"})
	fallback := client.imageDownloadFallback.(*plainTransport)
	require.True(t, fallback.direct)
	custom := &fakeTransport{}
	require.Nil(t, NewClient(ClientConfig{ProxyURL: "http://proxy.example:8080", DownloadTransport: custom}).imageDownloadFallback)
	// Even a global proxy configured by the environment must not intercept fallback.
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	httpClient, err := fallback.ensureClient()
	require.NoError(t, err)
	transport := httpClient.Transport.(*http.Transport)
	require.Nil(t, transport.Proxy)
	require.False(t, fallback.resolveGuard)
	_, err = transport.DialContext(context.Background(), "tcp", "127.0.0.1:1")
	require.ErrorIs(t, err, ErrBlockedDestination, "actual dial destination must remain protected")
	redirect, _ := http.NewRequest(http.MethodGet, "https://localhost/private", nil)
	require.ErrorIs(t, fallback.checkRedirect(redirect, nil), ErrBlockedDestination)
	redirect, _ = http.NewRequest(http.MethodGet, "https://cdn.example/image", nil)
	require.ErrorIs(t, fallback.checkRedirect(redirect, make([]*http.Request, maxDownloadRedirects)), ErrBlockedDestination)
}

func TestImageDownloadFallbackRecoversRealTLSFailure(t *testing.T) {
	var proxyCalls, originCalls atomic.Int32
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originCalls.Add(1)
		require.Equal(t, "/image.png?signature=unchanged%2Fvalue", r.RequestURI)
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		_, _ = w.Write([]byte("original image bytes"))
	}))
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		require.Equal(t, http.MethodConnect, r.Method)
		conn, _, err := w.(http.Hijacker).Hijack()
		require.NoError(t, err)
		defer conn.Close()
		// Reproduce a broken tunnel returning plaintext during the TLS handshake.
		_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\nHTTP/1.1 502 Bad Gateway\r\n\r\n"))
	}))
	defer proxy.Close()
	originalResolver := downloadResolver
	downloadResolver = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	defer func() { downloadResolver = originalResolver }()
	u, _ := url.Parse(origin.URL)
	mediaURL := "https://" + net.JoinHostPort(origin.Certificate().DNSNames[0], u.Port()) + "/image.png?signature=unchanged%2Fvalue"
	_, api, _ := pollClient(t, "https://firefly.adobe.io/jobs/one-job", func(int) (*Response, error) { return completedWith(t, mediaURL), nil })
	client := NewClient(ClientConfig{Transport: api, ProxyURL: proxy.URL})
	direct := client.imageDownloadFallback.(*plainTransport)
	httpClient, err := direct.ensureClient()
	require.NoError(t, err)
	transport := httpClient.Transport.(*http.Transport)
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	// Route only the test certificate's hostname to the local TLS fixture.
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, u.Host)
	}
	result, err := generateTestImage(client, t)
	require.NoError(t, err)
	require.Equal(t, []byte("original image bytes"), result.Bytes)
	require.True(t, result.Timings.DownloadFallback)
	require.Equal(t, int32(1), proxyCalls.Load())
	require.Equal(t, int32(1), originCalls.Load())
	require.Len(t, api.calls, 2)
	transport.CloseIdleConnections()
}

type downloadFallbackFunc func(context.Context, *Request) (*Response, error)

func (f downloadFallbackFunc) Do(ctx context.Context, req *Request) (*Response, error) {
	return f(ctx, req)
}

func TestImageDownloadFallbackConcurrentRequests(t *testing.T) {
	var primaryCalls, fallbackCalls atomic.Int32
	primary := downloadFallbackFunc(func(context.Context, *Request) (*Response, error) {
		primaryCalls.Add(1)
		return nil, NewUpstreamTemporaryError("timeout", 0, ErrorTypeTimeout)
	})
	fallback := downloadFallbackFunc(func(_ context.Context, r *Request) (*Response, error) {
		fallbackCalls.Add(1)
		return bytesResponse(200, []byte(r.URL)), nil
	})
	client := NewClient(ClientConfig{DownloadTransport: primary, ImageDownloadFallbackTransport: fallback})
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := fmt.Sprintf("https://cdn/image-%d?signature=unique", i)
			var timings GenerationTimings
			data, err := client.downloadMeasured(context.Background(), u, 1024, time.Second, &timings, client.imageDownloadFallback)
			if err != nil || string(data) != u || !timings.DownloadFallback || timings.DownloadAttempts != 2 {
				t.Errorf("request %d: incorrect isolated fallback result: %v", i, err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(24), primaryCalls.Load())
	require.Equal(t, int32(24), fallbackCalls.Load())
}

func TestImageDownloadFallbackDoesNotRetryOversizedBody(t *testing.T) {
	primary := &fakeTransport{handler: func(*Request, int) (*Response, error) { return nil, NewRequestError("response body too large") }}
	fallback := &fakeTransport{handler: func(*Request, int) (*Response, error) { return nil, errors.New("unexpected fallback") }}
	_, err := testClient(nil, primary).downloadMeasured(context.Background(), "https://cdn/image", 64, time.Second, nil, fallback)
	require.ErrorContains(t, err, "too large")
	require.Empty(t, fallback.calls)
}
