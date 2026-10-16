package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

func TestImageMemoryUnknownAndCompressedShareBudget(t *testing.T) {
	for _, encoding := range []string{"chunked", "gzip", "zstd"} {
		t.Run(encoding, func(t *testing.T) {
			payload := []byte(strings.Repeat("x", 1<<20))
			wire := payload
			if encoding == "gzip" {
				var b bytes.Buffer
				z := gzip.NewWriter(&b)
				_, err := z.Write(payload)
				require.NoError(t, err)
				require.NoError(t, z.Close())
				wire = b.Bytes()
			}
			if encoding == "zstd" {
				encoder, err := zstd.NewWriter(nil)
				require.NoError(t, err)
				wire = encoder.EncodeAll(payload, nil)
				encoder.Close()
			}
			g := newImageMemoryGuard(1<<30, 128<<20)
			entered := make(chan struct{}, 8)
			release := make(chan struct{})
			router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
				body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
				require.NoError(t, err)
				require.Equal(t, payload, body)
				require.True(t, FinishImageMemoryPreparation(c, int64(len(body)), 1))
				entered <- struct{}{}
				<-release
				c.Status(200)
			})
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				r := httptest.NewRequest("POST", "/v1/images/edits", io.NopCloser(bytes.NewReader(wire)))
				r.ContentLength = -1
				if encoding != "chunked" {
					r.Header.Set("Content-Encoding", encoding)
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					w := httptest.NewRecorder()
					router.ServeHTTP(w, r)
					require.Equal(t, 200, w.Code, w.Body.String())
				}()
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					close(release)
					wg.Wait()
					t.Fatal("unknown length upload monopolized capacity")
				}
			}
			require.EqualValues(t, 8, g.active.Load())
			require.Less(t, g.used.Load(), g.capacity)
			close(release)
			wg.Wait()
			require.Zero(t, g.used.Load())
			require.Zero(t, g.active.Load())
		})
	}
}

func TestImageMemoryReleasesPreparationHeadroomBeforeSlowGeneration(t *testing.T) {
	g := newImageMemoryGuard(232<<20, 128<<20)
	payload := bytes.Repeat([]byte("x"), 8<<20)
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var wg sync.WaitGroup
	// Each request retains 2 input copies plus 32 MiB output; the raw copy is mapped.
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		require.NoError(t, err)
		require.True(t, FinishImageMemoryPreparation(c, int64(len(body)), 1))
		entered <- struct{}{}
		<-release
		c.Status(200)
	})
	for i := 0; i < 4; i++ {
		r := httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(payload))
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			require.Equal(t, 200, w.Code)
		}()
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			close(release)
			wg.Wait()
			t.Fatal("temporary headroom was retained")
		}
	}
	require.EqualValues(t, 192<<20, g.used.Load())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(payload))
	router.ServeHTTP(w, r)
	require.Equal(t, 429, w.Code)
	close(release)
	wg.Wait()
	require.Zero(t, g.used.Load())
}

func TestImageMemoryGrowingBodiesStopBeforeDownstream(t *testing.T) {
	for _, mode := range []string{"chunked", "underdeclared", "compressed"} {
		t.Run(mode, func(t *testing.T) {
			payload := bytes.Repeat([]byte("x"), 8<<20)
			wire := payload
			if mode == "compressed" {
				var b bytes.Buffer
				z := gzip.NewWriter(&b)
				_, err := z.Write(payload)
				require.NoError(t, err)
				require.NoError(t, z.Close())
				wire = b.Bytes()
			}
			reader := &memoryTrackedReader{Reader: bytes.NewReader(wire)}
			r := httptest.NewRequest("POST", "/v1/images/edits", reader)
			r.ContentLength = -1
			if mode == "underdeclared" {
				r.ContentLength = 1
			}
			if mode == "compressed" {
				r.Header.Set("Content-Encoding", "gzip")
			}
			g := newImageMemoryGuard(imageRequestBaseMemory+6*(1<<20), 128<<20)
			router := imageMemoryRouter(g, service.PlatformAdobe, func(*gin.Context) { t.Error("partial/over-budget body reached downstream") })
			w := httptest.NewRecorder()
			started := time.Now()
			router.ServeHTTP(w, r)
			require.Equal(t, 429, w.Code, w.Body.String())
			require.Equal(t, "1", w.Header().Get("Retry-After"))
			require.Less(t, time.Since(started), time.Second)
			require.Zero(t, g.used.Load())
			require.Zero(t, g.active.Load())
			if mode != "compressed" {
				require.Greater(t, reader.Reader.(*bytes.Reader).Len(), 0, "reader consumed the entire over-budget upload")
			}
		})
	}
}

func TestImageMemoryMultiImageReservationPrecedesGeneration(t *testing.T) {
	g := newImageMemoryGuard(128<<20, 128<<20)
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		require.False(t, FinishImageMemoryPreparation(c, 0, 10))
		require.True(t, c.IsAborted())
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/v1/images/edits", strings.NewReader(`{"n":10}`)))
	require.Equal(t, 429, w.Code)
	require.Zero(t, g.used.Load())
	require.Zero(t, g.active.Load())
}

func TestImageMemoryReadFailuresAndPanicsReleaseReservation(t *testing.T) {
	for _, mode := range []string{"invalid-gzip", "oversize-chunked", "panic"} {
		t.Run(mode, func(t *testing.T) {
			g := newImageMemoryGuard(128<<20, 16)
			router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) { panic("handler failure") })
			r := httptest.NewRequest("POST", "/v1/images/edits", strings.NewReader("not gzip or a valid bounded body"))
			r.ContentLength = -1
			if mode == "invalid-gzip" {
				r.Header.Set("Content-Encoding", "gzip")
			}
			w := httptest.NewRecorder()
			if mode == "oversize-chunked" {
				r.Body = http.MaxBytesReader(w, r.Body, 16)
			}
			if mode == "panic" {
				require.Panics(t, func() { router.ServeHTTP(w, r) })
			} else {
				router.ServeHTTP(w, r)
				require.Contains(t, []int{400, 413}, w.Code)
			}
			require.Zero(t, g.used.Load())
			require.Zero(t, g.active.Load())
		})
	}
}

// Closing an HTTP/1 request body may drain a slow upload. The HTTP server owns
// that close; capacity rejection must write its response without draining first.
type closeBlockingImageBody struct {
	io.Reader
	closed chan struct{}
}

func (b *closeBlockingImageBody) Close() error { <-b.closed; return nil }
func TestImageMemoryRejectionDoesNotDrainSlowBody(t *testing.T) {
	g := newImageMemoryGuard(imageRequestBaseMemory+4096, 128<<20)
	body := &closeBlockingImageBody{Reader: strings.NewReader(strings.Repeat("x", 8192)), closed: make(chan struct{})}
	defer close(body.closed)
	r := httptest.NewRequest("POST", "/v1/images/edits", body)
	r.ContentLength = -1
	router := imageMemoryRouter(g, service.PlatformAdobe, func(*gin.Context) { t.Error("over-budget body reached downstream") })
	done := make(chan struct{})
	w := httptest.NewRecorder()
	go func() { router.ServeHTTP(w, r); close(done) }()
	select {
	case <-done:
		require.Equal(t, 429, w.Code)
		require.Zero(t, g.used.Load())
	case <-time.After(time.Second):
		t.Fatal("capacity response waited for body drain")
	}
}
