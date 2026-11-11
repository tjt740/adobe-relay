//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"os"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

func adobeTestWebP(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/adobe-webp-alpha.webp")
	require.NoError(t, err)
	return data
}

func TestAdobeWebPIdentityAndDecode(t *testing.T) {
	src := adobeTestWebP(t)
	for _, compression := range []*int{nil, intPtr(0), intPtr(70), intPtr(100)} {
		got, err := transcodeAdobeImage(src, adobeOutputFormatWebP, compression)
		require.NoError(t, err)
		require.Equal(t, src, got, "native WebP must not be recompressed locally")
		require.NoError(t, validateAdobeOutputBytes(got, adobeOutputFormatWebP))
	}
	img, format, err := image.Decode(bytes.NewReader(src))
	require.NoError(t, err)
	require.Equal(t, "webp", format)
	require.True(t, imageHasTransparency(img))
	for _, data := range [][]byte{adobeTestPNG(t, false), []byte("RIFF0000WEBP"), src[:20]} {
		require.Error(t, validateAdobeOutputBytes(data, adobeOutputFormatWebP))
	}
}

func TestAdobeWebPServiceNativeDelivery(t *testing.T) {
	for _, model := range []string{"gpt-image-2.6-flare", "gpt-image-2.6-sunburst", "gpt-image-2.6-star-swirl", "gpt-image-2.6-aurora"} {
		for _, delivery := range []string{"url", "b64_json"} {
			t.Run(model+"/"+delivery, func(t *testing.T) {
				api := &adobeFakeTransport{}
				original := adobeTestWebP(t)
				svc := newAdobeTestService(t, adobeSubmitPollDownload(t, api, original), nil)
				result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
					Model: model, Prompt: "red mug", Size: "1024x1024", Quality: "medium", N: 1,
					OutputFormat: " WEBP ", OutputCompression: intPtr(70), Background: "transparent", ResponseFormat: delivery,
				})
				require.NoError(t, err)
				require.Equal(t, original, result.Images[0])
				bodies := adobeSubmitBodies(t, api)
				require.Len(t, bodies, 1)
				msp := bodies[0]["modelSpecificPayload"].(map[string]any)
				require.Equal(t, "webp", msp["output_format"])
				require.EqualValues(t, 70, msp["output_compression"])
				require.Equal(t, "transparent", msp["background"])
				require.NotContains(t, msp, "frame_count")
				var body map[string]any
				require.NoError(t, json.Unmarshal(result.Body, &body))
				require.Equal(t, "webp", body["output_format"])
				require.Equal(t, "2x2", body["size"])
				item := body["data"].([]any)[0].(map[string]any)
				if delivery == "url" {
					require.Equal(t, "https://cdn/img.png", item["url"], "preserve the original URL irrespective of its suffix")
				} else {
					decoded, err := base64.StdEncoding.DecodeString(item["b64_json"].(string))
					require.NoError(t, err)
					require.Equal(t, original, decoded)
				}
				require.Equal(t, "1K", result.Forward.ImageSize)
				require.Equal(t, "medium", result.Forward.ImageQuality)
				require.Equal(t, 1, result.Forward.ImageCount)
				require.Zero(t, result.Forward.Usage.OutputTokens)
			})
		}
	}
}

func TestAdobeWebPRejectsUnsupportedBeforeReferenceUpload(t *testing.T) {
	for _, tc := range []struct {
		name, model, mapped string
		compression         *int
		code                string
	}{
		{"old model", "gpt-image-2.5-flare", "", nil, "unsupported_output_format"},
		{"mapped old model", "gpt-image-2.6-sunburst", "gpt-image-2.5-sunburst", nil, "unsupported_output_format"},
		{"negative compression", "gpt-image-2.6-flare", "", intPtr(-1), "invalid_output_compression"},
		{"high compression", "gpt-image-2.6-flare", "", intPtr(101), "invalid_output_compression"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) {
				t.Fatal("invalid request must not reach upstream")
				return nil, nil
			}}
			account := adobeTestAccount()
			if tc.mapped != "" {
				account.Credentials = map[string]any{"model_mapping": map[string]any{tc.model: tc.mapped}}
			}
			svc := newAdobeTestService(t, adobe.NewClient(adobe.ClientConfig{Transport: api}), nil)
			_, err := svc.Generate(context.Background(), account, "tok", &OpenAIImagesRequest{
				Model: tc.model, Prompt: "mug", OutputFormat: "webp", OutputCompression: tc.compression,
				InputImageURLs: []string{"https://invalid.example/reference.png"},
			})
			require.Error(t, err)
			require.Empty(t, api.calls)
			failure := classifyAdobeError(err).Failover
			require.Equal(t, 400, failure.ClientStatusCode)
			require.Equal(t, tc.code, failure.ClientErrorCode)
			require.Equal(t, NextAccountStop, failure.NextAccountAction)
		})
	}
}

func TestAdobeWebPWrongOutputStopsWithoutResubmission(t *testing.T) {
	for _, data := range [][]byte{adobeTestPNG(t, false), []byte("RIFF0000WEBP")} {
		api := &adobeFakeTransport{}
		svc := newAdobeTestService(t, adobeSubmitPollDownload(t, api, data), nil)
		result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
			Model: "gpt-image-2.6-sunburst", Prompt: "mug", OutputFormat: "webp", ResponseFormat: "url",
		})
		require.Error(t, err)
		require.Nil(t, result)
		require.Len(t, adobeSubmitBodies(t, api), 1)
		failure := classifyAdobeError(err).Failover
		require.Equal(t, 502, failure.ClientStatusCode)
		require.Equal(t, "invalid_image_output", failure.ClientErrorCode)
		require.Equal(t, "transcode", failure.ClientErrorStage)
		require.Equal(t, NextAccountStop, failure.NextAccountAction)
	}
}

func TestAdobeWebPCompressionRejectsFractionalJSON(t *testing.T) {
	conf, err := adobe.ResolveImage(adobe.ImageRequest{ModelID: "gpt-image-2.6-flare"})
	require.NoError(t, err)
	for _, tc := range []struct {
		raw    string
		parsed int
		valid  bool
	}{
		{"0", 0, true}, {"100", 100, true}, {"70", 70, true},
		{"-0.5", 0, false}, {"25.5", 25, false}, {"100.5", 100, false},
	} {
		req := &OpenAIImagesRequest{OutputCompression: &tc.parsed, Body: []byte(`{"output_compression":` + tc.raw + `}`)}
		err := validateAdobeWebPParams(req, conf, adobeOutputFormatWebP)
		if tc.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestAdobeWebPStoredURLPreservesFormat(t *testing.T) {
	api := &adobeFakeTransport{}
	original := adobeTestWebP(t)
	storage := &adobeFakeStorage{}
	uploader := NewImageResultUploader(storage, "adobe/", 0, nil)
	svc := newAdobeTestService(t, adobeSubmitPollDownload(t, api, original), func() (*ImageResultUploader, bool) { return uploader, true })
	result, err := svc.Generate(context.Background(), adobeTestAccount(), "tok", &OpenAIImagesRequest{
		Model: "gpt-image-2.6-flare", Prompt: "mug", OutputFormat: "webp", ResponseFormat: "url",
	})
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(result.Body, &body))
	url := body["data"].([]any)[0].(map[string]any)["url"].(string)
	require.Contains(t, url, ".webp")
	require.Equal(t, "webp", body["output_format"])
	require.Len(t, storage.saved, 1)
	for _, data := range storage.saved {
		require.Equal(t, original, data)
	}
}
