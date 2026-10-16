package middleware

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageMemoryMappedInputPreservesBytesAndBudgets(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "disk", true: "heap_fallback"}[fallback], func(t *testing.T) {
			g := newImageMemoryGuard(128<<20, 64<<20)
			if fallback {
				g.bufferInput = func([]byte) ([]byte, func(), error) { return nil, nil, errors.New("disk unavailable") }
			}
			body := strings.Repeat("x", (2<<20)+37) // Non-page-aligned mappings must preserve the exact size.
			router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
				read, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
				require.NoError(t, err)
				require.True(t, bytes.Equal(read, []byte(body)))
				lease := c.MustGet(imageMemoryLeaseKey).(*imageMemoryLease)
				mapped := int64(len(body))
				if fallback {
					mapped = 0
				}
				require.Equal(t, mapped, lease.mappedBodyBytes)
				require.Equal(t, mapped, g.mappedBytes.Load())
				require.EqualValues(t, 4<<20+6*len(body), lease.weight, "parse/audit peak remains reserved")
				require.True(t, FinishImageRequestParsing(c, 3*int64(len(body)), 2))
				require.EqualValues(t, 3*len(body)+(2<<20)-int(mapped), lease.weight)
				require.True(t, FinishImageMemoryPreparation(c, int64(len(body)), 2))
				require.EqualValues(t, 3*len(body)+(64<<20)-int(mapped), lease.weight, "legacy output allowance remains intact")
				c.Status(204)
			})
			req, _ := memoryRequest("/v1/images/edits", body)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, 204, w.Code)
			require.Zero(t, g.used.Load())
			require.Zero(t, g.mappedBytes.Load())
			if !fallback {
				require.Equal(t, http.NoBody, req.Body)
			}
		})
	}
}

func TestImageMemoryInputCleanupAfterDisconnectAndPanic(t *testing.T) {
	for _, panicHandler := range []bool{false, true} {
		t.Run(map[bool]string{false: "disconnect", true: "panic"}[panicHandler], func(t *testing.T) {
			g := newImageMemoryGuard(64<<20, 64<<20)
			var closed atomic.Int64
			g.bufferInput = func(body []byte) ([]byte, func(), error) {
				mapped, cleanup, err := bufferImageRequestInput(body)
				return mapped, func() { cleanup(); closed.Add(1) }, err
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
				cancel()
				require.Zero(t, closed.Load(), "accepted work retains its mapping despite disconnect")
				read, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
				require.NoError(t, err)
				require.Len(t, read, 2<<20)
				if panicHandler {
					panic("simulated handler failure")
				}
				c.Status(200)
			})
			req, _ := memoryRequest("/v1/images/edits", strings.Repeat("x", 2<<20))
			func() {
				defer func() {
					if panicHandler {
						require.NotNil(t, recover())
					}
				}()
				router.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))
			}()
			require.EqualValues(t, 1, closed.Load())
			require.Zero(t, g.used.Load())
			require.Zero(t, g.mappedBytes.Load())
		})
	}
}

func TestImageMemoryInputDoesNotMapAsyncOrSmallRequests(t *testing.T) {
	for _, tc := range []struct {
		path string
		size int
	}{{"/v1/images/edits/async", 2 << 20}, {"/v1/images/edits", 1024}, {"/images/generations", 2 << 20}} {
		g := newImageMemoryGuard(64<<20, 64<<20)
		g.bufferInput = func([]byte) ([]byte, func(), error) {
			t.Fatal("mapping lifetime must not cover asynchronous or unrelated endpoints")
			return nil, nil, nil
		}
		router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) { c.Status(204) })
		req, _ := memoryRequest(tc.path, strings.Repeat("x", tc.size))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, 204, w.Code)
	}
}

func TestImageMemoryMappedMultipartStillParsesAuditsAndPreservesUpload(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	require.NoError(t, w.WriteField("model", "gpt-image-2.5-flare"))
	require.NoError(t, w.WriteField("prompt", "unchanged prompt"))
	part, err := w.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	image := bytes.Repeat([]byte{0x89, 0x50, 0x4e, 0x47}, 1<<19)
	_, err = part.Write(image)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	g := newImageMemoryGuard(64<<20, 64<<20)
	router := imageMemoryRouter(g, service.PlatformAdobe, func(c *gin.Context) {
		raw, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		require.NoError(t, err)
		parsed, err := (&service.OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, raw)
		require.NoError(t, err)
		require.Equal(t, "unchanged prompt", parsed.Prompt)
		require.True(t, parsed.Multipart)
		require.Len(t, parsed.Uploads, 1)
		require.Equal(t, image, parsed.Uploads[0].Data)
		require.Contains(t, string(parsed.ModerationBody()), "unchanged prompt")
		require.True(t, FinishImageRequestParsing(c, service.AdobeImageRetainedBytes(parsed), 1))
		require.Less(t, g.used.Load(), int64(2<<20), "mapped upload must not retain its raw size in the heap ledger")
		c.Status(204)
	})
	req := httptest.NewRequest("POST", "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, 204, rec.Code)
	require.Zero(t, g.used.Load())
	require.Zero(t, g.mappedBytes.Load())
}
