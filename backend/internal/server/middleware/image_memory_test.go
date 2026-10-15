package middleware

import (
	"bytes"
	"context"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type memoryTrackedReader struct {
	io.Reader
	reads atomic.Int64
}

func (r *memoryTrackedReader) Read(p []byte) (int, error) { r.reads.Add(1); return r.Reader.Read(p) }
func (*memoryTrackedReader) Close() error                 { return nil }

func imageMemoryRouter(g *imageMemoryGuard, platform string, h gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: platform}})
		c.Next()
	})
	r.Use(g.handler)
	for _, path := range []string{"/v1/images/edits", "/v1/images/edits/async", "/images/generations", "/v1beta/models/image:generateContent", "/v1/responses"} {
		r.POST(path, h)
	}
	return r
}

func memoryRequest(path, body string) (*http.Request, *memoryTrackedReader) {
	tracked := &memoryTrackedReader{Reader: strings.NewReader(body)}
	r := httptest.NewRequest("POST", path, tracked)
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Type", "application/json")
	return r, tracked
}

func TestImageMemoryRejectsImmediatelyAndRetainsSlotAfterDisconnect(t *testing.T) {
	g := newImageMemoryGuard(imageRequestBaseMemory+4096, 128<<20)
	entered := make(chan struct{})
	release := make(chan struct{})
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, `{"n":1}`, string(body))
		close(entered)
		<-release
		c.Status(200)
	})
	first, _ := memoryRequest("/v1/images/edits", `{"n":1}`)
	ctx, cancel := context.WithCancel(first.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { router.ServeHTTP(httptest.NewRecorder(), first.WithContext(ctx)); close(done) }()
	<-entered
	cancel() // Accepted generation can continue after a disconnected client.
	second, reader := memoryRequest("/images/generations", `{"n":1}`)
	w := httptest.NewRecorder()
	started := time.Now()
	router.ServeHTTP(w, second)
	require.Less(t, time.Since(started), time.Second)
	require.Equal(t, 429, w.Code)
	require.Equal(t, "1", w.Header().Get("Retry-After"))
	require.Zero(t, reader.reads.Load())
	require.False(t, g.bytes.TryAcquire(g.capacity), "slot must remain until handler returns")
	close(release)
	<-done
	require.True(t, g.bytes.TryAcquire(g.capacity))
	g.bytes.Release(g.capacity)
}

