package adobe

import (
	"net/http"
	"strings"
	"time"
)

const maxPollObservations = 256

// PollObservation contains allowlisted status metadata, never response bodies,
// credentials, signed URLs or arbitrary status/error text from upstream.
type PollObservation struct {
	Attempt        int    `json:"attempt"`
	StartedMS      int64  `json:"started_ms"`
	HTTPMS         int64  `json:"http_ms"`
	HTTPStatus     int    `json:"http_status"`
	JobStatus      string `json:"job_status"`
	OutputReady    bool   `json:"output_ready"`
	TransportError bool   `json:"transport_error,omitempty"`
}

func newPollObservation(attempt int, started, elapsed time.Duration, resp *Response, body map[string]any, outputKey string, err error) PollObservation {
	p := PollObservation{Attempt: attempt, StartedMS: started.Milliseconds(), HTTPMS: elapsed.Milliseconds(), JobStatus: "UNKNOWN", TransportError: err != nil}
	if resp == nil {
		return p
	}
	p.HTTPStatus = resp.StatusCode
	if err != nil {
		return p
	}
	switch status := strings.TrimSpace(jobStatus(body, resp)); status {
	case "QUEUED", "PENDING", "RUNNING", "IN_PROGRESS", "PROCESSING", "COMPLETED", "DONE", "SUCCEEDED", "SUCCESS", "FAILED", "ERROR", "CANCELLED", "CANCELED":
		p.JobStatus = status
	default:
		if resp.StatusCode == http.StatusAccepted {
			p.JobStatus = "PENDING"
		}
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		_, p.OutputReady, _ = presignedURL(body, outputKey)
	}
	return p
}

func (t *GenerationTimings) recordPoll(p PollObservation) {
	t.PollHTTPMaxMS = max(t.PollHTTPMaxMS, p.HTTPMS)
	if len(t.Polls) < maxPollObservations {
		t.Polls = append(t.Polls, p)
		return
	}
	// Keep the early history and the final observation, with an explicit gap count.
	t.Polls[maxPollObservations-1] = p
	t.PollSamplesDropped++
}
