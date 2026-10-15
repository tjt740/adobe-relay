package middleware

import (
	"errors"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"runtime/metrics"
	"strings"
	"sync/atomic"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/semaphore"
)

const imageRequestBaseMemory = int64(4 << 20)
const imageUnbufferedOutputMemory = int64(32 << 20)

// ImageRequestMemoryBudget runs after authentication but BEFORE any middleware
// reads the body. Business concurrency admission happens after parsing/auditing
// and cannot protect memory occupied by requests waiting for those slots.
//
// Keep the configured Go/container limits and business concurrency unchanged.
// Three quarters of the finite Go limit budgets image work. The remaining quarter
// and the container headroom cover GC and non-image work. Reservations shrink
// after preparation; they remain held through detached generation and delivery.
func ImageRequestMemoryBudget(maxBodyBytes int64) gin.HandlerFunc {
	limit := debug.SetMemoryLimit(-1) // Read only: a negative value does not set it.
	if limit == math.MaxInt64 || limit <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	guard := newImageMemoryGuard(imageMemoryCapacity(limit), maxBodyBytes)
	guard.runtimeLimit = limit - limit/10
	return guard.handler
}

func imageMemoryCapacity(limit int64) int64 { return limit - limit/4 }

type imageMemoryGuard struct {
	capacity     int64
	maxBody      int64
	bytes        *semaphore.Weighted
	lastLog      atomic.Int64
	runtimeLimit int64
	used         atomic.Int64
	active       atomic.Int64
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

var errImageMemoryBusy = errors.New("image memory capacity exhausted")

const imageMemoryLeaseKey = "image_request_memory_lease"

// During reading/parsing, allow for raw chunks, decoding and audit copies.
// Unknown sizes grow before allocation rather than reserving the entire pool.
func (g *imageMemoryGuard) weight(r *http.Request) int64 {
	size := r.ContentLength
	if size < 0 || (r.Header.Get("Content-Encoding") != "" && !strings.EqualFold(r.Header.Get("Content-Encoding"), "identity")) {
		size = 0
	}
	return imageMemoryWeight(size, 6, 1)
}

func imageMemoryWeight(size, multiplier, images int64) int64 {
	if size < 0 || size > (math.MaxInt64-imageRequestBaseMemory*images)/multiplier {
		return math.MaxInt64
	}
	return imageRequestBaseMemory*images + size*multiplier
}

type imageMemoryLease struct {
	guard  *imageMemoryGuard
	weight int64 // Owned by the synchronous request/middleware chain.
}

func (l *imageMemoryLease) resize(weight int64) bool {
	delta := weight - l.weight
	if delta > 0 {
		if l.guard.runtimeLimit > 0 && imageRuntimeMemory()+uint64(delta) >= uint64(l.guard.runtimeLimit) {
			return false
		}
		if !l.guard.bytes.TryAcquire(delta) {
			return false
		}
	} else if delta < 0 {
		l.guard.bytes.Release(-delta)
	}
	l.guard.used.Add(delta)
	l.weight = weight
	return true
}

// FinishImageMemoryPreparation releases temporary parse/audit headroom after
// those stages. Retain three input copies plus 32 MiB per output image, including
// while the client is gone. A multi-image increase must fit before any submit.
// bodyBytes includes separately retained Gemini relay JSON when present.
func FinishImageMemoryPreparation(c *gin.Context, bodyBytes int64, images int) bool {
	raw, ok := c.Get(imageMemoryLeaseKey)
	if !ok {
		return true
	}
	lease := raw.(*imageMemoryLease)
	if images < 1 || images > 10 {
		images = 1
	} // Invalid values use the existing parameter validator.
	output := imageUnbufferedOutputMemory * int64(images)
	weight := int64(math.MaxInt64)
	if bodyBytes >= 0 && bodyBytes <= (math.MaxInt64-output)/3 {
		weight = bodyBytes*3 + output
	}
	if !lease.resize(weight) {
		lease.guard.reject(c, "preparation", weight)
		return false
	}
	return true
}

// FinishImageRequestParsing retains inputs and small working buffers. Output
// memory must be reserved separately before an unbuffered upstream attempt.
// Buffered native attempts use this same weight after reserving physical disk.
func FinishImageRequestParsing(c *gin.Context, retainedBytes int64, images int) bool {
	raw, ok := c.Get(imageMemoryLeaseKey)
	if !ok {
		return true
	}
	if images < 1 || images > 10 {
		images = 1
	}
	lease := raw.(*imageMemoryLease)
	weight := imageMemoryWeight(retainedBytes, 1, int64(images))
	if !lease.resize(weight) {
		lease.guard.reject(c, "parsed_input", weight)
		return false
	}
	return true
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
	lease := &imageMemoryLease{guard: g}
	weight := g.weight(c.Request)
	if !lease.resize(weight) {
		g.reject(c, "admission", weight)
		return
	}
	g.active.Add(1)
	defer func() { lease.resize(0); g.active.Add(-1) }()
	if c.Request.Context().Err() != nil {
		c.Abort()
		return
	}
	attemptedWeight := lease.weight
	body, err := pkghttputil.ReadRequestBodyWithBudget(c.Request, func(wire, decoded int64) error {
		if err := c.Request.Context().Err(); err != nil {
			return err
		}
		required := imageMemoryWeight(wire+decoded, 6, 1)
		attemptedWeight = required
		if required > lease.weight && !lease.resize(required) {
			return errImageMemoryBusy
		}
		return nil
	})
	if err != nil {
		if c.Request.Context().Err() != nil {
			c.Abort()
			return
		}
		if errors.Is(err, errImageMemoryBusy) {
			g.reject(c, "body_read", attemptedWeight)
			return
		}
		status, message := http.StatusBadRequest, "Failed to read request body"
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			status, message = http.StatusRequestEntityTooLarge, "Request body is too large"
		}
		groupModelAllowlistErrorWriter(c)(c, status, message)
		c.Abort()
		return
	}
	// Drop compressed/wire and overdeclared length headroom after the exact
	// decoded body is available. Later readers reuse this immutable slice.
	if !lease.resize(imageMemoryWeight(int64(len(body)), 6, 1)) {
		g.reject(c, "body_complete", lease.weight)
		return
	}
	c.Request.Body = pkghttputil.NewPrereadBody(body)
	c.Set(imageMemoryLeaseKey, lease)
	c.Next()
}

func (g *imageMemoryGuard) reject(c *gin.Context, phase string, requested int64) {
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
		slog.Info("image_memory.capacity_rejected", "budget_bytes", g.capacity, "reserved_bytes", g.used.Load(), "requested_bytes", requested, "active_requests", g.active.Load(), "runtime_bytes", imageRuntimeMemory(), "phase", phase, "queue_enabled", false)
	}
}
