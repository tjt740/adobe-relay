package service

import (
	"bytes"
	"fmt"
	"image"
	"net/url"
	"strings"
)

type adobeImageResponseMetadata struct {
	Request      *OpenAIImagesRequest
	GPTImage     bool
	UpstreamURLs []string
	Usage        *adobeImageUsage
}

func validAdobeImageURLs(urls []string, count int) bool {
	if len(urls) != count || count == 0 {
		return false
	}
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return false
		}
	}
	return true
}

func (m adobeImageResponseMetadata) addTo(payload map[string]any, images [][]byte) {
	// Preserve verified upstream counts, including zero and partial details.
	// Only the response gets synthetic fallback counts; billing stays separate.
	usage, source := m.Usage, "upstream"
	if usage == nil {
		hasInputImage := m.Request != nil && (len(m.Request.InputImageURLs) > 0 || len(m.Request.Uploads) > 0)
		usage, source = simulatedAdobeImageUsage(hasInputImage), "simulated"
	}
	payload["usage"], payload["usage_source"] = usage, source
	if m.Request != nil {
		payload["model"] = m.Request.Model
		if m.GPTImage {
			background := strings.ToLower(strings.TrimSpace(m.Request.Background))
			if background == "" {
				background = "auto"
			}
			if background == "auto" || background == "opaque" || background == "transparent" {
				payload["background"] = background
			}
			// Firefly's existing safety checks apply. The client's moderation
			// parameter is not forwarded, so never claim that "low" took effect.
			payload["moderation"] = "auto"
		}
	}
	// Describe delivered bytes, not requested size/format: models may choose
	// their own size, and the existing transcoder can fall back to the original.
	size, format := "", ""
	for i, data := range images {
		cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return
		}
		actualSize := fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
		if i == 0 {
			size, format = actualSize, kind
		} else {
			if size != actualSize {
				size = ""
			}
			if format != kind {
				format = ""
			}
		}
	}
	if size != "" {
		payload["size"] = size
	}
	if format != "" {
		payload["output_format"] = format
	}
}
