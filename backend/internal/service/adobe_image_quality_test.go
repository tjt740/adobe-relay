//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

// 同时检查上游提交和对外响应，确保保留 xhigh 标签时不改变提交档位、默认值或模型上限。
func TestAdobeImageServiceResponseQualityMatchesSubmittedDetailLevel(t *testing.T) {
	cases := []struct {
		name        string
		model       string
		requested   string
		quality     string
		detailLevel int
	}{
		{"flare_max", "gpt-image-2.5-flare", "max", "max", 7},
		{"flare_xhigh", "gpt-image-2.5-flare", "xhigh", "xhigh", 7},
		{"sunburst_max", "gpt-image-2.5-sunburst", "max", "max", 7},
		{"sunburst_xhigh", "gpt-image-2.5-sunburst", "xhigh", "xhigh", 7},
		{"prism_xhigh", "gpt-image-2.5-prism", "xhigh", "xhigh", 7},
		{"high", "gpt-image-2.5-flare", "high", "high", 5},
		{"medium", "gpt-image-2.5-flare", "medium", "medium", 3},
		{"low", "gpt-image-2.5-flare", "low", "low", 1},
		{"default", "gpt-image-2.5-flare", "", "low", 1},
		{"auto", "gpt-image-2.5-flare", "auto", "low", 1},
		{"unknown_uses_existing_default", "gpt-image-2.5-flare", "ultra", "low", 1},
		{"case_and_whitespace", "gpt-image-2.5-flare", " MAX ", "max", 7},
		{"xhigh_case_and_whitespace", "gpt-image-2.5-flare", " XHIGH ", "xhigh", 7},
		{"v2_max_clamped", "gpt-image-2", "max", "high", 5},
		{"v2_xhigh_clamped", "gpt-image-2", "xhigh", "high", 5},
		{"v15_xhigh_clamped", "gpt-image-1.5", "xhigh", "high", 5},
		{"nano_banana_omits_quality", "nano-banana-pro", "max", "", 0},
		{"flux_omits_quality", "flux-pro", "max", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &adobeFakeTransport{}
			client := adobeSubmitPollDownload(t, api, []byte("PNGDATA"))
			svc := newAdobeTestService(t, client, nil)
			result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
				Model: tc.model, Prompt: "a cat", Size: "1024x1024", Quality: tc.requested, N: 2,
			})
			require.NoError(t, err)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(result.Body, &payload))
			require.Len(t, payload["data"], 2)
			bodies := adobeSubmitBodies(t, api)
			require.Len(t, bodies, 2)
			if tc.quality == "" {
				require.NotContains(t, payload, "quality")
				for _, body := range bodies {
					require.NotContains(t, body, "generationSettings")
				}
				return
			}
			require.Equal(t, tc.quality, payload["quality"])
			for _, body := range bodies {
				settings, ok := body["generationSettings"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, float64(tc.detailLevel), settings["detailLevel"])
			}
		})
	}
}

func TestAdobeImageServiceResponseQualityUsesMappedModel(t *testing.T) {
	cases := []struct {
		name, model, mappedModel, requested, quality, version string
		detailLevel                                           int
	}{
		{"max_to_flare", "gpt-image-2", "firefly-gpt-image-2-5-flare", "max", "max", "gpt-image-2.5-flare", 7},
		{"xhigh_to_flare", "gpt-image-2", "firefly-gpt-image-2-5-flare", "xhigh", "xhigh", "gpt-image-2.5-flare", 7},
		{"xhigh_to_sunburst", "gpt-image-2", "firefly-gpt-image-2-5-prism", "xhigh", "xhigh", "gpt-image-2.5-prism", 7},
		{"xhigh_to_v2", "gpt-image-2.5-flare", "firefly-gpt-image-2", "xhigh", "high", "2", 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &adobeFakeTransport{}
			client := adobeSubmitPollDownload(t, api, []byte("PNGDATA"))
			svc := newAdobeTestService(t, client, nil)
			account := adobeTestAccount()
			account.Credentials = map[string]any{
				"model_mapping": map[string]any{"custom-image": tc.mappedModel},
			}
			call := NewAdobeImageCall(&OpenAIImagesRequest{
				Model: tc.model, Prompt: "a cat", Size: "1024x1024", Quality: tc.requested, N: 1,
			}, "custom-image")
			result, err := svc.GenerateCall(context.Background(), account, "tok", call)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(result.Body, &payload))
			require.Equal(t, tc.quality, payload["quality"])
			bodies := adobeSubmitBodies(t, api)
			require.Len(t, bodies, 1)
			require.Equal(t, tc.version, bodies[0]["modelVersion"])
			require.Equal(t, float64(tc.detailLevel), bodies[0]["generationSettings"].(map[string]any)["detailLevel"])
		})
	}
}

