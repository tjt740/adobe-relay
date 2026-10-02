//go:build unit

package service

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdobeDiagnosticsPreserveNetworkFailureWithoutSyntheticStatus(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(OpsUpstreamStatusCodeKey, 503)
	proxyID := int64(44)
	account := &Account{ID: 10, Platform: PlatformAdobe, ProxyID: &proxyID,
		Proxy: &Proxy{ID: 44, Name: "test proxy"}, Credentials: map[string]any{"access_token": "exact-private-token"}}
	err := &adobe.OperationError{Stage: "download", JobID: "job-123", Attempts: 3, ElapsedMS: 1234,
		Err: adobe.NewUpstreamTemporaryError(`Get "https://cdn.example/file?signature=private": EOF exact-private-token`, 0, adobe.ErrorTypeConnection)}
	fields := RecordAdobeImageFailure(c, account, "gpt-image-2.5-flare", err)
	require.NotEmpty(t, fields)
	require.Zero(t, c.GetInt(OpsUpstreamStatusCodeKey))
	value, exists := c.Get(OpsUpstreamErrorsKey)
	require.True(t, exists)
	events := value.([]*OpsUpstreamErrorEvent)
	require.Len(t, events, 1)
	require.Equal(t, "download", events[0].Stage)
	require.Zero(t, events[0].UpstreamStatusCode)
	require.Equal(t, &proxyID, events[0].ProxyID)
	require.NotContains(t, events[0].Message, "private")
	require.Contains(t, events[0].Message, "EOF")
	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(events[0].Detail), &detail))
	require.Equal(t, "job-123", detail["job_id"])
	require.Equal(t, float64(3), detail["stage_attempts"])
	require.Equal(t, float64(1234), detail["stage_elapsed_ms"])
}

func TestAdobeDiagnosticsDoNotResubmitAcceptedJobs(t *testing.T) {
	for _, stage := range []string{"poll", "download"} {
		err := &adobe.OperationError{Stage: stage, Err: adobe.NewUpstreamTemporaryError("timeout", 0, adobe.ErrorTypeTimeout)}
		failure := classifyAdobeError(err)
		require.Equal(t, NextAccountStop, failure.Failover.NextAccountAction, stage)
		require.Equal(t, 502, failure.Failover.StatusCode)
	}
	failure := classifyAdobeError(&adobe.OperationError{Stage: "submit", Err: adobe.NewUpstreamTemporaryError("HTTP 503", 503, adobe.ErrorTypeStatus)})
	require.Equal(t, NextAccountRetry, failure.Failover.NextAccountAction)
}
