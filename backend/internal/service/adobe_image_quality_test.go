//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 同时检查上游提交和对外响应，避免只回显输入而掩盖默认值、别名或模型上限。
func TestAdobeImageServiceResponseQualityMatchesSubmittedDetailLevel(t *testing.T) {
	cases := []struct {
		name        string
		model       string
		requested   string
		quality     string
		detailLevel int
	}{
		{"flare_max", "gpt-image-2.5-flare", "max", "max", 7},
		{"flare_xhigh", "gpt-image-2.5-flare", "xhigh", "max", 7},
		{"sunburst_max", "gpt-image-2.5-sunburst", "max", "max", 7},
		{"high", "gpt-image-2.5-flare", "high", "high", 5},
		{"medium", "gpt-image-2.5-flare", "medium", "medium", 3},
		{"low", "gpt-image-2.5-flare", "low", "low", 1},
		{"default", "gpt-image-2.5-flare", "", "low", 1},
		{"auto", "gpt-image-2.5-flare", "auto", "low", 1},
		{"unknown_uses_existing_default", "gpt-image-2.5-flare", "ultra", "low", 1},
		{"case_and_whitespace", "gpt-image-2.5-flare", " MAX ", "max", 7},
		{"v2_max_clamped", "gpt-image-2", "max", "high", 5},
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
	api := &adobeFakeTransport{}
	client := adobeSubmitPollDownload(t, api, []byte("PNGDATA"))
	svc := newAdobeTestService(t, client, nil)
	account := adobeTestAccount()
	account.Credentials = map[string]any{
		"model_mapping": map[string]any{"custom-flare": "firefly-gpt-image-2-5-flare"},
	}
	call := NewAdobeImageCall(&OpenAIImagesRequest{
		Model: "gpt-image-2", Prompt: "a cat", Size: "1024x1024", Quality: "max", N: 1,
	}, "custom-flare")
	result, err := svc.GenerateCall(context.Background(), account, "tok", call)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(result.Body, &payload))
	require.Equal(t, "max", payload["quality"])
	bodies := adobeSubmitBodies(t, api)
	require.Len(t, bodies, 1)
	require.Equal(t, "gpt-image-2.5-flare", bodies[0]["modelVersion"])
	require.Equal(t, float64(7), bodies[0]["generationSettings"].(map[string]any)["detailLevel"])
}
