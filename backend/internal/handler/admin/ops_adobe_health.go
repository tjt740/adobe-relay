package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *OpsHandler) AdobeHealthReport(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, 503, "报表暂不可用")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	report, err := h.opsService.AdobeHealthReport(c.Request.Context())
	if err != nil {
		response.Error(c, 503, "报表读取失败，请稍后重试")
		return
	}
	response.Success(c, report)
}

func (h *OpsHandler) SendAdobeHealthReport(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, 503, "报表暂不可用")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.opsService.SendAdobeHealthReport(c.Request.Context()); err != nil {
		response.Error(c, 503, err.Error())
		return
	}
	response.Success(c, gin.H{"sent": true, "message": "邮件服务器已接受报表，请查看专属收件邮箱"})
}
