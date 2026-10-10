package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterClashRoutes registers the token-protected read-only Clash export.
func RegisterClashRoutes(r *gin.Engine, h *handler.Handlers) {
	r.GET("/api/v1/clash/subscribe/:token", h.Admin.Proxy.ClashSubscription)
}
