package adobe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Adobe's 2026-11-07 discovery schema advertises 1–7, and the current
// Firefly quality conversion maps Extra High to 6 and Max to 7.
func TestGPTImage26QualityContract(t *testing.T) {
	for _, model := range []string{"gpt-image-2.6-flare", "gpt-image-2.6-sunburst", "gpt-image-2.6-star-swirl", "gpt-image-2.6-aurora"} {
		for _, tc := range []struct {
			quality string
			detail  int
		}{{"low", 1}, {"medium", 3}, {"high", 5}, {"xhigh", 6}, {"max", 7}, {" XHIGH ", 6}, {" MAX ", 7}, {"auto", 3}, {"", 3}, {"unknown", 1}} {
			t.Run(model+"/"+tc.quality, func(t *testing.T) {
				for _, edit := range []bool{false, true} {
					conf, err := ResolveImage(ImageRequest{ModelID: model, Size: "3840x2160"})
					require.NoError(t, err)
					payloads, err := BuildImagePayloadCandidates(ImagePayloadOptions{
						UpstreamModelID: conf.UpstreamModelID, UpstreamModelVersion: conf.UpstreamModelVersion,
						PayloadKind: conf.PayloadKind, SizePixels: conf.SizePixels,
						Prompt: "a mug", QualityLevel: tc.quality, Complexity: "high", Edit: edit,
					})
					require.NoError(t, err)
					require.Len(t, payloads, 1)
					require.Equal(t, tc.detail, payloads[0]["generationSettings"].(map[string]any)["detailLevel"])
					require.Equal(t, "high", payloads[0]["modelSpecificPayload"].(map[string]any)["complexity"])
				}
			})
		}
	}
}
