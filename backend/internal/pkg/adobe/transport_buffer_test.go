//go:build unit

package adobe

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/imagebuffer"
	"github.com/stretchr/testify/require"
)

func TestPlainImageDownloadUsesBoundedDiskAndRetainsHeaders(t *testing.T) {
	allowLoopbackDownloads(t)
	raw := bytes.Repeat([]byte("image data"), 10000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(raw)
	}))
	defer server.Close()
	a, err := imagebuffer.New(160 << 20)
	require.NoError(t, err)
	defer a.Close()
	ctx := imagebuffer.WithArena(context.Background(), a)
	res, err := NewPlainTransport("").Do(ctx, &Request{Method: "GET", URL: server.URL, MaxBodyBytes: MaxImageDownloadBytes})
	require.NoError(t, err)
	require.Equal(t, raw, res.Body)
	require.Equal(t, "image/png", res.Header("content-type"))
	// A second image cannot silently allocate on the heap after disk quota is used.
	_, err = a.Read(io.LimitReader(&endlessZeroReader{}, 100<<20), 100<<20)
	require.ErrorIs(t, err, imagebuffer.ErrCapacity)
}

type endlessZeroReader struct{}

func (*endlessZeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestPlainBufferedDownloadRejectsOversizeAndReusesFailedExtent(t *testing.T) {
	allowLoopbackDownloads(t)
	var tooLarge atomic.Bool
	tooLarge.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tooLarge.Load() {
			_, _ = io.CopyN(w, &endlessZeroReader{}, MaxImageDownloadBytes+1)
		} else {
			_, _ = w.Write([]byte("image"))
		}
	}))
	defer server.Close()
	a, err := imagebuffer.New(160 << 20)
	require.NoError(t, err)
	defer a.Close()
	ctx := imagebuffer.WithArena(context.Background(), a)
	transport := NewPlainTransport("")
	for i := 0; i < 3; i++ {
		_, err = transport.Do(ctx, &Request{Method: "GET", URL: server.URL, MaxBodyBytes: MaxImageDownloadBytes})
		var requestErr *RequestError
		require.ErrorAs(t, err, &requestErr)
	}
	tooLarge.Store(false)
	res, err := transport.Do(ctx, &Request{Method: "GET", URL: server.URL, MaxBodyBytes: MaxImageDownloadBytes})
	require.NoError(t, err)
	require.Equal(t, "image", string(res.Body))
}
