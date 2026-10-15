//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/imagebuffer"
	"github.com/stretchr/testify/require"
)

func TestAdobeBufferedResponsePreservesEveryImageAndMetadata(t *testing.T) {
	for _, format := range []string{"", "b64_json", "url"} {
		for _, storageMode := range []string{"none", "success", "failed"} {
			t.Run(format+"/"+storageMode, func(t *testing.T) {
				req := &OpenAIImagesRequest{Model: "gpt-image-2.5-sunburst", Prompt: "test", N: 2, ResponseFormat: format}
				ctx, cleanup, ok := PrepareAdobeImageBuffer(context.Background(), req)
				require.True(t, ok)
				defer cleanup()
				arena := imagebuffer.FromContext(ctx)
				raw := adobeTestPNG(t, false)
				mapped, err := arena.Read(bytes.NewReader(raw), 64<<20)
				require.NoError(t, err)
				images := [][]byte{mapped, mapped}
				var resolver ImageStorageResolver
				if storageMode != "none" {
					resolver = func() (*ImageResultUploader, bool) {
						var s ImageStorage = &adobeFakeStorage{}
						if storageMode == "failed" {
							s = &adobeFailingStorage{}
						}
						return NewImageResultUploader(s, "adobe/", 0, nil), true
					}
				}
				svc := NewAdobeImageService(resolver)
				timing := &AdobeImageTimings{}
				body, err := svc.buildResponseBody(ctx, "request", images, "high", format, timing, adobeImageResponseMetadata{Request: req, GPTImage: true, UpstreamURLs: []string{"https://cdn.example/a", "https://cdn.example/b"}})
				require.NoError(t, err)
				var response map[string]any
				require.NoError(t, json.Unmarshal(body, &response))
				require.Equal(t, req.Model, response["model"])
				require.Equal(t, "high", response["quality"])
				require.Contains(t, response, "usage")
				require.Contains(t, response, "size")
				require.Contains(t, response, "output_format")
				items := response["data"].([]any)
				require.Len(t, items, 2)
				for _, item := range items {
					entry := item.(map[string]any)
					if b64, ok := entry["b64_json"].(string); ok {
						data, err := base64.StdEncoding.DecodeString(b64)
						require.NoError(t, err)
						require.Equal(t, raw, data)
					} else {
						require.NotEmpty(t, entry["url"])
					}
				}
				if format == "b64_json" || (storageMode != "success" && format == "") {
					require.Equal(t, "b64_json", timing.Delivery)
				}
			})
		}
	}
}

func TestAdobeBufferedGenerationPreservesBillingAndDetachedContext(t *testing.T) {
	req := &OpenAIImagesRequest{Model: "gpt-image-2.5-sunburst", Prompt: "test", N: 2, ResponseFormat: "b64_json"}
	ctx, cleanup, ok := PrepareAdobeImageBuffer(context.Background(), req)
	require.True(t, ok)
	defer cleanup()
	api := &adobeFakeTransport{}
	svc := newAdobeTestService(t, adobeSubmitPollDownload(t, api, pngBytes), nil)
	result, err := svc.Generate(ctx, adobeTestAccount(), "token", req)
	require.NoError(t, err)
	require.Equal(t, 2, result.Forward.ImageCount)
	require.True(t, result.Timings.DiskBuffer)
	require.Len(t, adobeSubmitBodies(t, api), 2)
	require.Len(t, result.Images, 2)
	require.True(t, json.Valid(result.Body))
	require.Contains(t, string(result.Body), base64.StdEncoding.EncodeToString(pngBytes))
}

func TestAdobeBufferEligibilityAndRetainedInputs(t *testing.T) {
	for _, req := range []*OpenAIImagesRequest{nil, {N: 11}, {OutputFormat: "png"}, {OutputFormat: "jpeg"}, {Stream: true}} {
		ctx, cleanup, ok := PrepareAdobeImageBuffer(context.Background(), req)
		require.False(t, ok)
		require.Nil(t, imagebuffer.FromContext(ctx))
		cleanup()
	}
	raw := make([]byte, 1000)
	req := &OpenAIImagesRequest{Multipart: true, Body: raw, Prompt: "abc", Uploads: []OpenAIImagesUpload{{Data: raw[20:500], sharesBody: true}, {Data: make([]byte, 100)}}}
	require.EqualValues(t, 1103, AdobeImageRetainedBytes(req))
	req = &OpenAIImagesRequest{Body: raw, InputImageURLs: []string{"https://example.com/input.png", "data:image/png;base64,AAAA"}}
	require.EqualValues(t, 3000+adobeInputImageMaxBytes, AdobeImageRetainedBytes(req))
}
