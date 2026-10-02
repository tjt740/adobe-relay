package adobe

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// OperationError preserves the original typed error while identifying the failed
// stage. URLs and credentials are deliberately excluded from its metadata.
type OperationError struct {
	Stage             string
	UpstreamRequestID string
	JobID             string
	Attempts          int
	ElapsedMS         int64
	Err               error
}

func (e *OperationError) Error() string { return e.Err.Error() }
func (e *OperationError) Unwrap() error { return e.Err }

func operationError(err error, stage string, started time.Time, attempts int, resp *Response, jobURL string) error {
	if err == nil {
		return nil
	}
	var existing *OperationError
	if errors.As(err, &existing) {
		// Preserve the most specific stage (download inside poll inside submit).
		if existing.JobID == "" {
			copy := *existing
			copy.JobID = diagnosticJobID(jobURL)
			return &copy
		}
		return err
	}
	id := ""
	for _, header := range []string{"x-request-id", "x-adobe-request-id", "x-correlation-id", "request-id"} {
		if id = diagnosticID(resp.Header(header)); id != "" {
			break
		}
	}
	return &OperationError{Stage: stage, UpstreamRequestID: id, JobID: diagnosticJobID(jobURL),
		Attempts: attempts, ElapsedMS: time.Since(started).Milliseconds(), Err: err}
}

var safeDiagnosticID = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,160}$`)

func diagnosticID(value string) string {
	if safeDiagnosticID.MatchString(value) {
		return value
	}
	return ""
}

func diagnosticJobID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || !isAdobeIOHost(u.Hostname()) {
		return ""
	}
	parts := strings.Split(strings.TrimRight(u.Path, "/"), "/")
	return diagnosticID(parts[len(parts)-1])
}

var (
	diagnosticURL    = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)
	diagnosticSecret = regexp.MustCompile(`(?i)("?(?:authorization|cookie|set-cookie|access_token|refresh_token|client_secret|api_key|password|token)"?\s*[:=]\s*)(?:"[^"\r\n]*"|[^\s,;}]+)`)
	diagnosticBearer = regexp.MustCompile(`(?i)Bearer\s+[a-zA-Z0-9._~+/=-]+`)
	diagnosticJWT    = regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`)
	diagnosticCookie = regexp.MustCompile(`(?i)("?(?:cookie|set-cookie)"?\s*[:=]).*`)
	diagnosticAPIKey = regexp.MustCompile(`sk-[a-zA-Z0-9_-]{8,}`)
)

// SafeDiagnosticMessage is for internal logs, never a client-facing message.
// Strip complete URLs (including signed queries/userinfo), credentials and JWTs
// before truncation so an incomplete credential cannot escape redaction.
func SafeDiagnosticMessage(message string) string {
	message = strings.ReplaceAll(strings.ReplaceAll(message, "\r", " "), "\n", " ")
	message = diagnosticURL.ReplaceAllString(message, "[url redacted]")
	message = diagnosticBearer.ReplaceAllString(message, "Bearer [redacted]")
	message = diagnosticCookie.ReplaceAllString(message, "$1[redacted]")
	message = diagnosticSecret.ReplaceAllString(message, "$1[redacted]")
	message = diagnosticJWT.ReplaceAllString(message, "[token redacted]")
	message = diagnosticAPIKey.ReplaceAllString(message, "[key redacted]")
	if len(message) > 2048 {
		message = message[:2048] + "…"
	}
	return message
}
