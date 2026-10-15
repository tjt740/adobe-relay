package routes

import (
	"io"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type memoryRouteBody struct {
	io.Reader
	entered chan struct{}
	unblock chan struct{}
	once    sync.Once
	reads   atomic.Int64
}

func (r *memoryRouteBody) Read(p []byte) (int, error) {
	r.reads.Add(1)
	if r.entered != nil {
		r.once.Do(func() { close(r.entered) })
		<-r.unblock
	}
	return r.Reader.Read(p)
}
func (*memoryRouteBody) Close() error { return nil }

func TestImageMemoryBudgetPrecedesAllowlistBodyReadAndSharesAliasCapacity(t *testing.T) {
	old := debug.SetMemoryLimit(96 << 20)
	defer debug.SetMemoryLimit(old)
	group := allowlistGroup(service.PlatformAdobe, true, "gpt-image-2")
	router := newGatewayRoutesTestRouterWithGroup(group)
	payload := `{"model":"blocked"}`
	first := &memoryRouteBody{Reader: strings.NewReader(payload), entered: make(chan struct{}), unblock: make(chan struct{})}
	r := httptest.NewRequest("POST", "/v1/images/edits", first)
	r.ContentLength = int64(len(payload))
	r.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() {
		defer close(done)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		require.Equal(t, 404, w.Code)
	}()
	select {
	case <-first.entered:
	case <-time.After(time.Second):
		t.Fatal("first request never reached allowlist reader")
	}
	second := &memoryRouteBody{Reader: strings.NewReader(payload)}
	r2 := httptest.NewRequest("POST", "/images/generations", second)
	r2.ContentLength = int64(len(payload))
	r2.Header.Set("Content-Type", "application/json")
	rejected := httptest.NewRecorder()
	router.ServeHTTP(rejected, r2)
	require.Equal(t, 429, rejected.Code)
	require.Zero(t, second.reads.Load(), "alias read body before the first request released memory")
	close(first.unblock)
	<-done
	// The same request still reaches the original model policy after capacity returns.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r2)
	require.Equal(t, 404, w.Code)
	require.Positive(t, second.reads.Load())
}
