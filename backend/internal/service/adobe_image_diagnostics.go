package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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
		for _, key := range []string{"access_token", "refresh_token", "cookie", "api_key"} {
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
	}
	proxyID, proxyName := opsUpstreamProxyAttribution(account)
	detail, _ := json.Marshal(map[string]any{
		"stage": stage, "error_type": errorType, "upstream_http_status": status,
		"stage_attempts": attempts, "stage_elapsed_ms": elapsedMS, "job_id": jobID,
		"upstream_request_id": requestID, "message": message,
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
		Kind: "request_error", Stage: stage, Reason: errorType, Message: message, Detail: string(detail),
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
	}
}
