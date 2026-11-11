//go:build unit

package adobe

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestAdobeRequestDiagnosticsUseSubmittedCredentialAndPreserveRejection(t *testing.T) {
	token := fakeToken(t)
	api := &fakeTransport{handler: func(req *Request, _ int) (*Response, error) {
		require.Equal(t, "POST", req.Method)
		return jsonResponse(t, 403, map[string]any{"error_code": "access_error", "message": "Unauthorized to perform request."}, map[string]string{"x-request-id": "request-403", "x-access-error": "access_error"}), nil
	}}
	input := GenerateImageInput{Token: token, ARPSessionID: "private-arp", Options: ImagePayloadOptions{Prompt: "private-prompt", AspectRatio: "1:1", UpstreamModelID: "gpt-image"}}
	_, err := testClient(api, nil).GenerateImage(WithSingleSubmission(context.Background()), input)
	require.Error(t, err)
	require.Len(t, api.calls, 1)
	require.True(t, CanRetrySubmission(err))
	var op *OperationError
	require.True(t, errors.As(err, &op))
	d := op.RequestDiagnostics
	require.Len(t, d.CredentialFingerprint, 64)
	require.Len(t, d.BearerFingerprint, 64)
	require.Len(t, d.SessionFingerprint, 64)
	require.Len(t, d.ResponseBodySHA256, 64)
	require.Equal(t, "access_error", d.UpstreamErrorCode)
	require.Equal(t, "request-403", d.UpstreamRequestID)
	require.NotContains(t, err.Error(), "Token invalid or expired")
	raw, _ := json.Marshal(d)
	for _, secret := range []string{token, "private-arp", "private-prompt"} {
		require.NotContains(t, string(raw), secret)
	}
	in2 := input
	in2.Token = "different-token"
	require.NotEqual(t, d.CredentialFingerprint, requestDiagnostics(in2, time.Now(), nil).CredentialFingerprint)
	require.Equal(t, d.CredentialFingerprint, requestDiagnostics(input, time.Now(), nil).CredentialFingerprint)
	require.Equal(t, d.SessionFingerprint, requestDiagnostics(in2, time.Now(), nil).SessionFingerprint)
	in2 = input
	in2.ARPSessionID = "different-session"
	d2 := requestDiagnostics(in2, time.Now(), nil)
	require.NotEqual(t, d.CredentialFingerprint, d2.CredentialFingerprint)
	require.NotEqual(t, d.SessionFingerprint, d2.SessionFingerprint)
	require.Equal(t, d.BearerFingerprint, d2.BearerFingerprint)
	rawError := &OperationError{Stage: "poll", JobID: "accepted", Err: errors.New("failed")}
	wrapped := attachRequestDiagnostics(rawError, d)
	require.Empty(t, rawError.RequestDiagnostics.CredentialFingerprint, "do not mutate an error shared with another request")
	require.False(t, CanRetrySubmission(wrapped))
}
func TestAdobeRequestDiagnosticsRejectSecretLookingHeaders(t *testing.T) {
	d := requestDiagnostics(GenerateImageInput{}, time.Now(), &Response{StatusCode: 403, Headers: map[string]string{"x-request-id": "Bearer secret", "x-access-error": strings.Repeat("x", 161)}, Body: []byte(`{"error_code":"http://private/secret"}`)})
	require.Empty(t, d.UpstreamRequestID)
	require.Empty(t, d.AccessError)
	require.Empty(t, d.UpstreamErrorCode)
	require.Empty(t, d.CredentialFingerprint)
}
