//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdobePublicErrorsPreserveNoReplayAndDiagnostics(t *testing.T) {
	for _, status := range []int{400, 424} {
		api := &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) {
			return adobeJSONResponse(t, status, map[string]string{"error_code": "bad_request", "message": "Invalid image size 1396x1861: both edges must be non-zero and multiples of 16 (got 1396x1861)"}, nil), nil
		}}
		svc := newAdobeTestService(t, adobe.NewClient(adobe.ClientConfig{Transport: api}), nil)
		account := adobeTestAccount()
		_, err := svc.Generate(context.Background(), account, "tok", &OpenAIImagesRequest{Model: "gpt-image-2.5-sunburst", Prompt: "fixture", Size: "2448x3264", N: 1})
		require.Error(t, err)
		f := classifyAdobeError(err).Failover
		require.Equal(t, NextAccountStop, f.NextAccountAction)
		require.Equal(t, status, f.ClientUpstreamStatus)
		require.Equal(t, "submit", f.ClientErrorStage)
		if status == 400 {
			require.Equal(t, 400, f.ClientStatusCode)
			require.Equal(t, "size", f.ClientErrorParam)
			require.Equal(t, "image_size_not_multiple_of_16", f.ClientErrorCode)
		} else {
			require.Equal(t, 502, f.ClientStatusCode)
			require.Equal(t, "upstream_dependency_failed", f.ClientErrorCode)
		}
		require.Len(t, adobeSubmitBodies(t, api), 1)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		RecordAdobeImageFailure(c, account, "gpt-image-2.5-sunburst", err)
		v, _ := c.Get(OpsUpstreamErrorsKey)
		events := v.([]*OpsUpstreamErrorEvent)
		var detail map[string]any
		require.NoError(t, json.Unmarshal([]byte(events[0].Detail), &detail))
		require.Equal(t, f.ClientErrorCode, detail["code"])
		require.Equal(t, f.ClientErrorParam, detail["param"])
		require.Equal(t, "upstream", detail["source"])
	}
}

func TestAdobeInputFetchFailureSeparatesPolicyAndNetwork(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errAdobeInputImageBlocked, 400, "input_image_url_not_allowed"},
		{fmt.Errorf("%w: connection refused", errAdobeInputImageFetchFailed), 502, "input_image_fetch_failed"},
		{fmt.Errorf("%w: %w", errAdobeInputImageFetchFailed, context.DeadlineExceeded), 504, "input_image_fetch_timeout"},
	} {
		err := &adobe.OperationError{Stage: "input_fetch", Err: adobeInputImageRequestError(tc.err)}
		f := classifyAdobeError(err).Failover
		require.Equal(t, tc.status, f.ClientStatusCode)
		require.Equal(t, tc.code, f.ClientErrorCode)
		require.Equal(t, "input_fetch", f.ClientErrorStage)
		require.Equal(t, NextAccountStop, f.NextAccountAction)
	}
}

func TestAdobeLocalParametersAreRejectedBeforeAnyUpstreamCall(t *testing.T) {
	for _, tc := range []struct {
		req                OpenAIImagesRequest
		code, param, stage string
	}{
		{OpenAIImagesRequest{Model: "gpt-image-2", N: 11}, "invalid_n", "n", "validation"},
		{OpenAIImagesRequest{Model: "gpt-image-2", OutputFormat: "webp"}, "unsupported_output_format", "output_format", "validation"},
		{OpenAIImagesRequest{Model: "gpt-image-2", OutputFormat: "jpeg", Background: "transparent"}, "incompatible_image_background", "background", "validation"},
		{OpenAIImagesRequest{Model: "unavailable-image-model"}, "unsupported_image_model", "model", "prepare"},
	} {
		api := &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) {
			t.Fatal("validation must precede network calls")
			return nil, nil
		}}
		svc := newAdobeTestService(t, adobe.NewClient(adobe.ClientConfig{Transport: api}), nil)
		_, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &tc.req)
		require.Error(t, err)
		f := classifyAdobeError(err)
		require.Equal(t, 400, f.Failover.ClientStatusCode)
		require.Equal(t, tc.code, f.Failover.ClientErrorCode)
		require.Equal(t, tc.param, f.Failover.ClientErrorParam)
		require.Equal(t, tc.stage, f.Failover.ClientErrorStage)
		require.Zero(t, f.Cooldown)
		require.False(t, f.InvalidateToken)
		require.Empty(t, api.calls)
	}
}
