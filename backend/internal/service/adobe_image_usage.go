package service

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

// The parser and aggregator accept only explicit upstream counts. The response
// layer may also use this shape for explicitly labeled simulated data; it never
// feeds that data back into the parser, aggregator or billing.
type adobeImageUsage struct {
	InputTokens         int64                   `json:"input_tokens"`
	InputTokensDetails  *adobeImageTokenDetails `json:"input_tokens_details,omitempty"`
	OutputTokens        int64                   `json:"output_tokens"`
	TotalTokens         int64                   `json:"total_tokens"`
	OutputTokensDetails *adobeImageTokenDetails `json:"output_tokens_details,omitempty"`
}

type adobeImageTokenDetails struct {
	ImageTokens *int64 `json:"image_tokens,omitempty"`
	TextTokens  *int64 `json:"text_tokens,omitempty"`
}

// JSON was decoded into float64 by the Adobe client. Beyond this bound an
// integer cannot be represented reliably, even before aggregation.
const maxAdobeUsageTokens int64 = 1<<53 - 1

func adobeUsageNumber(value any) (int64, bool) {
	var n int64
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > float64(maxAdobeUsageTokens) || math.Trunc(v) != v {
			return 0, false
		}
		n = int64(v)
	case json.Number:
		var err error
		n, err = strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return 0, false
		}
	case int64:
		n = v
	case int:
		n = int64(v)
	default:
		return 0, false
	}
	return n, n >= 0 && n <= maxAdobeUsageTokens
}

func parseAdobeTokenDetails(value any, limit int64) (*adobeImageTokenDetails, bool) {
	if value == nil {
		return nil, true
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	details := &adobeImageTokenDetails{}
	var sum int64
	for key, dest := range map[string]**int64{"image_tokens": &details.ImageTokens, "text_tokens": &details.TextTokens} {
		if raw, present := m[key]; present {
			n, valid := adobeUsageNumber(raw)
			if !valid || n > limit {
				return nil, false
			}
			*dest = &n
			sum += n
		}
	}
	if sum > limit {
		return nil, false
	}
	if details.ImageTokens == nil && details.TextTokens == nil {
		return nil, true
	}
	return details, true
}

func parseAdobeImageUsage(raw map[string]any) *adobeImageUsage {
	value, present := raw["usage"]
	if !present {
		// Each Firefly task generates one image. Accept a per-output usage
		// envelope only when there is exactly one unambiguous output.
		if outputs, ok := raw["outputs"].([]any); ok && len(outputs) == 1 {
			if output, ok := outputs[0].(map[string]any); ok {
				value = output["usage"]
			}
		}
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	input, inputOK := adobeUsageNumber(m["input_tokens"])
	output, outputOK := adobeUsageNumber(m["output_tokens"])
	total, totalOK := adobeUsageNumber(m["total_tokens"])
	if !inputOK || !outputOK || !totalOK || input+output != total {
		return nil
	}
	inputDetails, inputOK := parseAdobeTokenDetails(m["input_tokens_details"], input)
	outputDetails, outputOK := parseAdobeTokenDetails(m["output_tokens_details"], output)
	if !inputOK || !outputOK {
		return nil
	}
	return &adobeImageUsage{InputTokens: input, OutputTokens: output, TotalTokens: total,
		InputTokensDetails: inputDetails, OutputTokensDetails: outputDetails}
}

func addAdobeTokenDetails(a, b *adobeImageTokenDetails) *adobeImageTokenDetails {
	if a == nil || b == nil {
		return nil
	}
	addKnown := func(a, b *int64) *int64 {
		if a == nil || b == nil {
			return nil
		}
		n := *a + *b
		return &n
	}
	details := &adobeImageTokenDetails{ImageTokens: addKnown(a.ImageTokens, b.ImageTokens), TextTokens: addKnown(a.TextTokens, b.TextTokens)}
	if details.ImageTokens == nil && details.TextTokens == nil {
		return nil
	}
	return details
}

func aggregateAdobeImageUsage(results []*adobe.GenerateResult) *adobeImageUsage {
	var total *adobeImageUsage
	for _, result := range results {
		if result == nil {
			return nil
		}
		usage := parseAdobeImageUsage(result.Raw)
		if usage == nil {
			return nil // A partial batch is not the batch's actual total.
		}
		if total == nil {
			total = usage
			continue
		}
		if usage.TotalTokens > maxAdobeUsageTokens-total.TotalTokens {
			return nil
		}
		total.InputTokens += usage.InputTokens
		total.OutputTokens += usage.OutputTokens
		total.TotalTokens += usage.TotalTokens
		total.InputTokensDetails = addAdobeTokenDetails(total.InputTokensDetails, usage.InputTokensDetails)
		total.OutputTokensDetails = addAdobeTokenDetails(total.OutputTokensDetails, usage.OutputTokensDetails)
	}
	return total
}
