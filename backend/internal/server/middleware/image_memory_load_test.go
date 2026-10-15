package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Synthetic, network-free cgroup replay. Includes the body, audit encoding,
// overlapping generation and a simultaneous output-encoding burst.
func TestImageMemoryAdaptiveCgroupReplay(t *testing.T) {
	mode := os.Getenv("ADOBE_ADAPTIVE_REPLAY")
	if mode != "large" && mode != "small" && mode != "compressed" {
		t.Skip("isolated cgroup replay only")
	}
	total := 192
	size := 1024
	if mode == "large" {
		total = 48
		size = 40 << 20
	}
	if mode == "compressed" {
		total = 64
		size = 4 << 20
	}
	if n, err := strconv.Atoi(os.Getenv("ADOBE_REPLAY_REQUESTS")); err == nil && n > 0 && n <= 512 {
		total = n
	}
	payload := bytes.Repeat([]byte("x"), size)
	wire := payload
	if mode == "compressed" {
		var b bytes.Buffer
		z := gzip.NewWriter(&b)
		_, err := z.Write(payload)
		require.NoError(t, err)
		require.NoError(t, z.Close())
		wire = b.Bytes()
	}
	g := newImageMemoryGuard(debug.SetMemoryLimit(-1)/3*2, 64<<20)
	g.runtimeLimit = debug.SetMemoryLimit(-1) * 9 / 10
	var active, peak, ok, rejected atomic.Int64
	var maxHeap atomic.Uint64
	entered := make(chan struct{}, total)
	release := make(chan struct{})
	outputReady := make(chan struct{}, total)
	outputRelease := make(chan struct{})
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		raw, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		require.NoError(t, err)
		require.True(t, bytes.Equal(payload, raw))
		// Match the temporary audit expansion without retaining it during generation.
		func() {
			audit := base64.StdEncoding.EncodeToString(raw)
			require.NotEmpty(t, audit)
			runtime.KeepAlive(audit)
		}()
		if !FinishImageMemoryPreparation(c, int64(len(raw)), 1) {
			return
		}
		n := active.Add(1)
		defer active.Add(-1)
		for p := peak.Load(); n > p; p = peak.Load() {
			if peak.CompareAndSwap(p, n) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		// All accepted requests finish together: raw output + base64 + JSON copy.
		output := bytes.Repeat([]byte("y"), 4<<20)
		encoded := base64.StdEncoding.EncodeToString(output)
		envelope := []byte(encoded)
		outputReady <- struct{}{}
		<-outputRelease
		runtime.KeepAlive(raw)
		runtime.KeepAlive(output)
		runtime.KeepAlive(encoded)
		runtime.KeepAlive(envelope)
		c.Status(200)
	})
	stop := make(chan struct{})
	sampleDone := make(chan struct{})
	go func() {
		defer close(sampleDone)
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
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
	for i := 0; i < total; i++ {
		r := httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(wire))
		if mode == "compressed" {
			r.ContentLength = -1
			r.Body = io.NopCloser(bytes.NewReader(wire))
			r.Header.Set("Content-Encoding", "gzip")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			switch w.Code {
			case 200:
				ok.Add(1)
			case 429:
				rejected.Add(1)
			default:
				t.Errorf("unexpected status %d", w.Code)
			}
		}()
	}
	// Every request must either enter or fail promptly, without a waiting queue.
	require.Eventually(t, func() bool { return int64(len(entered))+rejected.Load() == int64(total) }, 45*time.Second, 10*time.Millisecond)
	expectedOutputs := len(entered)
	close(release)
	require.Eventually(t, func() bool { return len(outputReady) == expectedOutputs }, 60*time.Second, 10*time.Millisecond)
	var held runtime.MemStats
	runtime.ReadMemStats(&held)
	for p := maxHeap.Load(); held.HeapAlloc > p; p = maxHeap.Load() {
		if maxHeap.CompareAndSwap(p, held.HeapAlloc) {
			break
		}
	}
	close(outputRelease)
	wg.Wait()
	close(stop)
	<-sampleDone
	require.Positive(t, ok.Load())
	require.EqualValues(t, total, ok.Load()+rejected.Load())
	require.Zero(t, g.used.Load())
	require.Zero(t, g.active.Load())
	if mode == "small" {
		require.GreaterOrEqual(t, ok.Load(), int64(120), "regression: low-memory calls rejected by the old 64-request ceiling")
	}
	fmt.Printf("adaptive_replay mode=%s go_limit=%d success=%d rejected=%d peak_active=%d peak_heap_mib=%d\n", mode, debug.SetMemoryLimit(-1), ok.Load(), rejected.Load(), peak.Load(), maxHeap.Load()>>20)
	if raw, err := os.ReadFile("/sys/fs/cgroup/memory.peak"); err == nil {
		fmt.Printf("cgroup_memory_peak_bytes=%s", raw)
	}
}
