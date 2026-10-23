//go:build unit

package adobe

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestRouteReviewOnlyRetriesExplicitUnacceptedSubmission(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stage    string
		response *Response
		want     bool
	}{
		{"rejected 408", "submit", &Response{StatusCode: 408}, true},
		{"rejected 429", "submit", &Response{StatusCode: 429}, true},
		{"unknown transport", "submit", nil, false},
		{"unknown 502", "submit", &Response{StatusCode: 502}, false},
		{"accepted 200 malformed", "submit", &Response{StatusCode: 200}, false},
		{"408 with job", "submit", &Response{StatusCode: 408, Headers: map[string]string{"x-override-status-link": "https://firefly-3p.ff.adobe.io/jobs/accepted"}}, false},
		{"poll failure", "poll", &Response{StatusCode: 408}, false},
		{"download failure", "download", &Response{StatusCode: 408}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := operationError(errors.New("fixture failure"), tc.stage, time.Now(), 1, tc.response, "")
			require.Equal(t, tc.want, CanRetrySubmission(err))
		})
	}
}

func TestRouteReviewSubmissionBudgetSpansAuthAndPayloadCalls(t *testing.T) {
	api := &fakeTransport{handler: func(*Request, int) (*Response, error) { return &Response{StatusCode: 408}, nil }}
	c := testClient(api, nil)
	ctx := WithSingleSubmission(context.Background())
	for i := 0; i < 2; i++ {
		_, _ = c.postSubmit(ctx, ImageSubmitURL, nil, nil, []byte("{}"))
	}
	require.Len(t, api.calls, 1, "one source must never multiply the outer three-source budget")
}
