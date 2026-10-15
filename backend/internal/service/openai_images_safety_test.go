package service

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestImagesContentSafetyStreamBeforeAndAfterCommit(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, started := range []bool{false, true} {
			for _, multiline := range []bool{false, true} {
				t.Run(fmt.Sprintf("direct=%v/started=%v/multiline=%v", direct, started, multiline), func(t *testing.T) {
					c, w := createOpenAIImagesSafetyContext()
					if started {
						_, _ = c.Writer.WriteString(": heartbeat\n\n")
						c.Writer.Flush()
					}
					payload := `{"type":"error","error":{"code":"content_policy_violation","message":"blocked"}}`
					frame := "event: error\ndata: " + payload + "\n\n"
					if multiline {
						frame = "event: error\ndata: {\"type\":\"error\",\ndata: \"error\":{\"code\":\"content_policy_violation\",\"message\":\"blocked\"}}\n\n"
					}
					response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(frame))}
					var parsed *OpenAIImagesRequest
					if direct {
						parsed = &OpenAIImagesRequest{Model: "gpt-image-2", ResponseFormat: "b64_json"}
					}
					_, count, _, _, err := (&OpenAIGatewayService{}).handleOpenAIImagesStreamingResponse(response, c, time.Now(), parsed)
					var rejected *OpenAIImagesUpstreamError
					require.ErrorAs(t, err, &rejected)
					require.Equal(t, 451, rejected.StatusCode)
					require.Zero(t, count)
					if started {
						require.Equal(t, 200, w.Code)
						require.Contains(t, w.Body.String(), `"status_code":451`)
						require.Equal(t, 1, strings.Count(w.Body.String(), "event: error"))
					} else {
						require.Equal(t, 451, w.Code, w.Body.String())
						require.Contains(t, w.Header().Get("Content-Type"), "application/json")
						require.Equal(t, int64(451), gjson.Get(w.Body.String(), "error.status_code").Int())
					}
				})
			}
		}
	}
}
func createOpenAIImagesSafetyContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	return c, w
}
func TestImagesContentSafetyDoesNotClassifyGenericErrors(t *testing.T) {
	for _, code := range []string{"invalid_value", "rate_limit_exceeded", "invalid_api_key", "insufficient_quota", "region_blocked", "permission_denied", "image_generation_user_error"} {
		body := []byte(fmt.Sprintf(`{"error":{"type":"invalid_request_error","code":%q,"message":"prompt_unsafe"}}`, code))
		require.Nil(t, imagesContentRejectedHTTPError(400, nil, body), code)
	}
}
