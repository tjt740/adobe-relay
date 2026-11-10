package service

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Missing cells inherit existing billing. Pricing never changes generation quality.
func image26PriceModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "gpt-image-2.6-flare", "gpt-image-2.6-star-swirl", "firefly-gpt-image-2-6-flare":
		return "gpt-image-2.6-flare"
	case "gpt-image-2.6-sunburst", "gpt-image-2.6-aurora", "firefly-gpt-image-2-6-sunburst":
		return "gpt-image-2.6-sunburst"
	default:
		return ""
	}
}

func CloneImageQualityPrices(in map[string]map[string]map[string]float64) map[string]map[string]map[string]float64 {
	if in == nil {
		return nil
	}
	out := make(map[string]map[string]map[string]float64, len(in))
	for model, sizes := range in {
		out[model] = make(map[string]map[string]float64, len(sizes))
		for size, qualities := range sizes {
			out[model][size] = make(map[string]float64, len(qualities))
			for quality, price := range qualities {
				out[model][size][quality] = price
			}
		}
	}
	return out
}

func validateImageQualityPrices(platform string, in map[string]map[string]map[string]float64) (map[string]map[string]map[string]float64, error) {
	invalid := func(message string) (map[string]map[string]map[string]float64, error) {
		return nil, infraerrors.BadRequest("INVALID_IMAGE_QUALITY_PRICE", message)
	}
	if len(in) == 0 {
		return CloneImageQualityPrices(in), nil
	}
	if platform != PlatformAdobe {
		return invalid("image_quality_prices requires an Adobe group")
	}
	for model, sizes := range in {
		if image26PriceModel(model) != model {
			return invalid("image_quality_prices model must be gpt-image-2.6-flare or gpt-image-2.6-sunburst")
		}
		for size, qualities := range sizes {
			if size != "1K" && size != "2K" && size != "4K" {
				return invalid("image_quality_prices resolution must be 1K, 2K or 4K")
			}
			for quality, price := range qualities {
				switch quality {
				case "low", "medium", "high", "xhigh", "max":
				default:
					return invalid("image_quality_prices quality must be low, medium, high, xhigh or max")
				}
				if math.IsNaN(price) || math.IsInf(price, 0) || price < 0 || price > 1e8 {
					return invalid("image quality price must be a finite USD amount between 0 and 100000000")
				}
			}
		}
	}
	return CloneImageQualityPrices(in), nil
}

func lookupImageQualityPrice(prices map[string]map[string]map[string]float64, model, size, quality string) (float64, bool) {
	model = image26PriceModel(model)
	if model == "" {
		return 0, false
	}
	price, ok := prices[model][size][quality]
	return price, ok && price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func adobeImage26BillingQuality(conf adobe.ImageModelConf, requested string) string {
	if image26PriceModel(conf.UpstreamModelVersion) == "" {
		return ""
	}
	return adobeImageResponseQuality(conf, requested)
}

// JSON null cells mean unset, never free. Explicit numeric zero stays billable at zero.
type ImageQualityPriceInput map[string]map[string]map[string]float64

func (p *ImageQualityPriceInput) UnmarshalJSON(data []byte) error {
	var raw map[string]map[string]map[string]*float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		*p = nil
		return nil
	}
	out := make(ImageQualityPriceInput)
	for model, sizes := range raw {
		for size, qualities := range sizes {
			for quality, price := range qualities {
				if price == nil {
					continue
				}
				if out[model] == nil {
					out[model] = make(map[string]map[string]float64)
				}
				if out[model][size] == nil {
					out[model][size] = make(map[string]float64)
				}
				out[model][size][quality] = *price
			}
		}
	}
	*p = out
	return nil
}
