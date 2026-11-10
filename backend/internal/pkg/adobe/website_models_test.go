package adobe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type websiteRoundTripper func(*http.Request) (*http.Response, error)

func (f websiteRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWebsiteModelsFetchPublicAdobeDirectory(t *testing.T) {
	client := NewWebsiteModelsClient()
	require.Equal(t, 8*time.Second, client.client.Timeout)
	require.ErrorIs(t, client.client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
	requests := 0
	client.client.Transport = websiteRoundTripper(func(r *http.Request) (*http.Response, error) {
		requests++
		require.Equal(t, WebsiteModelsURL, r.URL.String())
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, "clio-playground-web", r.Header.Get("x-api-key"))
		require.Equal(t, "https://firefly.adobe.com", r.Header.Get("Origin"))
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
			`{"models":[{"modelId":"gpt-image","modelVersions":{"gpt-image-2.6-star-swirl":{"modelDisplayName":"GPT Image 2.6 Flare","enabled":true,"outputModality":["image"],"requestSchema":{"ignored":true}}}}],"commonRequestSchema":{"ignored":true}}`))}, nil
	})
	for range 2 {
		result, err := client.Fetch(context.Background())
		require.NoError(t, err)
		require.Equal(t, "adobe", result.Source)
		require.Equal(t, WebsiteModelsURL, result.SourceURL)
		require.WithinDuration(t, time.Now(), result.FetchedAt, time.Second)
		require.Equal(t, "GPT Image 2.6 Flare", result.Models[0].Versions["gpt-image-2.6-star-swirl"].DisplayName)
	}
	require.Equal(t, 2, requests, "each load should fetch fresh metadata")
}

func TestWebsiteModelsFailureDoesNotInventSource(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{"upstream", 503, "private upstream error", nil},
		{"redirect", 302, "", nil},
		{"html", 200, "<html>sign in</html>", nil},
		{"empty", 200, `{"models":[]}`, nil},
		{"malformed", 200, `{"models":"oops"}`, nil},
		{"oversized", 200, strings.Repeat(" ", websiteModelsMaxBytes+1), nil},
		{"network", 0, "", errors.New("private transport detail")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewWebsiteModelsClient()
			client.client.Transport = websiteRoundTripper(func(*http.Request) (*http.Response, error) {
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			result, err := client.Fetch(context.Background())
			require.Error(t, err)
			require.Nil(t, result)
			require.NotContains(t, err.Error(), "private")
		})
	}
}

func TestWebsiteModelsHonorsCancellation(t *testing.T) {
	client := NewWebsiteModelsClient()
	client.client.Transport = websiteRoundTripper(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := client.Fetch(ctx)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestWebsitePricingSchemaPreservesActualTiersAndResolvesOnlyLocalRefs(t *testing.T) {
	var raw struct {
		Schema map[string]any
		Common map[string]any
	}
	require.NoError(t, json.Unmarshal([]byte(`{
 "Schema":{"properties":{"prompt":{"type":"string","description":"not forwarded"},"resolution":{"$ref":"#/$defs/resolution"},"generationSettings":{"properties":{"detailLevel":{"anyOf":[{"minimum":1,"maximum":5,"type":"integer"},{"type":"null"}]},"contentPolicy":{"enum":["not forwarded"]}}},"size":{"properties":{"width":{"minimum":480,"maximum":3840},"height":{"minimum":480,"maximum":3840}}}}},
 "Common":{"$defs":{"resolution":{"anyOf":[{"enum":["512","1K","1.5K","2K","4K"]},{"type":"null"}]}}}
 }`), &raw))
	out := websitePricingSchema(raw.Schema, raw.Common, 0)
	body, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(body), "prompt")
	require.NotContains(t, string(body), "contentPolicy")
	require.NotContains(t, string(body), "not forwarded")
	require.NotContains(t, string(body), "$ref")
	require.Contains(t, string(body), `"maximum":5`)
	require.Contains(t, string(body), `"1.5K"`)
	require.Contains(t, string(body), `"maximum":3840`)
	for _, ref := range []string{"https://example.invalid/schema", "#/$defs/missing"} {
		require.Equal(t, map[string]any{"$ref": ref}, websitePricingSchema(map[string]any{"$ref": ref}, raw.Common, 0))
	}
	// A malformed upstream cycle is bounded without any extra network requests.
	cycle := map[string]any{"$ref": "#/$defs/self"}
	require.Nil(t, websitePricingSchema(cycle, map[string]any{"$defs": map[string]any{"self": cycle}}, 0))
}
