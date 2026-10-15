package service

import "math/rand/v2"

// simulatedAdobeImageUsage is response-only sample data requested by the site
// operator. It is neither measured usage nor a token estimate, and must always
// be accompanied by usage_source=simulated. Never pass it to billing or health
// evidence. The range applies to the whole response, including multi-image calls.
func simulatedAdobeImageUsage(hasInputImage bool) *adobeImageUsage {
	return buildSimulatedAdobeImageUsage(hasInputImage, rand.Int64N)
}

func buildSimulatedAdobeImageUsage(hasInputImage bool, randomN func(int64) int64) *adobeImageUsage {
	total := int64(1800) + randomN(1201)
	input := int64(2) + randomN(total-2)
	output := total - input
	inputImage := int64(0)
	if hasInputImage {
		inputImage = 1 + randomN(input-1)
	}
	inputText, outputText := input-inputImage, int64(0)
	return &adobeImageUsage{
		InputTokens: input, OutputTokens: output, TotalTokens: total,
		InputTokensDetails:  &adobeImageTokenDetails{ImageTokens: &inputImage, TextTokens: &inputText},
		OutputTokensDetails: &adobeImageTokenDetails{ImageTokens: &output, TextTokens: &outputText},
	}
}
