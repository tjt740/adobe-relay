//go:build unit

package service

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

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
	err := &adobe.OperationError{Stage: "download", JobID: "job-123", Attempts: 3, ElapsedMS: 1234, DownloadFallback: true,
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
	require.Equal(t, true, detail["download_fallback"])
}

func TestAdobeDiagnosticsDoNotResubmitAcceptedJobs(t *testing.T) {
	for _, stage := range []string{"poll", "download"} {
		err := &adobe.OperationError{Stage: stage, Err: adobe.NewUpstreamTemporaryError("timeout", 0, adobe.ErrorTypeTimeout)}
		failure := classifyAdobeError(err)
		require.Equal(t, NextAccountStop, failure.Failover.NextAccountAction, stage)
		require.Equal(t, 502, failure.Failover.StatusCode)
	}
	failure := classifyAdobeError(&adobe.OperationError{Stage: "submit", Err: adobe.NewUpstreamTemporaryError("HTTP 503", 503, adobe.ErrorTypeStatus)})
	require.Equal(t, NextAccountStop, failure.Failover.NextAccountAction, "5xx submission outcome is unknown")
	failure = classifyAdobeError(&adobe.OperationError{Stage: "submit", SubmissionRejected: true, Err: adobe.NewUpstreamTemporaryError("HTTP 408", 408, adobe.ErrorTypeStatus)})
	require.Equal(t, NextAccountRetry, failure.Failover.NextAccountAction)
}

func TestAdobe403DiagnosticFingerprintSurvivesQueueWithoutCredentials(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	pid := int64(12)
	a := &Account{ID: 22, Platform: PlatformAdobe, ProxyID: &pid, Proxy: &Proxy{ID: 12, Host: "proxy.local", Port: 20202, Username: "private-user", Password: "private-pass"}, Credentials: map[string]any{"access_token": "old-account-token"}}
	fp := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	err := &adobe.OperationError{Stage: "submit", HTTPStatus: 403, SubmissionRejected: true, UpstreamRequestID: "r403", RequestDiagnostics: adobe.RequestDiagnostics{CredentialFingerprint: fp, FingerprintVersion: "adobe-bearer-arp-v1", UpstreamErrorCode: "access_error"}, Err: adobe.NewAuthError("Adobe denied", 403)}
	RecordAdobeImageFailure(c, a, "gpt-image-2.5-sunburst", err)
	raw, _ := c.Get(OpsUpstreamErrorsKey)
	events := raw.([]*OpsUpstreamErrorEvent)
	entry := &OpsInsertErrorLogInput{UpstreamErrors: events}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	events, e := ParseOpsUpstreamErrors(*entry.UpstreamErrorsJSON)
	require.NoError(t, e)
	require.Equal(t, fp, events[0].CredentialFingerprint)
	require.Len(t, events[0].ProxyFingerprint, 64)
	b, e := json.Marshal(events)
	require.NoError(t, e)
	for _, secret := range []string{"private-user", "private-pass", "old-account-token"} {
		require.NotContains(t, string(b), secret)
	}
	require.Contains(t, string(b), "access_denied_undetermined")
	require.Contains(t, string(b), "not_observed")
	entry.UpstreamErrorsJSON = nil
	entry.UpstreamErrors = events
	entry.UpstreamErrors[0].CredentialFingerprint = "Bearer unsafe"
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	events, e = ParseOpsUpstreamErrors(*entry.UpstreamErrorsJSON)
	require.NoError(t, e)
	require.Empty(t, events[0].CredentialFingerprint)
}

func TestAdobeProxyFingerprintTracksConfigurationNotRefreshTime(t *testing.T) {
	p := int64(7)
	a := &Account{ProxyID: &p, Proxy: &Proxy{ID: p, Host: "127.0.0.1", Port: 12000}}
	first, _ := adobeProxyDiagnosticIdentity(a)
	a.Proxy.UpdatedAt = a.Proxy.UpdatedAt.Add(time.Minute)
	same, _ := adobeProxyDiagnosticIdentity(a)
	require.Equal(t, first, same)
	a.Proxy.Port++
	changed, _ := adobeProxyDiagnosticIdentity(a)
	require.NotEqual(t, first, changed)
}
