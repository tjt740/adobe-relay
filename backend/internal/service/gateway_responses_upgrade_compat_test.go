//go:build unit

package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The upstream signed-thinking bridge must coexist with the fork's custom tools.
func TestOpus55ResponsesPreservesSignedThinkingAndCustomTools(t *testing.T) {
	payload := strings.Join([]string{
		"event: message_start\n" + `data: {"type":"message_start","message":{"id":"msg_signed_tool","model":"claude-opus-5-5","content":[],"usage":{"input_tokens":10}}}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Use the tool."}}`,
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"signed-test-block"}}`,
		"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":0}`,
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_exec","name":"exec","input":{}}}`,
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"input\":\"pwd\"}"}}`,
		"event: content_block_stop\n" + `data: {"type":"content_block_stop","index":1}`,
		"event: message_delta\n" + `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":5}}`,
		"event: message_stop\n" + `data: {"type":"message_stop"}`,
	}, "\n\n") + "\n\n"
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "buffered", true: "streaming"}[stream], func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(payload))}
			mapping := apicompat.ResponsesClientToolMapping{CustomTools: map[string]bool{"exec": true}}
			svc := &GatewayService{}
			var err error
			if stream {
				_, err = svc.handleResponsesStreamingResponse(resp, c, "public-opus", "claude-opus-5-5", nil, time.Now(), mapping)
			} else {
				_, err = svc.handleResponsesBufferedStreamingResponse(resp, c, "public-opus", "claude-opus-5-5", nil, time.Now(), mapping)
			}
			require.NoError(t, err)
			body := recorder.Body.String()
			require.Contains(t, body, "anthropic-thinking-v1:")
			require.Contains(t, body, `"type":"custom_tool_call"`)
			require.Contains(t, body, `"input":"pwd"`)
			require.Contains(t, body, `"call_id":"toolu_exec"`)
			require.Contains(t, body, `"model":"public-opus"`)
			require.NotContains(t, body, "signed-test-block", "the signature belongs in the opaque envelope")
		})
	}
}
