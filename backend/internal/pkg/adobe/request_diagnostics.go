package adobe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// RequestDiagnostics describes the exact bearer/session submitted by this SDK call.
// It never records the bearer, Cookie, prompt, signed URL or complete response body.
type RequestDiagnostics struct {
	CredentialFingerprint string `json:"credential_fingerprint,omitempty"`
	BearerFingerprint     string `json:"bearer_fingerprint,omitempty"`
	SessionFingerprint    string `json:"session_fingerprint,omitempty"`
	FingerprintVersion    string `json:"fingerprint_version,omitempty"`
	CredentialExpiresAt   int64  `json:"credential_expires_at,omitempty"`
	ExpiredAtStart        bool   `json:"expired_at_start"`
	UpstreamErrorCode     string `json:"upstream_error_code,omitempty"`
	AccessError           string `json:"access_error,omitempty"`
	ResponseBodySHA256    string `json:"response_body_sha256,omitempty"`
	UpstreamRequestID     string `json:"upstream_request_id,omitempty"`
}

func requestDiagnostics(input GenerateImageInput, started time.Time, resp *Response) RequestDiagnostics {
	d := RequestDiagnostics{FingerprintVersion: "adobe-bearer-arp-v1"}
	fingerprint := func(kind, value string) string {
		if value == "" {
			return ""
		}
		raw, _ := json.Marshal([]string{kind, value})
		h := sha256.Sum256(raw)
		return hex.EncodeToString(h[:])
	}
	d.BearerFingerprint = fingerprint("adobe-bearer-v1", input.Token)
	d.SessionFingerprint = fingerprint("adobe-arp-v1", input.ARPSessionID)
	if input.Token != "" {
		raw, _ := json.Marshal([]string{d.FingerprintVersion, input.Token, input.ARPSessionID})
		h := sha256.Sum256(raw)
		d.CredentialFingerprint = hex.EncodeToString(h[:])
		if exp, ok := DecodeJWTExp(input.Token); ok && exp > 0 && exp <= 253402300799 {
			d.CredentialExpiresAt = exp
			d.ExpiredAtStart = exp <= started.Unix()
		}
	}
	if resp != nil {
		for _, header := range []string{"x-request-id", "x-adobe-request-id", "x-correlation-id", "request-id"} {
			if d.UpstreamRequestID = diagnosticID(resp.Header(header)); d.UpstreamRequestID != "" {
				break
			}
		}
		if resp.StatusCode >= 400 {
			d.UpstreamErrorCode = diagnosticID(ErrorCode(resp.BodyPreview()))
			d.AccessError = diagnosticID(resp.Header("x-access-error"))
			h := sha256.Sum256(resp.Body)
			d.ResponseBodySHA256 = hex.EncodeToString(h[:])
		}
	}
	return d
}
func attachRequestDiagnostics(err error, d RequestDiagnostics) error {
	var op *OperationError
	if !errors.As(err, &op) {
		return err
	}
	copy := *op
	copy.RequestDiagnostics = d
	return &copy
}
