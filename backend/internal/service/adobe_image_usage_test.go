//go:build unit

package service

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

func adobeUsageExample(t *testing.T) map[string]any {
	t.Helper()
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(`{"usage":{"input_tokens":2120,"input_tokens_details":{"image_tokens":0,"text_tokens":2120},"output_tokens":3336,"total_tokens":5456,"output_tokens_details":{"image_tokens":3336,"text_tokens":0}}}`), &raw))
	return raw
}

func TestAdobeImageUsageParsesExplicitUpstreamCounts(t *testing.T) {
	raw := adobeUsageExample(t)
	usage := parseAdobeImageUsage(raw)
	require.NotNil(t, usage)
	body, err := json.Marshal(usage)
	require.NoError(t, err)
	expected, err := json.Marshal(raw["usage"])
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(body))

	perOutput := map[string]any{"outputs": []any{map[string]any{"usage": raw["usage"]}}}
	require.Equal(t, usage, parseAdobeImageUsage(perOutput))
	perOutput["outputs"] = append(perOutput["outputs"].([]any), map[string]any{"usage": raw["usage"]})
	require.Nil(t, parseAdobeImageUsage(perOutput))
}

func TestAdobeImageUsageRejectsUnknownAndInvalidCounts(t *testing.T) {
	cases := map[string]func(map[string]any){
		"absent":             func(raw map[string]any) { delete(raw, "usage") },
		"null":               func(raw map[string]any) { raw["usage"] = nil },
		"credits_only":       func(raw map[string]any) { raw["usage"] = map[string]any{"credits": 10} },
		"missing_total":      func(raw map[string]any) { delete(raw["usage"].(map[string]any), "total_tokens") },
		"inconsistent_total": func(raw map[string]any) { raw["usage"].(map[string]any)["total_tokens"] = 1 },
		"negative":           func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens"] = -1 },
		"fraction":           func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens"] = 0.5 },
		"numeric_string":     func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens"] = "2120" },
		"nan":                func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens"] = math.NaN() },
		"infinite":           func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens"] = math.Inf(1) },
		"unsafe_integer":     func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens"] = float64(1 << 53) },
		"negative_details": func(raw map[string]any) {
			raw["usage"].(map[string]any)["input_tokens_details"].(map[string]any)["image_tokens"] = -1
		},
		"excess_details": func(raw map[string]any) {
			raw["usage"].(map[string]any)["input_tokens_details"].(map[string]any)["image_tokens"] = 10
		},
		"invalid_details": func(raw map[string]any) { raw["usage"].(map[string]any)["input_tokens_details"] = "unknown" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			raw := adobeUsageExample(t)
			change(raw)
			require.Nil(t, parseAdobeImageUsage(raw))
		})
	}
}

func TestAdobeImageUsageBatchRequiresEveryImage(t *testing.T) {
	results := []*adobe.GenerateResult{{Raw: adobeUsageExample(t)}, {Raw: adobeUsageExample(t)}}
	usage := aggregateAdobeImageUsage(results)
	require.EqualValues(t, 4240, usage.InputTokens)
	require.EqualValues(t, 6672, usage.OutputTokens)
	require.EqualValues(t, 10912, usage.TotalTokens)
	require.EqualValues(t, 6672, *usage.OutputTokensDetails.ImageTokens)
	require.Zero(t, *usage.InputTokensDetails.ImageTokens)

	delete(results[1].Raw["usage"].(map[string]any), "input_tokens_details")
	usage = aggregateAdobeImageUsage(results)
	require.Nil(t, usage.InputTokensDetails) // Missing breakdown is not zero.
	require.NotNil(t, usage.OutputTokensDetails)
	delete(results[1].Raw, "usage")
	require.Nil(t, aggregateAdobeImageUsage(results))
	results[1] = nil
	require.Nil(t, aggregateAdobeImageUsage(results))
	require.Nil(t, aggregateAdobeImageUsage(nil))
}

func TestAdobeImageUsagePartialDetailsAndOverflow(t *testing.T) {
	raw := adobeUsageExample(t)
	delete(raw["usage"].(map[string]any)["input_tokens_details"].(map[string]any), "image_tokens")
	usage := aggregateAdobeImageUsage([]*adobe.GenerateResult{{Raw: raw}, {Raw: adobeUsageExample(t)}})
	require.NotNil(t, usage.InputTokensDetails)
	require.Nil(t, usage.InputTokensDetails.ImageTokens)
	require.EqualValues(t, 4240, *usage.InputTokensDetails.TextTokens)

	large := map[string]any{"usage": map[string]any{"input_tokens": maxAdobeUsageTokens, "output_tokens": 0, "total_tokens": maxAdobeUsageTokens}}
	require.NotNil(t, parseAdobeImageUsage(large))
	require.Nil(t, aggregateAdobeImageUsage([]*adobe.GenerateResult{{Raw: large}, {Raw: adobeUsageExample(t)}}))

	zero := map[string]any{"usage": map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}}
	require.NotNil(t, parseAdobeImageUsage(zero)) // Explicit upstream zero is valid.
}

func TestAdobeImageUsageRealFireflyResponseHasNoCounts(t *testing.T) {
	body, err := os.ReadFile("testdata/adobe_image_response_20261014.json")
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(body, &raw))
	require.NotEmpty(t, raw["outputs"])
	require.Nil(t, parseAdobeImageUsage(raw))
}
