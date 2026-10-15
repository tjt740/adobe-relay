package middleware

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Run only inside the isolated 2 GiB container from image-memory-stress.sh.
// The legacy control deliberately permits the old OOM and must never run on
// the production process. Neither variant performs any network or database I/O.
func TestImageMemoryCgroupReplay(t *testing.T) {
	mode := os.Getenv("ADOBE_MEMORY_REPLAY")
	if mode != "guarded" && mode != "legacy" {
		t.Skip("isolated cgroup replay only")
	}
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	require.NoError(t, w.WriteField("model", "gpt-image-2.5-sunburst"))
	require.NoError(t, w.WriteField("prompt", "synthetic memory replay"))
	image := bytes.Repeat([]byte{0x55}, 20<<20)
	copy(image, []byte("\x89PNG\r\n\x1a\n"))
	for i := 0; i < 2; i++ {
		part, err := w.CreatePart(textproto.MIMEHeader{"Content-Disposition": {fmt.Sprintf(`form-data; name="image[]"; filename="%d.png"`, i)}, "Content-Type": {"image/png"}})
		require.NoError(t, err)
		_, err = part.Write(image)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	payload := body.Bytes()
	contentType := w.FormDataContentType()
	g := newImageMemoryGuard(debug.SetMemoryLimit(-1)/3, 256<<20)
	var peak, active, success, rejected atomic.Int64
	var maxHeap atomic.Uint64
	handler := func(c *gin.Context) {
		n := active.Add(1)
		defer active.Add(-1)
		for p := peak.Load(); n > p; p = peak.Load() {
			if peak.CompareAndSwap(p, n) {
				break
			}
		}
		raw, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		require.NoError(t, err)
		require.True(t, bytes.Equal(payload, raw))
		parsed, err := (&service.OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, raw)
		require.NoError(t, err)
		require.Len(t, parsed.Uploads, 2)
		for _, upload := range parsed.Uploads {
			require.True(t, bytes.Equal(image, upload.Data))
		}
		audit := parsed.ModerationBody()
		require.Greater(t, len(audit), len(payload))
		time.Sleep(100 * time.Millisecond) // In-flight upstream/user-slot waiting.
		runtime.KeepAlive(parsed)
		runtime.KeepAlive(audit)
		c.Status(200)
	}
	platform := service.PlatformAdobe
	if mode == "legacy" {
		platform = service.PlatformOpenAI
	} // Skip only this memory guard.
	router := imageMemoryRouter(g, platform, handler)
	stop := make(chan struct{})
	sampleDone := make(chan struct{})
	go func() {
		defer close(sampleDone)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				for p := maxHeap.Load(); m.HeapAlloc > p; p = maxHeap.Load() {
					if maxHeap.CompareAndSwap(p, m.HeapAlloc) {
						break
					}
				}
			}
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < 48; i++ {
		r := httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(payload))
		r.Header.Set("Content-Type", contentType)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			switch w.Code {
			case 200:
				success.Add(1)
			case 429:
				rejected.Add(1)
			default:
				t.Errorf("unexpected status %d", w.Code)
			}
		}()
	}
	wg.Wait()
	close(stop)
	<-sampleDone
	fmt.Printf("memory_replay mode=%s success=%d rejected=%d max_active=%d peak_heap_mib=%d\n", mode, success.Load(), rejected.Load(), peak.Load(), maxHeap.Load()>>20)
	if raw, err := os.ReadFile("/sys/fs/cgroup/memory.peak"); err == nil {
		fmt.Printf("cgroup_memory_peak_bytes=%s", raw)
	}
	if mode == "guarded" {
		require.LessOrEqual(t, peak.Load(), g.capacity/g.weight(httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(payload))))
		require.Positive(t, rejected.Load())
		require.Positive(t, success.Load())
		require.EqualValues(t, 48, success.Load()+rejected.Load())
		require.True(t, g.bytes.TryAcquire(g.capacity))
		g.bytes.Release(g.capacity)
	}
}
