package admin

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/clash"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *ProxyHandler) RelatedAccounts(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, idErr := strconv.ParseInt(c.Param("id"), 10, 64)
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	relation := c.DefaultQuery("relation", "all")
	if idErr != nil || pageErr != nil || sizeErr != nil || id <= 0 || page < 1 || page > 100000 || size < 1 || size > 50 ||
		(relation != "all" && relation != "current" && relation != "departed") || len(c.Query("search")) > 200 {
		response.BadRequest(c, "Invalid proxy or pagination")
		return
	}
	if h.clash == nil {
		response.Error(c, 503, "Proxy account details unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	view, err := h.clash.RelatedAccounts(ctx, id, clash.RelatedAccountsQuery{
		Page: page, PageSize: size, Relation: relation, Search: c.Query("search"),
	})
	switch {
	case errors.Is(err, clash.ErrRelatedAccountsQuery):
		response.BadRequest(c, "Invalid related accounts query")
	case errors.Is(err, clash.ErrRelatedProxyNotFound):
		response.NotFound(c, "Proxy not found")
	case err != nil:
		response.InternalError(c, "Unable to read related accounts")
	default:
		response.Success(c, view)
	}
}
