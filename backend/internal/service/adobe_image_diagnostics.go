package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RecordAdobeImageFailure preserves the actual HTTP status (zero for a network
// failure), rather than the synthetic 502 used for the client response.
func RecordAdobeImageFailure(c *gin.Context, account *Account, model string, err error) []zap.Field {
	if err == nil {
		return nil
	}
	message := err.Error()
	if account != nil {
		for _, key := range []string{"access_token", "refresh_token", "cookie", "adobe_session_cookie", "arp_session_id", "api_key"} {
			if secret := account.GetCredential(key); secret != "" {
				message = strings.ReplaceAll(message, secret, "[redacted]")
			}
		}
	}
	message = adobe.SafeDiagnosticMessage(message)
	stage, errorType := "unknown", "internal"
	status, attempts := 0, 0
	var elapsedMS int64
	requestID, jobID := "", ""
	downloadFallback := false
	var requestDiagnostic adobe.RequestDiagnostics
	var reqErr *adobe.RequestError
	if errors.As(err, &reqErr) {
		status, errorType = reqErr.StatusCode, reqErr.ErrorType
		if errorType == "" {
			errorType = "request"
		}
	}
	if errors.Is(err, context.Canceled) {
		errorType = "canceled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		errorType = adobe.ErrorTypeTimeout
	}
	var op *adobe.OperationError
	if errors.As(err, &op) {
		stage, attempts, elapsedMS = op.Stage, op.Attempts, op.ElapsedMS
		requestID, jobID = op.UpstreamRequestID, op.JobID
		downloadFallback = op.DownloadFallback
		requestDiagnostic = op.RequestDiagnostics
	}
	proxyID, proxyName := opsUpstreamProxyAttribution(account)
	public := adobe.DescribeError(err)
	proxyFingerprint, proxyUpdatedAt := adobeProxyDiagnosticIdentity(account)
	assessment := ""
	if public.UpstreamStatus == 403 {
		assessment = "access_denied_undetermined"
		var ent *adobe.NotEntitledError
		var quota *adobe.QuotaExhaustedError
		if errors.As(err, &ent) {
			assessment = "explicit_entitlement_denial"
		}
		if errors.As(err, &quota) {
			assessment = "explicit_quota_exhausted"
		}
	}
	status = public.UpstreamStatus
	stage = public.Stage
	detail, _ := json.Marshal(map[string]any{
		"code": public.Code, "param": public.Param, "source": public.Source,
		"stage": stage, "error_type": errorType, "upstream_http_status": status,
		"stage_attempts": attempts, "stage_elapsed_ms": elapsedMS, "job_id": jobID,
		"upstream_request_id": requestID, "message": message,
		"download_fallback":   downloadFallback,
		"request_diagnostics": requestDiagnostic, "proxy_fingerprint": proxyFingerprint, "proxy_updated_at": proxyUpdatedAt,
		"forbidden_assessment": assessment, "egress_ip_observation": "not_observed",
	})
	setOpsUpstreamError(c, status, message, string(detail))
	if c != nil {
		// Clear a previous attempt's HTTP status when this attempt failed before
		// receiving any HTTP response.
		c.Set(OpsUpstreamStatusCodeKey, status)
	}
	ev := OpsUpstreamErrorEvent{
		Platform: PlatformAdobe, RequestedModel: model, ProxyID: proxyID, ProxyName: proxyName,
		UpstreamStatusCode: status, UpstreamRequestID: requestID,
		CredentialFingerprint: requestDiagnostic.CredentialFingerprint, ProxyFingerprint: proxyFingerprint,
		Kind: "request_error", Stage: stage, Reason: errorType, Message: message, Detail: string(detail),
	}
	if failure := classifyAdobeError(err); failure.Failover != nil {
		ev.Reason = string(failure.Failover.Reason)
	}
	if account != nil {
		ev.AccountID, ev.AccountName = account.ID, account.Name
	}
	if status > 0 {
		ev.Kind = "http_error"
	}
	if stage == "submit" {
		ev.UpstreamURL = adobe.ImageSubmitURL
	}
	appendOpsUpstreamError(c, ev)
	return []zap.Field{
		zap.String("error", message), zap.String("stage", stage), zap.String("error_type", errorType),
		zap.Int("upstream_http_status", status), zap.Int("stage_attempts", attempts),
		zap.Int64("stage_elapsed_ms", elapsedMS), zap.String("upstream_request_id", requestID),
		zap.String("job_id", jobID), zap.Any("proxy_id", proxyID),
		zap.Bool("download_fallback", downloadFallback),
		zap.Any("request_diagnostics", requestDiagnostic), zap.String("proxy_fingerprint", proxyFingerprint), zap.String("proxy_updated_at", proxyUpdatedAt), zap.String("forbidden_assessment", assessment), zap.String("egress_ip_observation", "not_observed"),
	}
}

func safeOpsDiagnosticFingerprint(s string) string {
	if len(s) != 64 {
		return ""
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return ""
	}
	return strings.ToLower(s)
}
func adobeProxyDiagnosticIdentity(a *Account) (string, string) {
	if a == nil || a.Proxy == nil || a.ProxyID == nil || a.Proxy.ID != *a.ProxyID {
		return "", ""
	}
	p := a.Proxy
	raw, _ := json.Marshal([]any{"adobe-proxy-v1", p.ID, p.Protocol, p.Host, p.Port, p.Username, p.Password})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), p.UpdatedAt.UTC().Format(time.RFC3339Nano)
}
