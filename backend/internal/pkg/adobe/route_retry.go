package adobe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
)

type singleSubmissionKey struct{}
type submissionBudget struct{ used atomic.Bool }

// WithSingleSubmission gives each source one submission. A surrounding review
// owns the three-source budget; the SDK must not multiply it by its own retries.
func WithSingleSubmission(ctx context.Context) context.Context {
	return context.WithValue(ctx, singleSubmissionKey{}, &submissionBudget{})
}

// CanRetrySubmission requires an explicit rejection before job acceptance.
// A transport timeout, 5xx, malformed success, poll or download failure does not
// prove that a paid generation was never started.
func CanRetrySubmission(err error) bool {
	var op *OperationError
	return errors.As(err, &op) && op.Stage == "submit" && op.JobID == "" && op.SubmissionRejected
}

func rejectedSubmission(resp *Response) bool {
	if resp == nil || (resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnavailableForLegalReasons) {
		return false
	}
	// Some gateways attach a usable job location even to an error response.
	if resp.Header("location") != "" || resp.Header("x-job-id") != "" {
		return false
	}
	if job, err := pollURLFromSubmit(resp, "submit"); err == nil && job != "" {
		return false
	}
	var body map[string]any
	if json.Unmarshal(resp.Body, &body) == nil {
		for _, key := range []string{"jobId", "job_id", "taskId", "task_id"} {
			if value, ok := body[key].(string); ok && value != "" {
				return false
			}
		}
		if status, ok := body["status"].(string); ok {
			switch strings.ToUpper(status) {
			case "ACCEPTED", "PENDING", "RUNNING", "COMPLETED", "SUCCEEDED":
				return false
			}
		}
	}
	return true
}
