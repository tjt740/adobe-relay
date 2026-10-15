package middleware

import (
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"runtime/metrics"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/semaphore"
)

const imageRequestBaseMemory = int64(32 << 20)

// ImageRequestMemoryBudget runs after authentication but BEFORE any middleware
// reads the body. Business concurrency admission happens after parsing/auditing
// and cannot protect memory occupied by requests waiting for those slots.
//
// Keep the configured Go/container limits and business concurrency unchanged.
// One third of an existing finite Go memory limit budgets image request work;
// leave the rest for downloads, responses, GC headroom and other application work.
func ImageRequestMemoryBudget(maxBodyBytes int64) gin.HandlerFunc {
	limit := debug.SetMemoryLimit(-1) // Read only: a negative value does not set it.
	if limit == math.MaxInt64 || limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	guard := newImageMemoryGuard(limit/3, maxBodyBytes)
	guard.runtimeLimit = limit - limit/10
	return guard.handler
}

type imageMemoryGuard struct {
	capacity     int64
	maxBody      int64
	bytes        *semaphore.Weighted
	lastLog      atomic.Int64
	runtimeLimit int64
}

func newImageMemoryGuard(capacity, maxBody int64) *imageMemoryGuard {
	if capacity < 1 {
		capacity = 1
	}
	return &imageMemoryGuard{capacity: capacity, maxBody: maxBody, bytes: semaphore.NewWeighted(capacity)}
}

func imageRuntimeMemory() uint64 {
	samples := [2]metrics.Sample{{Name: "/memory/classes/total:bytes"}, {Name: "/memory/classes/heap/released:bytes"}}
	metrics.Read(samples[:])
	return samples[0].Value.Uint64() - samples[1].Value.Uint64()
}

func (g *imageMemoryGuard) weight(r *http.Request) int64 {
	size := r.ContentLength
	// Chunked/encoded input cannot be estimated from its wire length. Reserve
	// the full budget, permitting one existing-size-limit request to progress.
	if size < 0 || (r.Header.Get("Content-Encoding") != "" && !strings.EqualFold(r.Header.Get("Content-Encoding"), "identity")) {
		return g.capacity
	}
	// Six times the wire body covers its retained raw copy, multipart or data
	// URL representation, decoded uploads and temporary audit/encoding copies.
	// Saturate rather than overflow or reject a previously valid large request.
	if size > (math.MaxInt64-imageRequestBaseMemory)/6 {
		return g.capacity
	}
	weight := imageRequestBaseMemory + size*6
	if weight > g.capacity {
		return g.capacity
	}
	return weight
}

func isAdobeImageMemoryRequest(c *gin.Context) bool {
	if c.Request == nil || c.Request.Method != http.MethodPost {
		return false
	}
	key, ok := GetAPIKeyFromContext(c)
	if !ok || key == nil || key.Group == nil {
		return false
	}
	platform := key.Group.Platform
	if platform != service.PlatformAdobe && platform != service.PlatformComposite {
		return false
	}
	path := strings.TrimSuffix(c.Request.URL.Path, "/async")
	if strings.HasSuffix(path, "/images/generations") || strings.HasSuffix(path, "/images/edits") {
		return true
	}
	return strings.Contains(path, "/v1beta/models/") &&
		(strings.HasSuffix(path, ":generateContent") || strings.HasSuffix(path, ":streamGenerateContent"))
}

func (g *imageMemoryGuard) handler(c *gin.Context) {
	if !isAdobeImageMemoryRequest(c) {
		c.Next()
		return
	}
	if c.Request.Context().Err() != nil {
		c.Abort()
		return
	}
	if g.maxBody > 0 && c.Request.ContentLength > g.maxBody {
		// Never let a declared oversized upload bypass memory admission and read
		// the entire configured maximum alongside otherwise admitted requests.
		groupModelAllowlistErrorWriter(c)(c, http.StatusRequestEntityTooLarge, "Request body is too large")
		c.Abort()
		return
	}
	weight := g.weight(c.Request)
	// Never queue here: reject before reading/decoding the upload. Check current
	// runtime memory too, since response encoding and other endpoints share it.
	if (g.runtimeLimit > 0 && imageRuntimeMemory() >= uint64(g.runtimeLimit)) || !g.bytes.TryAcquire(weight) {
		g.reject(c)
		return
	}
	defer g.bytes.Release(weight)
	if c.Request.Context().Err() != nil {
		c.Abort()
		return
	}
	c.Next()
}

func (g *imageMemoryGuard) reject(c *gin.Context) {
	if c.Request.Context().Err() == nil {
		c.Header("Retry-After", "1")
		service.MarkOpsClientBusinessLimited(c, "image_memory_capacity")
		message := "Image processing capacity is busy, please retry later"
		if strings.Contains(c.Request.URL.Path, "/v1beta/") {
			GoogleErrorWriter(c, http.StatusTooManyRequests, message)
		} else {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"type": "rate_limit_error", "message": message}})
		}
	}
	c.Abort()
	now := time.Now().Unix()
	last := g.lastLog.Load()
	if now-last >= 5 && g.lastLog.CompareAndSwap(last, now) {
		slog.Info("image_memory.capacity_rejected", "budget_bytes", g.capacity, "queue_enabled", false)
	}
}
