//go:build unit

package adobe

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdobePollObservationsDoNotExposeUpstreamSecrets(t *testing.T) {
	secret := "sk-secret-private-token"
	body := map[string]any{"status": secret, "outputs": []any{map[string]any{"image": map[string]any{"presignedUrl": "https://cdn.example/image?secret=" + secret}}}}
	p := newPollObservation(2, time.Second, 20*time.Millisecond, &Response{StatusCode: 200}, body, "image", nil)
	require.Equal(t, "UNKNOWN", p.JobStatus)
	require.True(t, p.OutputReady)
	b, err := json.Marshal(p)
	require.NoError(t, err)
	require.NotContains(t, string(b), secret)
	require.NotContains(t, string(b), "https:")
	p = newPollObservation(3, time.Second, 30*time.Millisecond, nil, nil, "image", errors.New(secret))
	require.True(t, p.TransportError)
	b, err = json.Marshal(p)
	require.NoError(t, err)
	require.NotContains(t, string(b), secret)
	require.Equal(t, 0, p.HTTPStatus)
}

func TestAdobePollObservationsKeepKnownStatesAndBoundHistory(t *testing.T) {
	p := newPollObservation(1, 0, time.Millisecond, &Response{StatusCode: 202}, nil, "image", nil)
	require.Equal(t, "PENDING", p.JobStatus)
	p = newPollObservation(2, time.Second, time.Millisecond, &Response{StatusCode: 200, Headers: map[string]string{"x-task-status": "running"}}, nil, "image", nil)
	require.Equal(t, "RUNNING", p.JobStatus)
	p = newPollObservation(3, 2*time.Second, time.Millisecond, &Response{StatusCode: 200}, map[string]any{"status": "QUEUED"}, "image", nil)
	require.Equal(t, "QUEUED", p.JobStatus)
	var timings GenerationTimings
	for i := 1; i <= maxPollObservations+2; i++ {
		timings.recordPoll(PollObservation{Attempt: i, HTTPMS: int64(i)})
	}
	require.Len(t, timings.Polls, maxPollObservations)
	require.Equal(t, 1, timings.Polls[0].Attempt)
	require.Equal(t, maxPollObservations+2, timings.Polls[maxPollObservations-1].Attempt)
	require.Equal(t, 2, timings.PollSamplesDropped)
	require.EqualValues(t, maxPollObservations+2, timings.PollHTTPMaxMS)
}
