package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func adobeImagesUnavailableError(c *gin.Context, reason string, retryAfter int) {
	if c.Writer.Written() || service.IsResponseCommitted(c) {
		return
	}
	messages := map[string]string{
		"empty_account_pool":               "No accounts are configured for this request.",
		"no_enabled_accounts":              "No enabled accounts are available for this request.",
		"proxy_unavailable":                "Accounts are waiting for an available proxy. Automatic recovery is in progress.",
		"account_authorization_required":   "Account authorization requires attention.",
		"accounts_temporarily_unavailable": "Accounts are temporarily unavailable. Please retry later.",
	}
	message, ok := messages[reason]
	if !ok {
		reason = "accounts_temporarily_unavailable"
		message = messages[reason]
	}
	c.Header("Retry-After", strconv.Itoa(max(1, min(60, retryAfter))))
	if adobeImagesWritesGemini(c) {
		googleError(c, http.StatusServiceUnavailable, message)
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"type": "api_error", "code": "no_available_accounts", "reason": reason, "message": message}})
}
