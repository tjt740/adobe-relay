//go:build unit

package handler

import (
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAdobeImagesContentRejectionReturns451(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, upstreamStatus := range []int{http.StatusOK, http.StatusBadRequest, http.StatusForbidden, http.StatusUnavailableForLegalReasons} {
		t.Run(fmt.Sprint(upstreamStatus), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			gateway := &service.GatewayService{}
			err := fmt.Errorf("generate: %w", adobe.NewContentRejectedError("private upstream details", upstreamStatus, ""))
			failover := gateway.AdobeFailover(c.Request.Context(), 1, "unused-token", err)

			adobeImagesFailoverError(c, failover)

			require.Equal(t, http.StatusUnavailableForLegalReasons, w.Code)
			require.Equal(t, "adobe_content_rejected", gjson.Get(w.Body.String(), "error.type").String())
			require.Equal(t, "Image content was rejected by the upstream safety filter", gjson.Get(w.Body.String(), "error.message").String())
			require.NotContains(t, w.Body.String(), "private upstream details")
		})
	}
}

type safetyRelayUpstream struct {
	service.HTTPUpstream
	status int
	body   string
	calls  int
}

func (u *safetyRelayUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	return &http.Response{StatusCode: u.status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(u.body))}, nil
}
func TestAdobeImagesRelayContentSafetyReturns451(t *testing.T) {
	for _, mode := range []string{"openai", "gemini"} {
		for _, status := range []int{200, 400, 401, 403, 451, 500} {
			t.Run(fmt.Sprintf("%s/%d", mode, status), func(t *testing.T) {
				upstream := &safetyRelayUpstream{status: status, body: `{"error":{"type":"invalid_request_error","code":"content_policy_violation","message":"content rejected"}}`}
				account := &service.Account{ID: 9371, Platform: service.PlatformAdobe, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"api_key": "test-key", "base_url": "https://relay.example.com"}}
				f := newAdobeAdmissionFixtureWithAccounts(t, upstream, account)
				var w *httptest.ResponseRecorder
				if mode == "gemini" {
					w = f.serveGemini(t, "nano-banana-pro", "generateContent", `{"contents":[{"parts":[{"text":"a cat"}]}]}`)
				} else {
					w = f.serve(t)
				}
				require.Equal(t, 451, w.Code, w.Body.String())
				require.Equal(t, 1, upstream.calls, "safety rejection must not be retried")
				require.NotContains(t, w.Body.String(), "test-key")
				if mode == "gemini" {
					require.Equal(t, "PERMISSION_DENIED", gjson.Get(w.Body.String(), "error.status").String())
				}
				f.assertSlotsReleased(t)
			})
		}
	}
}
func TestAdobeImagesLocalSafetyReturns451BeforeAccountSelection(t *testing.T) {
	for _, mode := range []string{"openai", "gemini"} {
		t.Run(mode, func(t *testing.T) {
			f := newAdobeAdmissionFixture(t)
			f.handler.securityAuditCoordinator = securityaudit.NewCoordinator(nil, blockingHandlerPromptEngine())
			var w *httptest.ResponseRecorder
			if mode == "gemini" {
				w = f.serveGemini(t, "nano-banana-pro", "generateContent", `{"contents":[{"parts":[{"text":"a cat"}]}]}`)
			} else {
				w = f.serve(t)
			}
			require.Equal(t, 451, w.Code, w.Body.String())
			require.Empty(t, f.slots.accountAttempts)
		})
	}
}
func TestImageSafetyDecisionPreservesLegacyAndUnavailable(t *testing.T) {
	original := &securityaudit.Decision{Kind: securityaudit.DecisionBlock, HTTPStatus: 403, ErrorCode: "content_policy_violation", Legacy: &securityaudit.LegacyDecision{Blocked: true, Message: "legacy message"}}
	c, w := securityAuditErrorTestContext(t)
	(&GatewayHandler{}).openAISecurityAuditError(c, imageSecurityAuditDecision(original))
	require.Equal(t, 451, w.Code)
	require.Contains(t, w.Body.String(), "legacy message")
	require.Equal(t, 403, original.HTTPStatus, "do not mutate persisted/shared audit decisions")
	unavailable := promptGuardDecision(securityaudit.DecisionUnavailable)
	require.Same(t, unavailable, imageSecurityAuditDecision(unavailable))
	require.Equal(t, 503, unavailable.HTTPStatus)
}
