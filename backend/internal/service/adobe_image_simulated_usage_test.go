//go:build unit

package service

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func assertSimulatedAdobeUsage(t *testing.T, usage *adobeImageUsage, hasInputImage bool) {
	t.Helper()
	require.NotNil(t, usage)
	require.GreaterOrEqual(t, usage.TotalTokens, int64(1800))
	require.LessOrEqual(t, usage.TotalTokens, int64(3000))
	require.Positive(t, usage.InputTokens)
	require.Positive(t, usage.OutputTokens)
	require.Equal(t, usage.TotalTokens, usage.InputTokens+usage.OutputTokens)
	require.NotNil(t, usage.InputTokensDetails)
	require.NotNil(t, usage.OutputTokensDetails)
	require.NotNil(t, usage.InputTokensDetails.ImageTokens)
	require.NotNil(t, usage.InputTokensDetails.TextTokens)
	require.NotNil(t, usage.OutputTokensDetails.ImageTokens)
	require.NotNil(t, usage.OutputTokensDetails.TextTokens)
	require.Equal(t, usage.InputTokens, *usage.InputTokensDetails.ImageTokens+*usage.InputTokensDetails.TextTokens)
	require.Positive(t, *usage.InputTokensDetails.TextTokens)
	if hasInputImage {
		require.Positive(t, *usage.InputTokensDetails.ImageTokens)
	} else {
		require.Zero(t, *usage.InputTokensDetails.ImageTokens)
	}
	require.Equal(t, usage.OutputTokens, *usage.OutputTokensDetails.ImageTokens)
	require.Zero(t, *usage.OutputTokensDetails.TextTokens)
}

func TestAdobeImageSimulatedUsageInclusiveBoundaries(t *testing.T) {
	for _, hasImage := range []bool{false, true} {
		low := buildSimulatedAdobeImageUsage(hasImage, func(int64) int64 { return 0 })
		high := buildSimulatedAdobeImageUsage(hasImage, func(n int64) int64 { return n - 1 })
		assertSimulatedAdobeUsage(t, low, hasImage)
		assertSimulatedAdobeUsage(t, high, hasImage)
		require.Equal(t, int64(1800), low.TotalTokens)
		require.Equal(t, int64(3000), high.TotalTokens)
	}
}

func TestAdobeImageSimulatedUsageConcurrentRequests(t *testing.T) {
	var wg sync.WaitGroup
	for worker := range 16 {
		wg.Go(func() {
			for range 32 {
				assertSimulatedAdobeUsage(t, simulatedAdobeImageUsage(worker%2 == 0), worker%2 == 0)
			}
		})
	}
	wg.Wait()
}

func TestAdobeImageSimulatedUsageResponseMetadata(t *testing.T) {
	for _, req := range []*OpenAIImagesRequest{
		{Model: "gpt-image-2", N: 1},
		{Model: "gpt-image-2", N: 3},
		{Model: "gpt-image-2", InputImageURLs: []string{"https://example.invalid/reference.png"}},
		{Model: "gpt-image-2", Uploads: []OpenAIImagesUpload{{Data: []byte("reference")}}},
	} {
		body := map[string]any{}
		meta := adobeImageResponseMetadata{Request: req, GPTImage: true}
		meta.addTo(body, [][]byte{adobeTestPNG(t, false)})
		require.Equal(t, "simulated", body["usage_source"])
		require.Nil(t, meta.Usage) // Never mutate the original measured usage.
		assertSimulatedAdobeUsage(t, body["usage"].(*adobeImageUsage), len(req.InputImageURLs)+len(req.Uploads) > 0)
		require.Equal(t, "2x2", body["size"])
		require.Equal(t, "png", body["output_format"])
	}
}

func TestAdobeImageSimulatedUsagePreservesMeasuredZeroAndPartialDetails(t *testing.T) {
	for _, usage := range []*adobeImageUsage{
		{},
		{InputTokens: 5, OutputTokens: 7, TotalTokens: 12},
		{InputTokens: 5, OutputTokens: 7, TotalTokens: 12, InputTokensDetails: &adobeImageTokenDetails{TextTokens: new(int64(5))}},
	} {
		before, err := json.Marshal(usage)
		require.NoError(t, err)
		body := map[string]any{}
		adobeImageResponseMetadata{Usage: usage}.addTo(body, [][]byte{adobeTestPNG(t, false)})
		require.Equal(t, "upstream", body["usage_source"])
		require.Same(t, usage, body["usage"])
		after, err := json.Marshal(body["usage"])
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
}