func TestImageMemoryCancellationAndRuntimePressureDoNotReadBodies(t *testing.T) {
	for _, mode := range []string{"cancel", "runtime_pressure"} {
		t.Run(mode, func(t *testing.T) {
			g := newImageMemoryGuard(imageRequestBaseMemory, 128<<20)
			router := imageMemoryRouter(g, service.PlatformAdobe, func(*gin.Context) { t.Error("downstream must not execute") })
			req, reader := memoryRequest("/v1/images/edits", "payload")
			if mode == "cancel" {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			} else {
				g.runtimeLimit = 1
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Zero(t, reader.reads.Load())
			if mode != "cancel" {
				require.Equal(t, 429, w.Code)
			}
			require.True(t, g.bytes.TryAcquire(g.capacity))
			g.bytes.Release(g.capacity)
		})
	}
}

func TestImageMemoryWeightsUnknownCompressedAndOverflowSafely(t *testing.T) {
	g := newImageMemoryGuard(512<<20, 128<<20)
	for _, tc := range []struct {
		length   int64
		encoding string
		want     int64
	}{
		{0, "", imageRequestBaseMemory}, {1 << 20, "", imageRequestBaseMemory + 6*(1<<20)},
		{-1, "", imageRequestBaseMemory}, {1, "gzip", imageRequestBaseMemory}, {math.MaxInt64, "", int64(math.MaxInt64)}, {1, "identity", imageRequestBaseMemory + 6},
	} {
		r := httptest.NewRequest("POST", "/v1/images/edits", nil)
		r.ContentLength = tc.length
		r.Header.Set("Content-Encoding", tc.encoding)
		require.Equal(t, tc.want, g.weight(r))
	}
}

func TestImageMemoryScopeAndExistingBodyLimit(t *testing.T) {
	for _, tc := range []struct {
		platform, path string
		guarded        bool
	}{
		{service.PlatformAdobe, "/v1/images/edits", true}, {service.PlatformComposite, "/images/generations", true},
		{service.PlatformAdobe, "/v1/images/edits/async", true},
		{service.PlatformAdobe, "/v1beta/models/image:generateContent", true},
		{service.PlatformOpenAI, "/v1/images/edits", false}, {service.PlatformAdobe, "/v1/responses", false},
	} {
		t.Run(tc.platform+tc.path, func(t *testing.T) {
			g := newImageMemoryGuard(imageRequestBaseMemory, 32)
			require.True(t, g.bytes.TryAcquire(g.capacity))
			defer g.bytes.Release(g.capacity)
			router := imageMemoryRouter(g, tc.platform, func(c *gin.Context) { c.Status(204) })
			req, reader := memoryRequest(tc.path, "small")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if tc.guarded {
				require.Equal(t, 429, w.Code)
			} else {
				require.Equal(t, 204, w.Code)
			}
			require.Zero(t, reader.reads.Load())
			if tc.guarded {
				req, reader = memoryRequest(tc.path, strings.Repeat("x", 33))
				w = httptest.NewRecorder()
				router.ServeHTTP(w, req)
				require.Equal(t, 413, w.Code)
				require.Zero(t, reader.reads.Load())
			}
		})
	}
}

func TestImageMemoryKeepsConfiguredGoLimit(t *testing.T) {
	old := debug.SetMemoryLimit(1536 << 20)
	defer debug.SetMemoryLimit(old)
	_ = ImageRequestMemoryBudget(128 << 20)
	require.EqualValues(t, 1536<<20, debug.SetMemoryLimit(-1))
}

// Reproduce the real failure path: large uploads overlap slow upstream work.
// Rejected requests must never read the body; accepted requests still receive
// byte-identical uploads and audit input, and no new queue retains callers.
func TestImageMemoryConcurrentLargeMultipartRemainsBounded(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	require.NoError(t, w.WriteField("model", "gpt-image-2.5-sunburst"))
	require.NoError(t, w.WriteField("prompt", "local memory regression"))
	part, err := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {`form-data; name="image"; filename="reference.png"`}, "Content-Type": {"image/png"}})
	require.NoError(t, err)
	image := bytes.Repeat([]byte{0x77}, 4<<20)
	copy(image, []byte("\x89PNG\r\n\x1a\n"))
	_, err = part.Write(image)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	payload := body.Bytes()
	contentType := w.FormDataContentType()
	g := newImageMemoryGuard(128<<20, 128<<20)
	entered := make(chan struct{}, 40)
	release := make(chan struct{})
	var active, peak atomic.Int64
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		n := active.Add(1)
		defer active.Add(-1)
		for p := peak.Load(); n > p; p = peak.Load() {
			if peak.CompareAndSwap(p, n) {
				break
			}
		}
		raw, e := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		require.NoError(t, e)
		require.Equal(t, payload, raw)
		parsed, e := (&service.OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, raw)
		require.NoError(t, e)
		require.Len(t, parsed.Uploads, 1)
		require.Equal(t, image, parsed.Uploads[0].Data)
		audit := parsed.ModerationBody()
		entered <- struct{}{}
		<-release
		runtime.KeepAlive(parsed)
		runtime.KeepAlive(audit)
		c.Status(200)
	})
	var wg sync.WaitGroup
	rejected := make(chan struct{}, 32)
	readers := make([]*memoryTrackedReader, 32)
	for i := range readers {
		readers[i] = &memoryTrackedReader{Reader: bytes.NewReader(payload)}
		r := httptest.NewRequest("POST", "/v1/images/edits", readers[i])
		r.ContentLength = int64(len(payload))
		r.Header.Set("Content-Type", contentType)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := httptest.NewRecorder()
			router.ServeHTTP(result, r)
			if result.Code == 429 {
				rejected <- struct{}{}
			} else {
				require.Equal(t, 200, result.Code)
			}
		}()
	}
	<-entered
	<-entered
	require.Eventually(t, func() bool { return len(rejected) == 30 }, 3*time.Second, time.Millisecond)
	read := 0
	for _, r := range readers {
		if r.reads.Load() > 0 {
			read++
		}
	}
	require.Equal(t, 2, read, "rejected requests must not allocate/parse images")
	require.EqualValues(t, 2, peak.Load())
	close(release)
	wg.Wait()
	require.True(t, g.bytes.TryAcquire(g.capacity))
	g.bytes.Release(g.capacity)
}
