package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stagedSlowBody struct {
	io.Reader
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (b *stagedSlowBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return b.Reader.Read(p)
}
func (*stagedSlowBody) Close() error { return nil }

func TestImageMemorySlowDeclaredUploadsReserveOnlyArrivedChunks(t *testing.T) {
	g := newImageMemoryGuard(64<<20, 128<<20)
	unblock := make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(unblock) })
	var wg sync.WaitGroup
	results := make(chan int, 4)
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		raw, e := io.ReadAll(c.Request.Body)
		if e != nil || string(raw) != "small actual body" {
			c.Status(500)
			return
		}
		c.Status(200)
	})
	for range 4 {
		body := &stagedSlowBody{Reader: strings.NewReader("small actual body"), entered: make(chan struct{}), release: unblock}
		req := httptest.NewRequest("POST", "/v1/images/edits", body)
		req.ContentLength = 80 << 20
		wg.Add(1)
		go func() { defer wg.Done(); w := httptest.NewRecorder(); router.ServeHTTP(w, req); results <- w.Code }()
		select {
		case <-body.entered:
		case code := <-results:
			t.Fatalf("declared bytes monopolized budget before reading: %d", code)
		case <-time.After(3 * time.Second):
			t.Fatal("slow upload failed to start immediately")
		}
	}
	require.EqualValues(t, 16<<20, g.used.Load(), "disk-backed uploads hold only bounded IO working memory before parse admission")
	// These read leases remain bounded: no queue and no body reads when full.
	free := g.capacity - g.used.Load()
	require.True(t, g.bytes.TryAcquire(free))
	req, reader := memoryRequest("/images/generations", "small")
	w := httptest.NewRecorder()
	started := time.Now()
	router.ServeHTTP(w, req)
	require.Equal(t, 429, w.Code)
	require.Zero(t, reader.reads.Load())
	require.Less(t, time.Since(started), time.Second)
	g.bytes.Release(free)
	release.Do(func() { close(unblock) })
	wg.Wait()
	for range 4 {
		require.Equal(t, 200, <-results)
	}
	require.Zero(t, g.used.Load())
	require.Zero(t, g.active.Load())
}

func TestImageMemoryParsedMultipleImagesRetainOnlyWorkingBuffers(t *testing.T) {
	g := newImageMemoryGuard(64<<20, 128<<20)
	lease := &imageMemoryLease{guard: g}
	defer lease.resize(0)
	require.True(t, lease.resize(60<<20))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", nil)
	c.Set(imageMemoryLeaseKey, lease)
	require.True(t, FinishImageRequestParsing(c, 2<<20, 10))
	require.EqualValues(t, 12<<20, g.used.Load())
	// Relay/transcode still needs the full output allowance before submission.
	require.False(t, FinishImageMemoryPreparation(c, 2<<20, 10))
	require.True(t, c.IsAborted())
	require.EqualValues(t, 12<<20, g.used.Load())
	require.True(t, lease.resize(0))
	require.Zero(t, g.used.Load())
}

func TestImageMemoryReusesFreeHeapButStillRejectsLivePressure(t *testing.T) {
	g := newImageMemoryGuard(128<<20, 128<<20)
	g.runtimeLimit = 100 << 20
	memory := imageRuntimeFootprint{committed: 110 << 20, reusable: 64 << 20}
	g.runtimeUsage = func() imageRuntimeFootprint { return memory }
	lease := &imageMemoryLease{guard: g}
	defer lease.resize(0)
	// A previous burst left committed pages, but these spans are fully free.
	// Growing within them need not increase the resident footprint.
	require.True(t, lease.resize(32<<20))
	require.EqualValues(t, 32<<20, g.used.Load())
	// As the spans become occupied, the same 90% pressure guard applies.
	memory.reusable = 32 << 20
	require.False(t, lease.resize(60<<20))
	require.EqualValues(t, 32<<20, lease.weight)
	require.True(t, lease.resize(0))
	memory.reusable = 0
	require.False(t, lease.resize(1<<20), "live memory pressure cannot borrow pool headroom")
	// Never turn inconsistent metrics into an unsigned underflow/free budget.
	memory.reusable = 120 << 20
	require.False(t, lease.resize(1<<20))
	require.Zero(t, g.used.Load())
}
