//go:build unit

package adobe

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdobeImagePollBackoffAndRetryAfter(t *testing.T) {
	old := pollRetryWait
	pollRetryWait = DefaultPollInterval
	defer func() { pollRetryWait = old }()
	params := pollParams{pollInterval: DefaultImagePollInterval}
	require.Equal(t, time.Second, params.pollInterval)
	require.Equal(t, 3*time.Second, pollRetryParams(params, 1, nil).pollInterval)
	require.Equal(t, 6*time.Second, pollRetryParams(params, 2, nil).pollInterval)
	require.Equal(t, 12*time.Second, pollRetryParams(params, 3, nil).pollInterval)
	require.Equal(t, 20*time.Second, pollRetryParams(params, 1, &Response{Headers: map[string]string{"retry-after": "20"}}).pollInterval)
	require.Equal(t, 3*time.Second, pollRetryParams(params, 1, &Response{Headers: map[string]string{"retry-after": "invalid"}}).pollInterval)
	require.Equal(t, 3*time.Second, pollRetryParams(params, 1, &Response{Headers: map[string]string{"retry-after": "-1"}}).pollInterval)
	date := time.Now().Add(time.Minute).UTC().Format(http.TimeFormat)
	require.Greater(t, pollRetryParams(params, 1, &Response{Headers: map[string]string{"retry-after": date}}).pollInterval, 58*time.Second)
	// A normal 202 can also ask clients to poll less often.
	require.Equal(t, 4*time.Second, pollRetryAfter(params, &Response{Headers: map[string]string{"retry-after": "4"}}).pollInterval)
}

func TestAdobeImageRetryAfterDoesNotExtendDeadline(t *testing.T) {
	for _, status := range []int{202, 429} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			api := &fakeTransport{handler: func(*Request, int) (*Response, error) {
				return &Response{StatusCode: status, Headers: map[string]string{"retry-after": "60"}}, nil
			}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := testClient(api, nil).poll(ctx, pollParams{pollURL: "https://firefly-3p.ff.adobe.io/jobs/1", label: "image", timeout: 10 * time.Millisecond, pollInterval: time.Millisecond})
			var temporary *UpstreamTemporaryError
			require.ErrorAs(t, err, &temporary)
			require.Equal(t, ErrorTypeTimeout, temporary.ErrorType)
			require.Len(t, api.calls, 1, "do not retry when Retry-After exceeds the job deadline")
		})
	}
}

func TestAdobeImageTimingsSeparateSubmitPollAndDownload(t *testing.T) {
	api := &fakeTransport{handler: func(req *Request, index int) (*Response, error) {
		time.Sleep(3 * time.Millisecond)
		if req.URL == ImageSubmitURL {
			return jsonResponse(t, 200, map[string]any{}, map[string]string{"x-override-status-link": "https://firefly-3p.ff.adobe.io/jobs/1"}), nil
		}
		if index == 1 {
			return jsonResponse(t, 202, nil, nil), nil
		}
		return jsonResponse(t, 200, map[string]any{"outputs": []any{map[string]any{"image": map[string]any{"presignedUrl": "https://cdn/img.png"}}}}, nil), nil
	}}
	download := &fakeTransport{handler: func(*Request, int) (*Response, error) {
		time.Sleep(3 * time.Millisecond)
		return bytesResponse(200, []byte("image")), nil
	}}
	result, err := testClient(api, download).GenerateImage(context.Background(), GenerateImageInput{
		Token: "token", Options: ImagePayloadOptions{Prompt: "x", UpstreamModelID: "gpt-image", UpstreamModelVersion: "2.5-flare"}, PollInterval: time.Millisecond,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Timings.SubmitAttempts)
	require.Equal(t, 2, result.Timings.PollCount)
	require.Equal(t, 1, result.Timings.DownloadAttempts)
	require.GreaterOrEqual(t, result.Timings.SubmitMS, int64(3))
	require.GreaterOrEqual(t, result.Timings.PollMS, int64(6))
	require.GreaterOrEqual(t, result.Timings.DownloadMS, int64(3))
}
