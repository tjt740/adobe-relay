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

func TestAdobeImageResponseURLUsageAndMetadata(t *testing.T) {
	for _, format := range []string{"url", "b64_json", ""} {
		t.Run("format_"+format, func(t *testing.T) {
			api := &adobeFakeTransport{}
			original := adobeTestPNG(t, false)
			client := adobeSubmitPollDownload(t, api, original)
			previous := api.handler
			api.handler = func(req *adobe.Request, index int) (*adobe.Response, error) {
				if req.URL == adobe.ImageSubmitURL {
					return previous(req, index)
				}
				raw := adobeUsageExample(t) // Synthetic contract fixture, not a measured Adobe count.
				raw["outputs"] = []any{map[string]any{"image": map[string]any{"presignedUrl": "https://cdn/img.png"}}}
				return adobeJSONResponse(t, 200, raw, nil), nil
			}
			svc := newAdobeTestService(t, client, nil)
			result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
				Model: "gpt-image-2", Prompt: "x", Size: "3840x2160", Quality: "medium", N: 2,
				ResponseFormat: format, OutputFormat: "png", Moderation: "low",
			})
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(result.Body, &body))
			require.Equal(t, "gpt-image-2", body["model"])
			require.Equal(t, "medium", body["quality"])
			require.Equal(t, "auto", body["background"])
			require.Equal(t, "auto", body["moderation"])
			require.Equal(t, "2x2", body["size"]) // Actual bytes, not the requested 4K.
			require.Equal(t, "png", body["output_format"])
			require.EqualValues(t, 10912, body["usage"].(map[string]any)["total_tokens"])
			require.Equal(t, "upstream", body["usage_source"])
			for _, entry := range body["data"].([]any) {
				item := entry.(map[string]any)
				if format == "url" {
					require.Equal(t, "https://cdn/img.png", item["url"])
					require.NotContains(t, item, "b64_json")
				} else {
					require.NotContains(t, item, "url")
					decoded, err := base64.StdEncoding.DecodeString(item["b64_json"].(string))
					require.NoError(t, err)
					require.Equal(t, original, decoded)
				}
			}
			// Response usage must not switch existing per-image billing to tokens.
			require.Equal(t, 2, result.Forward.ImageCount)
			require.Equal(t, "4K", result.Forward.ImageSize)
			require.Zero(t, result.Forward.Usage.InputTokens)
			require.Zero(t, result.Forward.Usage.OutputTokens)
		})
	}
}

func TestAdobeImageResponseTranscodeDoesNotReturnOriginalURL(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			api := &adobeFakeTransport{}
			svc := newAdobeTestService(t, adobeSubmitPollDownload(t, api, adobeTestPNG(t, false)), nil)
			result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
				Model: "gpt-image-2", Prompt: "x", Size: "1024x1024", N: 2, ResponseFormat: "url", OutputFormat: format,
			})
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(result.Body, &body))
			require.Contains(t, body, "usage")
			require.Equal(t, "simulated", body["usage_source"])
			total := body["usage"].(map[string]any)["total_tokens"].(float64)
			require.GreaterOrEqual(t, total, float64(1800))
			require.LessOrEqual(t, total, float64(3000))
			require.Zero(t, result.Forward.Usage.InputTokens)
			require.Zero(t, result.Forward.Usage.OutputTokens)
			require.Equal(t, format, body["output_format"])
			item := body["data"].([]any)[0].(map[string]any)
			if format == "png" {
				require.Equal(t, "https://cdn/img.png", item["url"])
				require.Equal(t, "upstream_url", result.Timings.Delivery)
			} else {
				require.NotContains(t, item, "url")
				require.Equal(t, "b64_json", result.Timings.Delivery)
				decoded, err := base64.StdEncoding.DecodeString(item["b64_json"].(string))
				require.NoError(t, err)
				require.Equal(t, "jpeg", sniffAdobeImageFormat(decoded))
			}
		})
	}
}

func TestAdobeImageResponseStorageAndFallback(t *testing.T) {
	for _, fails := range []bool{false, true} {
		var storage ImageStorage = &adobeFakeStorage{}
		if fails {
			storage = &adobeFailingStorage{}
		}
		svc := NewAdobeImageService(func() (*ImageResultUploader, bool) {
			return NewImageResultUploader(storage, "adobe/", 0, nil), true
		})
		for _, format := range []string{"url", "b64_json", ""} {
			timings := &AdobeImageTimings{}
			body, err := svc.buildResponseBody(context.Background(), "id", [][]byte{adobeTestPNG(t, false)}, "medium", format, timings,
				adobeImageResponseMetadata{UpstreamURLs: []string{"https://adobe.example/image?signature=secret"}})
			require.NoError(t, err)
			var result map[string]any
			require.NoError(t, json.Unmarshal(body, &result))
			item := result["data"].([]any)[0].(map[string]any)
			switch {
			case format == "b64_json" || (fails && format == ""):
				require.Contains(t, item, "b64_json")
				require.Equal(t, "b64_json", timings.Delivery)
			case fails:
				require.Equal(t, "https://adobe.example/image?signature=secret", item["url"])
				require.Equal(t, "upstream_url", timings.Delivery)
				require.True(t, timings.StorageFallback)
			default:
				require.Contains(t, item["url"], "https://cdn.example/adobe/")
				require.Equal(t, "storage_url", timings.Delivery)
			}
		}
	}
}

func TestAdobeImageResponseInvalidURLAndMetadataFallback(t *testing.T) {
	svc := NewAdobeImageService(nil)
	for _, urls := range [][]string{nil, {""}, {"http://example.com/image"}, {"data:image/png;base64,AA=="}, {"https://u:p@example.com/image"}, {"https://one.example", "https://two.example"}} {
		body, err := svc.buildResponseBody(context.Background(), "id", [][]byte{[]byte("original bytes")}, "medium", "url", &AdobeImageTimings{}, adobeImageResponseMetadata{UpstreamURLs: urls})
		require.NoError(t, err)
		var result map[string]any
		require.NoError(t, json.Unmarshal(body, &result))
		require.Contains(t, result["data"].([]any)[0], "b64_json")
		require.NotContains(t, result, "size")
		require.NotContains(t, result, "output_format")
	}
	meta := adobeImageResponseMetadata{Request: &OpenAIImagesRequest{Model: "flux-pro", Background: "transparent", Moderation: "low"}}
	payload := map[string]any{}
	meta.addTo(payload, [][]byte{adobeTestPNG(t, false), adobeTestJPEG(t, 90)})
	require.NotContains(t, payload, "size")
	require.NotContains(t, payload, "output_format")
	require.NotContains(t, payload, "background")
	require.NotContains(t, payload, "moderation")
}
