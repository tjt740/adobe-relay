package admin

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type adobeWebsiteModelProvider interface {
	Fetch(context.Context) (*adobe.WebsiteModelsResult, error)
}

// GetAdobeWebsiteModels proxies public Adobe discovery for group naming only.
// It does not change the supported generation models or the group allowlist.
func (h *GroupHandler) GetAdobeWebsiteModels(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.rejectUnsupportedSimpleModeOperation(c, "advanced") {
		return
	}
	result, err := h.adobeWebsiteModels.Fetch(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Adobe model list is temporarily unavailable")
		return
	}
	response.Success(c, result)
}
