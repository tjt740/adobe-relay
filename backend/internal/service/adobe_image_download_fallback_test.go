//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

func TestAdobeImageDownloadFallbackPreservesResponseAndBilling(t *testing.T) {
	for _, format := range []string{"url", "b64_json"} {
		t.Run(format, func(t *testing.T) {
			api := &adobeFakeTransport{handler: func(req *adobe.Request, index int) (*adobe.Response, error) {
				if req.URL == adobe.ImageSubmitURL {
					return adobeJSONResponse(t, 200, nil, map[string]string{"x-override-status-link": fmt.Sprintf("https://firefly.adobe.io/jobs/job-%d", index)}), nil
				}
				jobID := req.URL[strings.LastIndex(req.URL, "/")+1:]
				return adobeJSONResponse(t, 200, map[string]any{
					"outputs": []any{map[string]any{"image": map[string]any{"presignedUrl": "https://cdn.example/" + jobID + ".png?signature=original"}}},
				}, nil), nil
			}}
			primary := &adobeFakeTransport{handler: func(*adobe.Request, int) (*adobe.Response, error) {
				return nil, adobe.NewUpstreamTemporaryError("TLS handshake failed", 0, adobe.ErrorTypeNetwork)
			}}
			fallback := &adobeFakeTransport{handler: func(req *adobe.Request, _ int) (*adobe.Response, error) {
				require.Equal(t, "GET", req.Method)
				require.Equal(t, map[string]string{"accept": "*/*"}, req.Headers)
				return &adobe.Response{StatusCode: 200, Body: pngBytes}, nil
			}}
			client := adobe.NewClient(adobe.ClientConfig{Transport: api, DownloadTransport: primary, ImageDownloadFallbackTransport: fallback})
			svc := newAdobeTestService(t, client, nil)
			account := adobeTestAccount()
			proxyID := int64(904)
			account.ProxyID = &proxyID
			account.Credentials = map[string]any{"cookie": "original-cookie"}
			result, err := svc.Generate(context.Background(), account, "original-token", &OpenAIImagesRequest{
				Model: "gpt-image-2.5-sunburst", Prompt: "fixture", N: 3, Size: "1024x1024", ResponseFormat: format,
			})
			require.NoError(t, err)
			require.Equal(t, 3, result.Forward.ImageCount)
			require.Len(t, adobeSubmitBodies(t, api), 3, "one job per requested image; downloads add no generation or charge")
			require.Len(t, primary.calls, 3)
			require.Len(t, fallback.calls, 3)
			var primaryURLs, fallbackURLs []string
			for _, r := range primary.calls {
				primaryURLs = append(primaryURLs, r.URL)
			}
			for _, r := range fallback.calls {
				fallbackURLs = append(fallbackURLs, r.URL)
			}
			require.ElementsMatch(t, primaryURLs, fallbackURLs)
			require.Equal(t, int64(904), *account.ProxyID)
			require.Equal(t, "original-cookie", account.GetCredential("cookie"))
			for _, image := range result.Timings.Images {
				require.Equal(t, 1, image.SubmitAttempts)
				require.Equal(t, 2, image.DownloadAttempts)
				require.True(t, image.DownloadFallback)
			}
			var body struct {
				Data []struct {
					URL string `json:"url"`
					B64 string `json:"b64_json"`
				} `json:"data"`
				Usage struct {
					Total int `json:"total_tokens"`
				} `json:"usage"`
			}
			require.NoError(t, json.Unmarshal(result.Body, &body))
			require.Len(t, body.Data, 3)
			require.GreaterOrEqual(t, body.Usage.Total, 1800)
			require.LessOrEqual(t, body.Usage.Total, 3000)
			for _, image := range body.Data {
				if format == "url" {
					require.Contains(t, fallbackURLs, image.URL)
				} else {
					decoded, err := base64.StdEncoding.DecodeString(image.B64)
					require.NoError(t, err)
					require.Equal(t, pngBytes, decoded)
				}
			}
		})
	}
}
