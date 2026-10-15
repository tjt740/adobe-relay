package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/imagebuffer"
)

// PrepareAdobeImageBuffer reserves disk before submission. Only the native,
// unchanged-byte OpenAI response path opts in: transcodes, Gemini and relay
// responses retain their existing reservation until they have equivalent
// bounded encoders. Failure keeps the original memory-backed behavior.
func PrepareAdobeImageBuffer(ctx context.Context, req *OpenAIImagesRequest) (context.Context, func(), bool) {
	if req == nil || strings.TrimSpace(req.OutputFormat) != "" || req.Stream || req.N > 10 {
		return ctx, func() {}, false
	}
	n := adobeImageRequestCount(req.N)
	// One 64 MiB download and its Base64 JSON (plus metadata/page alignment)
	// per image. Retries reuse failed extents instead of consuming more space.
	a, err := imagebuffer.New(int64(n) * (160 << 20))
	if err != nil {
		return ctx, func() {}, false
	}
	return imagebuffer.WithArena(ctx, a), func() { _ = a.Close() }, true
}

// AdobeImageRetainedBytes accounts for actual retained request allocations.
// Multipart upload slices usually alias Body; decoded transfer encodings do
// not. JSON keeps the previous three-copy allowance for strings/data URLs.
func AdobeImageRetainedBytes(req *OpenAIImagesRequest) int64 {
	if req == nil {
		return 0
	}
	if !req.Multipart {
		size := int64(len(req.Body)) * 3
		for _, rawURL := range req.InputImageURLs {
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "data:") {
				size += adobeInputImageMaxBytes
			}
		}
		return size
	}
	size := int64(len(req.Body) + len(req.Prompt))
	for _, upload := range req.Uploads {
		if !upload.sharesBody {
			size += int64(cap(upload.Data))
		}
	}
	if req.MaskUpload != nil && !req.MaskUpload.sharesBody {
		size += int64(cap(req.MaskUpload.Data))
	}
	return size
}
