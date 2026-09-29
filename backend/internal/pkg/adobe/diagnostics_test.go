//go:build unit

package adobe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestImageDownloadRecoversWithoutResubmitting(t *testing.T) {
	for _, status := range []int{0, 408, 429, 502, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			url := "https://cdn/image.png?signature=private"
			client, api, download := pollClient(t, "https://firefly-3p.ff.adobe.io/jobs/job-1", func(int) (*Response, error) {
				return completedWith(t, url), nil
			})
			download.handler = func(req *Request, index int) (*Response, error) {
				require.Equal(t, url, req.URL)
				require.Empty(t, req.Headers["authorization"])
				require.Equal(t, MaxImageDownloadBytes, req.MaxBodyBytes)
				if index == 0 {
					if status == 0 {
						return nil, NewUpstreamTemporaryError("connection reset", 0, ErrorTypeConnection)
					}
					return bytesResponse(status, nil), nil
				}
				return bytesResponse(200, []byte("RECOVERED")), nil
			}
			out, err := generateTestImage(client, t)
			require.NoError(t, err)
			require.Equal(t, []byte("RECOVERED"), out.Bytes)
			require.Len(t, api.calls, 2, "one submit, one poll; do not generate a second image")
			require.Len(t, download.calls, 2)
		})
	}
}

func TestImageDownloadStopsAtRetryBudget(t *testing.T) {
	client, api, download := pollClient(t, "https://firefly-3p.ff.adobe.io/jobs/job-2", func(int) (*Response, error) {
		return completedWith(t, "https://cdn/image.png"), nil
	})
	download.handler = func(*Request, int) (*Response, error) {
		return &Response{StatusCode: 503, Headers: map[string]string{"x-request-id": "cdn-req"}}, nil
	}
	_, err := generateTestImage(client, t)
	var op *OperationError
	require.ErrorAs(t, err, &op)
	require.Equal(t, "download", op.Stage)
	require.Equal(t, "job-2", op.JobID)
	require.Equal(t, "cdn-req", op.UpstreamRequestID)
	require.Equal(t, 3, op.Attempts)
	var temporary *UpstreamTemporaryError
	require.ErrorAs(t, err, &temporary)
	require.Equal(t, 503, temporary.StatusCode)
	require.Len(t, api.calls, 2)
	require.Len(t, download.calls, 3)
}

func TestImageDownloadDoesNotRetryPermanentFailures(t *testing.T) {
	for _, status := range []int{400, 403, 404, 413, 451} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			download := &fakeTransport{handler: func(*Request, int) (*Response, error) { return bytesResponse(status, nil), nil }}
			_, err := testClient(nil, download).download(context.Background(), "https://cdn/image.png", 64, time.Second)
			require.Error(t, err)
			require.Len(t, download.calls, 1)
		})
	}
	download := &fakeTransport{handler: func(*Request, int) (*Response, error) { return nil, NewRequestError("response body too large") }}
	_, err := testClient(nil, download).download(context.Background(), "https://cdn/image.png", 64, time.Second)
	require.ErrorContains(t, err, "too large")
	require.Len(t, download.calls, 1)
}

func TestImageDownloadCancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	download := &fakeTransport{handler: func(*Request, int) (*Response, error) {
		cancel()
		return nil, NewUpstreamTemporaryError("timeout", 0, ErrorTypeTimeout)
	}}
	_, err := testClient(nil, download).download(ctx, "https://cdn/image.png", 64, time.Second)
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, download.calls, 1)
	require.ErrorIs(t, waitRetry(ctx, time.Hour, 1), context.Canceled)
}

func TestSubmitFailurePreservesActualStatusAndRequestID(t *testing.T) {
	api := &fakeTransport{handler: func(*Request, int) (*Response, error) {
		return jsonResponse(t, 400, map[string]string{"error_code": "invalid_size"}, map[string]string{"x-request-id": "adobe-req-123"}), nil
	}}
	_, err := generateTestImage(testClient(api, nil), t)
	var op *OperationError
	require.ErrorAs(t, err, &op)
	require.Equal(t, "submit", op.Stage)
	require.Equal(t, "adobe-req-123", op.UpstreamRequestID)
	var request *RequestError
	require.ErrorAs(t, err, &request)
	require.Equal(t, 400, request.StatusCode)
	require.Contains(t, err.Error(), "invalid_size")
}

func TestDiagnosticRedaction(t *testing.T) {
	message := `Get "https://user:password@cdn.example/image.png?X-Amz-Signature=secret": EOF; Authorization: Bearer abc.def; {"access_token":"private-token","cookie":"a=private-cookie; b=secret"} eyJabc.def.ghi`
	safe := SafeDiagnosticMessage(message)
	for _, secret := range []string{"user:password", "X-Amz", "private-token", "private-cookie", "b=secret", "eyJabc.def.ghi", "abc.def"} {
		require.NotContains(t, safe, secret)
	}
	require.Contains(t, safe, "EOF")
	require.NotContains(t, SafeDiagnosticMessage(`EOF Cookie: first=private; second=also-private`), "private")
	require.NotContains(t, SafeDiagnosticMessage(`HTTP 400 {"cookie":"first=private; second=truncated`), "private")
	require.NotContains(t, SafeDiagnosticMessage("upstream echoed sk-123456789test"), "123456789test")
	require.LessOrEqual(t, len(SafeDiagnosticMessage(strings.Repeat("x", 5000))), 2051)
	wrapped := operationError(errors.New("failure"), "poll", time.Now(), 2, nil, "https://firefly.adobe.io/jobs/job-1?token=secret")
	var op *OperationError
	require.ErrorAs(t, wrapped, &op)
	require.Equal(t, "job-1", op.JobID)
	require.Empty(t, diagnosticJobID("https://other.example/jobs/private"))
}
