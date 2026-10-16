//go:build unit

package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdobeNoAccountReturnsActionableFailureAndRetry(t *testing.T) {
	for _, reason := range []string{"proxy_unavailable", "no_enabled_accounts", "account_authorization_required", "empty_account_pool", "accounts_temporarily_unavailable"} {
		t.Run(reason, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			adobeImagesUnavailableError(c, reason, 30)
			require.Equal(t, 503, w.Code)
			require.Equal(t, "30", w.Header().Get("Retry-After"))
			var body struct {
				Error struct{ Code, Reason, Message string }
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, "no_available_accounts", body.Error.Code)
			require.Equal(t, reason, body.Error.Reason)
			require.NotEmpty(t, body.Error.Message)
		})
	}
}
func TestAdobeNoAccountCannotRewriteCommittedResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.String(200, "already delivered")
	c.Writer.Flush()
	adobeImagesUnavailableError(c, "proxy_unavailable", 30)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "already delivered", w.Body.String())
	require.Empty(t, w.Header().Get("Retry-After"))
}
