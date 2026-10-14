package routes

import (
	"crypto/sha256"
	"crypto/subtle"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// This key grants inventory/credential/verification/classification access, never account
// creation, deletion, billing, users or arbitrary account configuration.
func RegisterOkadIntegrationRoutes(v1 *gin.RouterGroup, h *handler.Handlers, cfg *config.Config) {
	group := v1.Group("/integrations/okad")
	group.Use(func(c *gin.Context) {
		key := cfg.Gateway.OkadIntegrationAPIKey
		expected := sha256.Sum256([]byte(key))
		actual := sha256.Sum256([]byte(c.GetHeader("X-Integration-Key")))
		if key == "" || subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			response.Error(c, 401, "invalid integration key")
			c.Abort()
			return
		}
		c.Next()
	})
	group.GET("/capabilities", func(c *gin.Context) {
		response.Success(c, gin.H{"protocol": 2, "site_id": cfg.Gateway.OkadSiteID, "versioned_credentials": true, "verification": true, "account_metadata": true})
	})
	group.POST("/account-metadata", h.Admin.Account.OkadSyncAccountMetadata)
	group.GET("/accounts", h.Admin.Account.OkadInventory)
	group.POST("/accounts/:id/credentials", h.Admin.Account.OkadApplyCredential)
	group.POST("/accounts/:id/verify", h.Admin.Account.OkadVerifyCredential)
}
