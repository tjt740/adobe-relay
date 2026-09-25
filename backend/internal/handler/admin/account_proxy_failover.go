package admin

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/proxyfailover"
	"github.com/gin-gonic/gin"
)

func (h *AccountHandler) GetProxyFailover(c *gin.Context)  { h.proxyFailoverRequest(c, false) }
func (h *AccountHandler) SaveProxyFailover(c *gin.Context) { h.proxyFailoverRequest(c, true) }
func (h *AccountHandler) proxyFailoverRequest(c *gin.Context, save bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, 400, "Invalid account ID")
		return
	}
	if h.proxyFailover == nil {
		response.Error(c, 503, "Proxy failover unavailable")
		return
	}
	var view proxyfailover.View
	if save {
		var policy proxyfailover.Policy
		if c.ShouldBindJSON(&policy) != nil {
			response.Error(c, 400, "Invalid policy")
			return
		}
		view, err = h.proxyFailover.Save(c.Request.Context(), id, policy)
	} else {
		view, err = h.proxyFailover.Get(c.Request.Context(), id)
	}
	switch {
	case errors.Is(err, proxyfailover.ErrConflict):
		response.Error(c, 409, err.Error())
	case errors.Is(err, proxyfailover.ErrInvalid):
		response.Error(c, 400, err.Error())
	case errors.Is(err, sql.ErrNoRows):
		response.Error(c, 404, "Adobe OAuth account not found")
	case err != nil:
		response.Error(c, 500, "Unable to load or save proxy failover")
	default:
		response.Success(c, view)
	}
}
