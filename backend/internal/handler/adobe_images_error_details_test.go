//go:build unit

package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAdobeImageErrorDetailsAcrossProtocols(t *testing.T) {
	for _, mode := range []string{adobeImagesWriteOpenAI, adobeImagesWriteGemini} {
		t.Run(mode, func(t *testing.T) {
			err := &adobe.OperationError{Stage: "submit", HTTPStatus: 400,
				Err: adobe.NewParameterError("Image width and height must be multiples of 16.", "image_size_not_multiple_of_16", "size")}
			failure := (&service.GatewayService{}).AdobeFailover(context.Background(), 1, "", err)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			setAdobeImagesWriteMode(c, mode)
			c.Header("X-Request-ID", "gateway-request-1")
			adobeImagesFailoverError(c, failure)
			require.Equal(t, 400, recorder.Code)
			require.Equal(t, "false", recorder.Header().Get("x-should-retry"))
			body := recorder.Body.String()
			if mode == adobeImagesWriteOpenAI {
				require.Equal(t, gjson.False, gjson.Get(body, "error.retryable").Type)
				require.Equal(t, gjson.False, gjson.Get(body, "error.resubmit_allowed").Type)
				require.Equal(t, "image_size_not_multiple_of_16", gjson.Get(body, "error.code").String())
				require.Equal(t, "size", gjson.Get(body, "error.param").String())
				require.Equal(t, "submit", gjson.Get(body, "error.stage").String())
				require.Equal(t, "gateway-request-1", gjson.Get(body, "error.request_id").String())
				require.Equal(t, int64(400), gjson.Get(body, "error.upstream_status").Int())
			} else {
				require.Equal(t, "false", gjson.Get(body, "error.details.0.metadata.resubmit_allowed").String())
				require.Equal(t, "INVALID_ARGUMENT", gjson.Get(body, "error.status").String())
				require.Equal(t, "image_size_not_multiple_of_16", gjson.Get(body, "error.details.0.reason").String())
				require.Equal(t, "size", gjson.Get(body, "error.details.0.metadata.param").String())
				require.Equal(t, "submit", gjson.Get(body, "error.details.0.metadata.stage").String())
			}
			require.False(t, failure.ShouldRetryNextAccount())
		})
	}
}

func TestAdobeLocalParserErrorsHaveUsefulCodes(t *testing.T) {
	for _, tc := range []struct{ message, code, param string }{
		{"n must be a positive integer", "invalid_n", "n"},
		{"invalid stream field value", "invalid_stream", "stream"},
		{"images[].image_url is required", "invalid_images", "images"},
		{"multipart boundary is required", "invalid_multipart", ""},
		{"failed to parse request body", "invalid_json", ""},
		{"Request body is empty", "empty_request_body", ""},
		{"Failed to read request body", "request_body_read_failed", ""},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		adobeImagesError(c, 400, "invalid_request_error", tc.message)
		require.Equal(t, tc.code, gjson.Get(w.Body.String(), "error.code").String())
		require.Equal(t, tc.param, gjson.Get(w.Body.String(), "error.param").String())
		require.Equal(t, "validation", gjson.Get(w.Body.String(), "error.stage").String())
	}
}

func TestAdobeCommittedErrorResponseIsNotWrittenAgain(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.JSON(200, gin.H{"accepted": true})
	service.MarkResponseCommitted(c)
	before := w.Body.String()
	adobeImagesError(c, 400, "invalid_request_error", "n must be a positive integer")
	adobeImagesFailoverError(c, &service.UpstreamFailoverError{ClientStatusCode: 502, ClientErrorCode: "upstream_dependency_failed"})
	require.Equal(t, before, w.Body.String())
	require.Equal(t, 200, w.Code)
}

func TestAdobeTerminal502ExplicitlyDisallowsClientResubmission(t *testing.T) {
	err := &adobe.OperationError{Stage: "poll", JobID: "accepted-job", HTTPStatus: 424,
		Err: adobe.NewResponseError("private provider response", "upstream_dependency_failed", "An upstream dependency failed.")}
	failure := (&service.GatewayService{}).AdobeFailover(context.Background(), 1, "", err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	adobeImagesFailoverError(c, failure)
	require.Equal(t, 502, w.Code)
	require.Equal(t, "false", w.Header().Get("x-should-retry"))
	require.Equal(t, gjson.False, gjson.Get(w.Body.String(), "error.resubmit_allowed").Type)
	require.Equal(t, gjson.False, gjson.Get(w.Body.String(), "error.retryable").Type)
	require.Equal(t, int64(424), gjson.Get(w.Body.String(), "error.upstream_status").Int())
	require.Equal(t, "poll", gjson.Get(w.Body.String(), "error.stage").String())
	require.NotContains(t, w.Body.String(), "private")
	require.False(t, failure.ShouldRetryNextAccount())
}
