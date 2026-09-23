package admin

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/clash"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func NewProxyHandlerWithClash(adminService service.AdminService, manager *clash.Manager) *ProxyHandler {
	h := NewProxyHandler(adminService)
	h.clash = manager
	return h
}

func (h *ProxyHandler) ClashStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.clash == nil {
		response.Success(c, clash.View{Nodes: []clash.NodeView{}})
		return
	}
	result, err := h.clash.Status(c.Request.Context())
	if err != nil {
		response.InternalError(c, "无法读取 Clash 订阅配置")
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) ClashPreview(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req struct {
		URL string `json:"url"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "订阅请求格式无效")
		return
	}
	if h.clash == nil {
		response.BadRequest(c, "Clash 服务未配置")
		return
	}
	result, err := h.clash.Preview(c.Request.Context(), strings.TrimSpace(req.URL))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) ClashImport(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req clash.ImportRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<10)
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "订阅请求格式无效")
		return
	}
	if h.clash == nil {
		response.BadRequest(c, "Clash 服务未配置")
		return
	}
	result, err := h.clash.Import(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) rejectManagedClashProxy(c *gin.Context, id int64) bool {
	if h.clash == nil {
		return false
	}
	yes, err := h.clash.IsManaged(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "无法读取代理所属订阅")
		return true
	}
	if yes {
		response.BadRequest(c, "此代理由 Clash 订阅管理，请在 Clash 订阅中更新或取消选择该节点")
		return true
	}
	return false
}
