package admin

import (
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

func (h *AccountHandler) OkadInventory(c *gin.Context) {
	admin, ok := h.adminService.(service.OkadIntegrationAdmin)
	if !ok {
		response.Error(c, 503, "联动服务不可用")
		return
	}
	after, _ := strconv.ParseInt(c.Query("after_id"), 10, 64)
	var since *time.Time
	if raw := c.Query("since"); raw != "" {
		v, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			response.BadRequest(c, "invalid since")
			return
		}
		since = &v
	}
	items, err := admin.ListOkadInventory(c.Request.Context(), after, 200, since)
	if err != nil {
		response.Error(c, 503, "读取联动状态失败")
		return
	}
	var next int64
	if len(items) == 200 {
		next = items[len(items)-1].ID
	}
	response.Success(c, gin.H{"items": items, "next_cursor": next, "checked_at": time.Now().UTC()})
}
func (h *AccountHandler) OkadApplyCredential(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var p service.OkadCredentialUpdate
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 300000)
	if c.ShouldBindJSON(&p) != nil {
		response.BadRequest(c, "请检查凭据、来源和版本参数")
		return
	}
	admin, ok := h.adminService.(service.OkadIntegrationAdmin)
	if !ok {
		response.Error(c, 503, "联动服务不可用")
		return
	}
	a, err := admin.ApplyOkadCredential(c.Request.Context(), id, p)
	if err != nil {
		if errors.Is(err, service.ErrOkadConflict) {
			response.Error(c, 409, "旧版本、身份不一致或账号已被修改，未覆盖")
		} else {
			response.Error(c, 503, "凭据暂未写入，请查询状态后重试")
		}
		return
	}
	if h.tokenCacheInvalidator != nil {
		_ = h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), a)
	}
	response.Success(c, gin.H{"id": a.ID, "status": a.Status, "schedulable": a.Schedulable, "integration": a.Extra["okad"]})
}
func (h *AccountHandler) OkadVerifyCredential(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	a, err := h.adminService.GetAccount(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "账号不存在")
		return
	}
	if h.accountUsageService == nil {
		response.Error(c, 503, "验证服务不可用")
		return
	}
	a, err = h.accountUsageService.VerifyOkadCredential(c.Request.Context(), a)
	if err != nil {
		response.Error(c, 409, "验证期间账号状态已变化，请重新读取")
		return
	}
	if h.tokenCacheInvalidator != nil {
		_ = h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), a)
	}
	response.Success(c, gin.H{"id": a.ID, "status": a.Status, "schedulable": a.Schedulable, "integration": a.Extra["okad"]})
}
