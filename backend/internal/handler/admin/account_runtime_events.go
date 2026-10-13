package admin

import (
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// Admin-authenticated invalidations only; no account secrets travel in SSE.
// Each connection expires so reconnects re-check authorization. Polling this
// local revision does not send any requests to Adobe or Okad.
func (h *AccountHandler) RuntimeEvents(c *gin.Context) {
	if h.accountTestService == nil {
		response.Error(c, 503, "状态同步暂不可用")
		return
	}
	revision, err := h.accountTestService.AccountRuntimeRevision(c.Request.Context())
	if err != nil {
		response.Error(c, 503, "状态同步暂不可用")
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-store")
	c.Header("X-Accel-Buffering", "no")
	send := func(kind string) { fmt.Fprintf(c.Writer, "data: {\"type\":%q}\n\n", kind); c.Writer.Flush() }
	send("change")
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	end := time.NewTimer(30 * time.Second)
	defer end.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-end.C:
			return
		case <-tick.C:
			next, err := h.accountTestService.AccountRuntimeRevision(c.Request.Context())
			if err != nil {
				send("unavailable")
				return
			}
			if next != revision {
				revision = next
				send("change")
			} else {
				send("heartbeat")
			}
		}
	}
}
