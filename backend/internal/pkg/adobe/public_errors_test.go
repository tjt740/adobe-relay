//go:build unit

package adobe

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicHTTPValidationDetailsDoNotExposeProviderPayload(t *testing.T) {
	for _, tc := range []struct {
		status            int
		body, code, param string
		clientStatus      int
	}{
		{400, `{"error_code":"bad_request","message":"Invalid image size 1396x1861: both edges must be non-zero and multiples of 16 (got 1396x1861); Bearer secret-token https://private.example/?sig=secret"}`, "image_size_not_multiple_of_16", "size", 0},
		{422, `{"error_code":"invalid_size","message":"private-details"}`, "invalid_image_size", "size", 0},
		{400, `{"error_code":"invalid_quality","message":"secret-token"}`, "invalid_image_quality", "quality", 0},
		{400, `{"error_code":"invalid_parameter","param":"background","message":"private-value"}`, "invalid_image_parameter", "background", 0},
		{400, `{"error_code":"invalid_parameter","param":"https://private.example/?sig=secret"}`, "upstream_request_rejected", "", 0},
		{400, `{"error_code":"bad_request","message":"Cookie: private-cookie"}`, "upstream_request_rejected", "", 0},
		{400, `<html>https://private.example/?sig=secret</html>`, "upstream_request_rejected", "", 0},
		{413, `{}`, "input_image_too_large", "image", 0},
		{415, `{}`, "unsupported_image_format", "image", 0},
		{424, `{"message":"invalid image size 123x456: both edges must be non-zero and multiples of 16"}`, "upstream_dependency_failed", "", 502},
		{404, `{}`, "upstream_resource_unavailable", "", 502},
		{409, `{}`, "upstream_request_failed", "", 502},
	} {
		t.Run(fmt.Sprintf("%d/%s", tc.status, tc.code), func(t *testing.T) {
			err := classifyAdobeHTTPError(tc.status, tc.body, "internal provider diagnostics")
			var request *RequestError
			require.ErrorAs(t, err, &request)
			require.Equal(t, tc.code, request.Code)
			require.Equal(t, tc.param, request.Param)
			require.Equal(t, tc.status, request.StatusCode)
			require.Equal(t, tc.clientStatus, request.ClientStatusCode)
			for _, private := range []string{"private", "secret", "https://", "Bearer", "Cookie"} {
				require.NotContains(t, request.User(), private)
			}
			require.False(t, IsRotatable(err), "new status reporting must not add retries")
		})
	}
}

func TestPublicErrorsPreserveOperationAndSingleSubmission(t *testing.T) {
	for _, scenario := range []string{"submit424", "missing_task", "poll424", "failed_task", "malformed_output", "download404"} {
		t.Run(scenario, func(t *testing.T) {
			api := &fakeTransport{handler: func(req *Request, index int) (*Response, error) {
				if index == 0 {
					if scenario == "submit424" {
						return jsonResponse(t, 424, map[string]string{"message": "private provider data"}, nil), nil
					}
					if scenario == "missing_task" {
						return jsonResponse(t, 200, map[string]any{}, nil), nil
					}
					return jsonResponse(t, 200, nil, map[string]string{"x-override-status-link": "https://firefly.adobe.io/jobs/job-1"}), nil
				}
				if scenario == "poll424" {
					return jsonResponse(t, 424, nil, nil), nil
				}
				if scenario == "failed_task" {
					return jsonResponse(t, 200, map[string]any{"status": "FAILED", "message": "private provider data"}, nil), nil
				}
				if scenario == "malformed_output" {
					return jsonResponse(t, 200, map[string]any{"outputs": []any{"malformed"}}, nil), nil
				}
				return completedWith(t, "https://cdn/image.png?signature=private"), nil
			}}
			download := &fakeTransport{handler: func(*Request, int) (*Response, error) { return bytesResponse(404, nil), nil }}
			_, err := generateTestImage(testClient(api, download), t)
			require.Error(t, err)
			d := DescribeError(err)
			wantStage, wantCode, wantHTTP := "poll", "upstream_invalid_response", 200
			switch scenario {
			case "submit424":
				wantStage, wantCode, wantHTTP = "submit", "upstream_dependency_failed", 424
			case "missing_task":
				wantStage = "submit"
			case "poll424":
				wantCode, wantHTTP = "upstream_dependency_failed", 424
			case "failed_task":
				wantCode = "upstream_generation_failed"
			case "download404":
				wantStage, wantCode, wantHTTP = "download", "upstream_resource_unavailable", 404
			}
			require.Equal(t, wantCode, d.Code)
			require.Equal(t, wantStage, d.Stage)
			require.Equal(t, wantHTTP, d.UpstreamStatus)
			require.False(t, CanRetrySubmission(err))
			posts := 0
			for _, request := range api.calls {
				if request.Method == "POST" {
					posts++
				}
			}
			require.Equal(t, 1, posts)
			require.LessOrEqual(t, len(download.calls), 1)
		})
	}
}

func TestPublicUploadErrorsHaveStageAndActualHTTPStatus(t *testing.T) {
	for _, status := range []int{200, 400, 424} {
		api := &fakeTransport{handler: func(*Request, int) (*Response, error) {
			return jsonResponse(t, status, map[string]any{}, map[string]string{"x-request-id": "upload-1"}), nil
		}}
		_, err := testClient(api, nil).UploadImage(context.Background(), "private-token", []byte("image"), "image/png")
		require.Error(t, err)
		d := DescribeError(err)
		require.Equal(t, "upload", d.Stage)
		require.Equal(t, status, d.UpstreamStatus)
		var op *OperationError
		require.ErrorAs(t, err, &op)
		require.Equal(t, "upload-1", op.UpstreamRequestID)
		require.Len(t, api.calls, 1)
	}
}

func TestPublicDownloadRejectionsAreNotInputValidationFailures(t *testing.T) {
	for _, status := range []int{400, 403, 404, 413, 415, 424} {
		download := &fakeTransport{handler: func(*Request, int) (*Response, error) { return bytesResponse(status, nil), nil }}
		_, err := testClient(nil, download).download(context.Background(), "https://cdn/image.png", 64, 0)
		var request *RequestError
		require.ErrorAs(t, err, &request)
		require.Equal(t, 502, request.ClientStatusCode)
		require.Empty(t, request.Param)
		require.Equal(t, "download", DescribeError(err).Stage)
		require.Len(t, download.calls, 1)
	}
}
