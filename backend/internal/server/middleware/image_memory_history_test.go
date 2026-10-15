package middleware

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/imagebuffer"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type historyTrace struct {
	Requests []struct {
		OffsetMS         int64 `json:"offset_ms"`
		AuditBytes       int64 `json:"audit_bytes"`
		DurationMS       int64 `json:"duration_ms"`
		DurationObserved bool  `json:"duration_observed"`
		ImageCount       int   `json:"image_count"`
	} `json:"requests"`
}
type historyFill struct{ remaining int64 }

func (r *historyFill) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.remaining {
		n = int(r.remaining)
	}
	for i := 0; i < n; i++ {
		p[i] = byte(i % 251)
	}
	r.remaining -= int64(n)
	return n, nil
}

// Real read/parse/audit plus simultaneous JSON image output, using only size
// metadata from the afternoon production trace; never contacts Adobe or a DB.
func TestImageMemoryHistoricalBurst(t *testing.T) {
	path := os.Getenv("ADOBE_HISTORY_TRACE")
	if path == "" {
		t.Skip("explicit isolated historical replay only")
	}
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var trace historyTrace
	require.NoError(t, json.Unmarshal(b, &trace))
	require.Greater(t, len(trace.Requests), 400)
	outputMB := 4
	if n, e := strconv.Atoi(os.Getenv("ADOBE_HISTORY_OUTPUT_MIB")); e == nil && n > 0 {
		outputMB = n
	}
	paced := os.Getenv("ADOBE_HISTORY_PACED") == "1"
	timeScale := 0.05
	if scale, err := strconv.ParseFloat(os.Getenv("ADOBE_HISTORY_TIME_SCALE"), 64); err == nil && scale > 0 {
		timeScale = scale
	}
	compact := os.Getenv("ADOBE_HISTORY_COMPACT_RESERVATION") == "1"
	mixed := os.Getenv("ADOBE_HISTORY_MIXED") == "1"
	totals := []int{100, 200, 300, 400}
	if count, e := strconv.Atoi(os.Getenv("ADOBE_HISTORY_TOTAL")); e == nil && count > 0 {
		totals = []int{count}
	}
	if paced {
		totals = []int{len(trace.Requests)}
	}
	for _, total := range totals {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			g := newImageMemoryGuard(imageMemoryCapacity(debug.SetMemoryLimit(-1)), 256<<20)
			g.runtimeLimit = debug.SetMemoryLimit(-1) * 9 / 10
			t.Logf("historical_budget go_limit_bytes=%d image_budget_bytes=%d runtime_pressure_bytes=%d", debug.SetMemoryLimit(-1), g.capacity, g.runtimeLimit)
			var arrived, accepted, rejected, active, peak atomic.Int64
			var inputTotal atomic.Int64
			release := make(chan struct{})
			outputs := make(chan struct{}, total)
			finish := make(chan struct{})
			router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
				raw, e := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
				if e != nil {
					t.Error(e)
					c.Status(500)
					arrived.Add(1)
					return
				}
				parsed, e := (&service.OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, raw)
				if e != nil {
					t.Error(e)
					c.Status(500)
					arrived.Add(1)
					return
				}
				func() {
					audit := parsed.ModerationBody()
					if len(audit) == 0 {
						t.Error("missing actual audit body")
					}
					runtime.KeepAlive(audit)
				}()
				if !FinishImageRequestParsing(c, service.AdobeImageRetainedBytes(parsed), parsed.N) {
					return
				}
				legacy := c.GetHeader("X-Replay-Legacy") == "1"
				bufferCtx := c.Request.Context()
				cleanup := func() {}
				buffered := false
				if legacy {
					if !FinishImageMemoryPreparation(c, int64(len(raw)), parsed.N) {
						return
					}
				} else if compact {
					// CI reserves the known synthetic output bound, keeping
					// identical mapped IO while avoiding 160 MiB of unused
					// physical disk per tiny fake image on limited runners.
					a, err := imagebuffer.New(int64(parsed.N) * int64((outputMB*3+1)<<20))
					buffered = err == nil
					cleanup = func() {
						if a != nil {
							_ = a.Close()
						}
					}
					bufferCtx = imagebuffer.WithArena(c.Request.Context(), a)
				} else {
					bufferCtx, cleanup, buffered = service.PrepareAdobeImageBuffer(c.Request.Context(), parsed)
				}
				defer cleanup()
				if !legacy && !buffered {
					t.Error("isolated replay needs disk buffering")
					c.Status(500)
					arrived.Add(1)
					return
				}
				arena := imagebuffer.FromContext(bufferCtx)
				n := active.Add(1)
				defer active.Add(-1)
				for p := peak.Load(); n > p; p = peak.Load() {
					if peak.CompareAndSwap(p, n) {
						break
					}
				}
				accepted.Add(1)
				inputTotal.Add(int64(len(raw)))
				arrived.Add(1)
				if paced {
					d, _ := strconv.ParseInt(c.GetHeader("X-Replay-Duration-Ms"), 10, 64)
					time.Sleep(time.Duration(float64(d)*timeScale) * time.Millisecond)
				} else {
					<-release
				}
				images := make([][]byte, parsed.N)
				for i := range images {
					readLimit := int64(64 << 20)
					if compact {
						readLimit = int64(outputMB << 20)
					}
					if legacy {
						images[i], e = io.ReadAll(&historyFill{int64(outputMB << 20)})
					} else {
						images[i], e = arena.Read(&historyFill{int64(outputMB << 20)}, readLimit)
					}
					if e != nil {
						t.Error(e)
						break
					}
				}
				var body []byte
				if legacy {
					items := make([]map[string]string, len(images))
					for i, img := range images {
						items[i] = map[string]string{"b64_json": base64.StdEncoding.EncodeToString(img)}
					}
					body, e = json.Marshal(map[string]any{"created": 1, "data": items})
				} else {
					body, e = imagebuffer.OpenAIResponse(arena, map[string]any{"created": 1}, images)
				}
				if e != nil {
					t.Error(e)
				}

				outputs <- struct{}{}
				if !paced {
					<-finish
				}
				runtime.KeepAlive(raw)
				runtime.KeepAlive(parsed)
				runtime.KeepAlive(images)
				runtime.KeepAlive(body)
				c.Status(200)
			})
			var wg sync.WaitGroup
			replayStart := time.Now()
			for i := 0; i < total; i++ {
				// Deterministic evenly spaced samples retain the measured long tail.
				sample := trace.Requests[i*len(trace.Requests)/total]
				if paced {
					time.Sleep(time.Until(replayStart.Add(time.Duration(float64(sample.OffsetMS)*timeScale) * time.Millisecond)))
				}
				size := sample.AuditBytes // conservative: normalized audit size as raw upload size
				if size < 1024 {
					size = 1024
				}
				n := sample.ImageCount
				if n < 1 {
					n = 1
				}
				prefix := fmt.Sprintf("--hist\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\ngpt-image-2.5-sunburst\r\n--hist\r\nContent-Disposition: form-data; name=\"prompt\"\r\n\r\nhistorical size-only replay\r\n--hist\r\nContent-Disposition: form-data; name=\"n\"\r\n\r\n%d\r\n--hist\r\nContent-Disposition: form-data; name=\"image\"; filename=\"reference.png\"\r\nContent-Type: image/png\r\n\r\n", n)
				suffix := "\r\n--hist--\r\n"
				reader := io.MultiReader(strings.NewReader(prefix), &historyFill{size}, strings.NewReader(suffix))
				req := httptest.NewRequest("POST", "/v1/images/edits", reader)
				req.Header.Set("X-Replay-Duration-Ms", strconv.FormatInt(sample.DurationMS, 10))
				req.Header.Set("Content-Type", "multipart/form-data; boundary=hist")
				if mixed && i%40 < 7 { // observed 17.5% requested explicit transcoding
					req.Header.Set("X-Replay-Legacy", "1")
				}
				req.ContentLength = int64(len(prefix)+len(suffix)) + size
				if i%10 == 0 {
					req.ContentLength = -1
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)
					if w.Code == 429 {
						rejected.Add(1)
						arrived.Add(1)
					} else if w.Code != 200 {
						t.Errorf("unexpected response %d", w.Code)
					}
				}()
			}
			ready := func() bool { return arrived.Load() == int64(total) }
			deadline := time.Now().Add(60 * time.Second)
			for !ready() && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if !ready() {
				close(release)
				close(finish)
				wg.Wait()
				t.Fatal("requests did not enter or reject promptly")
			}
			close(release)
			deadline = time.Now().Add(90 * time.Second)
			for len(outputs) < int(accepted.Load()) && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("historical_burst requested=%d accepted=%d rejected=%d peak=%d input_mib=%.2f output_per_image_mib=%d held_heap_mib=%.2f reserved_mib=%.2f\n", total, accepted.Load(), rejected.Load(), peak.Load(), float64(inputTotal.Load())/(1<<20), outputMB, float64(m.HeapAlloc)/(1<<20), float64(g.used.Load())/(1<<20))
			all := len(outputs) == int(accepted.Load())
			close(finish)
			wg.Wait()
			require.True(t, all, "outputs did not complete")
			require.Zero(t, g.used.Load())
			require.Zero(t, g.active.Load())
			runtime.GC()
		})
	}
}