func TestAdobeImageServiceXHighResponseAcrossDeliveryPaths(t *testing.T) {
	cases := []struct {
		name, endpoint, format, storage, delivery string
		buffered                                  bool
	}{
		{"base64", openAIImagesGenerationsEndpoint, "b64_json", "none", "b64_json", false},
		{"upstream_url", openAIImagesGenerationsEndpoint, "url", "none", "upstream_url", false},
		{"storage_url", openAIImagesGenerationsEndpoint, "url", "success", "storage_url", false},
		{"storage_fallback", openAIImagesGenerationsEndpoint, "", "failed", "b64_json", false},
		{"edits_base64", openAIImagesEditsEndpoint, "b64_json", "none", "b64_json", false},
		{"edits_url", openAIImagesEditsEndpoint, "url", "none", "upstream_url", false},
		{"buffered_base64", openAIImagesGenerationsEndpoint, "b64_json", "none", "b64_json", true},
		{"buffered_storage_fallback", openAIImagesEditsEndpoint, "", "failed", "b64_json", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := adobeTestPNG(t, false)
			api := &adobeFakeTransport{}
			client := adobeSubmitPollDownload(t, api, original)
			previous := api.handler
			api.handler = func(req *adobe.Request, index int) (*adobe.Response, error) {
				if req.URL == adobe.ImageUploadURL {
					return adobeJSONResponse(t, 200, map[string]any{
						"images": []any{map[string]any{"id": "img-1"}},
					}, nil), nil
				}
				return previous(req, index)
			}
			var resolver ImageStorageResolver
			if tc.storage != "none" {
				resolver = func() (*ImageResultUploader, bool) {
					var storage ImageStorage = &adobeFakeStorage{}
					if tc.storage == "failed" {
						storage = &adobeFailingStorage{}
					}
					return NewImageResultUploader(storage, "adobe/", 0, nil), true
				}
			}
			req := &OpenAIImagesRequest{
				Model: "gpt-image-2.5-sunburst", Prompt: "a cat", Size: "1024x1024",
				Quality: "xhigh", N: 2, Endpoint: tc.endpoint, ResponseFormat: tc.format,
			}
			if req.IsEdits() {
				req.Uploads = []OpenAIImagesUpload{{Data: original, ContentType: "image/png"}}
			}
			ctx := context.Background()
			if tc.buffered {
				var cleanup func()
				var ok bool
				ctx, cleanup, ok = PrepareAdobeImageBuffer(ctx, req)
				require.True(t, ok)
				defer cleanup()
			}
			result, err := newAdobeTestService(t, client, resolver).Generate(ctx, adobeTestAccount(), "tok", req)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(result.Body, &payload))
			require.Equal(t, "xhigh", payload["quality"])
			require.Equal(t, tc.delivery, result.Timings.Delivery)
			require.Equal(t, tc.storage == "failed", result.Timings.StorageFallback)
			items := payload["data"].([]any)
			require.Len(t, items, 2)
			for _, item := range items {
				entry := item.(map[string]any)
				if tc.delivery == "b64_json" {
					decoded, err := base64.StdEncoding.DecodeString(entry["b64_json"].(string))
					require.NoError(t, err)
					require.Equal(t, original, decoded)
				} else {
					require.NotEmpty(t, entry["url"])
				}
			}
			bodies := adobeSubmitBodies(t, api)
			require.Len(t, bodies, 2)
			for _, body := range bodies {
				require.Equal(t, float64(7), body["generationSettings"].(map[string]any)["detailLevel"])
				if req.IsEdits() {
					require.Equal(t, []any{map[string]any{"id": "img-1", "usage": "subject"}}, body["referenceBlobs"])
				}
			}
		})
	}
}
