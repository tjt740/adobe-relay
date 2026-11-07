//go:build unit

package adobe

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGPTImage26SuccessfulSubmissionRecordsAppliedQuality(t *testing.T) {
	for _, model := range []string{"gpt-image-2.6-flare", "gpt-image-2.6-sunburst"} {
		for quality, level := range map[string]int{"xhigh": 6, "max": 7} {
			t.Run(model+"/"+quality, func(t *testing.T) {
				conf, err := ResolveImage(ImageRequest{ModelID: model, Size: "3840x2160"})
				require.NoError(t, err)
				api := &fakeTransport{handler: func(req *Request, index int) (*Response, error) {
					if index == 0 {
						var body map[string]any
						require.NoError(t, json.Unmarshal(req.Body, &body))
						require.Equal(t, conf.UpstreamModelVersion, body["modelVersion"])
						require.Equal(t, float64(level), body["generationSettings"].(map[string]any)["detailLevel"])
						return jsonResponse(t, 200, map[string]any{}, map[string]string{"x-override-status-link": "https://firefly-3p.ff.adobe.io/jobs/quality"}), nil
					}
					return jsonResponse(t, 200, map[string]any{"outputs": []any{map[string]any{"image": map[string]any{"presignedUrl": "https://cdn/image.png"}}}}, nil), nil
				}}
				download := &fakeTransport{handler: func(*Request, int) (*Response, error) { return bytesResponse(200, []byte("image")), nil }}
				result, err := testClient(api, download).GenerateImage(WithSingleSubmission(context.Background()), GenerateImageInput{Token: fakeToken(t), Options: ImagePayloadOptions{UpstreamModelID: conf.UpstreamModelID, UpstreamModelVersion: conf.UpstreamModelVersion, PayloadKind: conf.PayloadKind, SizePixels: conf.SizePixels, Prompt: "private prompt", QualityLevel: quality}})
				require.NoError(t, err)
				require.Equal(t, level, result.Timings.SubmittedDetailLevel)
				require.Equal(t, conf.UpstreamModelVersion, result.Timings.SubmittedModelVersion)
				raw, err := json.Marshal(result.Timings)
				require.NoError(t, err)
				require.NotContains(t, string(raw), "private prompt")
				require.NotContains(t, string(raw), "https://cdn")
				require.Equal(t, 1, result.Timings.SubmitAttempts)
			})
		}
	}
}
