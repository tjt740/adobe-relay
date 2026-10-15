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
	unblock := make(chan struct{})
	var release sync.Once
	defer release.Do(func() { close(unblock) })
	var pending []chan int
	// Fill the actual pool instead of assuming one request consumes its old
	// 32 MiB reservation. Every admitted request holds its lease before reading.
	saturated := false
fill:
	for range 128 {
		body := &memoryRouteBody{Reader: strings.NewReader(payload), entered: make(chan struct{}), unblock: unblock}
		r := httptest.NewRequest("POST", "/v1/images/edits", body)
		r.ContentLength = int64(len(payload))
		r.Header.Set("Content-Type", "application/json")
		done := make(chan int, 1)
		go func() {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			done <- w.Code
		}()
		select {
		case <-body.entered:
			pending = append(pending, done)
		case code := <-done:
			require.Equal(t, 429, code)
			require.Zero(t, body.reads.Load(), "saturated route read a body before admission")
			saturated = true
			break fill
		case <-time.After(5 * time.Second):
			t.Fatal("request neither entered body reader nor was rejected immediately")
		}
	}
	require.True(t, saturated, "test failed to fill the image memory pool")
	require.NotEmpty(t, pending, "pool must admit requests before saturation")
	second := &memoryRouteBody{Reader: strings.NewReader(payload)}
	r2 := httptest.NewRequest("POST", "/images/generations", second)
	r2.ContentLength = int64(len(payload))
	r2.Header.Set("Content-Type", "application/json")
	rejected := httptest.NewRecorder()
	router.ServeHTTP(rejected, r2)
	require.Equal(t, 429, rejected.Code)
	require.Zero(t, second.reads.Load(), "alias read body before admitted requests released memory")
	release.Do(func() { close(unblock) })
	for _, done := range pending {
		select {
		case code := <-done:
			require.Equal(t, 404, code)
		case <-time.After(5 * time.Second):
			t.Fatal("admitted request did not complete after releasing its body reader")
		}
	}
	// The same request still reaches the original model policy after capacity returns.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r2)
	require.Equal(t, 404, w.Code)
	require.Positive(t, second.reads.Load())
}
