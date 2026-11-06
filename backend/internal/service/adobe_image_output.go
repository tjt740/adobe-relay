package service

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/tidwall/gjson"
)

const (
	adobeMaxBatchCount      = 10
	adobeFanOutConcurrency  = 2
	adobeJPEGDefaultQuality = 100
	adobeOutputFormatPNG    = "png"
	adobeOutputFormatJPEG   = "jpeg"
	adobeOutputFormatGIF    = "gif"
	adobeGIFMinFrameCount   = 2
	adobeGIFMaxFrameCount   = 16
	adobeGIFMaxFixedFrames  = 9
	adobeGIFDefaultFrames   = 9
	adobeGIFMinDurationMS   = 20
	adobeGIFMaxDurationMS   = 655350
	adobeGIFDefaultDuration = 200
	adobeGIFSize            = "896x896"
)

func adobeImageRequestCount(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

func validateAdobeImageParams(req *OpenAIImagesRequest) (int, string, error) {
	n := adobeImageRequestCount(req.N)
	if n > adobeMaxBatchCount {
		return 0, "", adobe.NewParameterError(
			fmt.Sprintf("adobe channel n must be between 1 and %d, got n=%d", adobeMaxBatchCount, n), "invalid_n", "n")
	}
	format, err := normalizeAdobeOutputFormat(req.OutputFormat)
	if err != nil {
		return 0, "", err
	}
	if format == adobeOutputFormatJPEG && strings.EqualFold(strings.TrimSpace(req.Background), "transparent") {
		return 0, "", adobe.NewParameterError(
			"adobe channel does not support output_format=jpeg with background=transparent", "incompatible_image_background", "background")
	}
	return n, format, nil
}

func normalizeAdobeOutputFormat(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case adobeOutputFormatPNG:
		return adobeOutputFormatPNG, nil
	case adobeOutputFormatJPEG, "jpg":
		return adobeOutputFormatJPEG, nil
	case adobeOutputFormatGIF:
		return adobeOutputFormatGIF, nil
	default:
		return "", adobe.NewParameterError(
			fmt.Sprintf("adobe channel does not support output_format=%q", strings.TrimSpace(raw)), "unsupported_output_format", "output_format")
	}
}

type adobeGIFOptions struct {
	FrameCount      int
	FrameDurationMS int
}

func validateAdobeComplexity(req *OpenAIImagesRequest, conf adobe.ImageModelConf) (string, error) {
	value := strings.ToLower(strings.TrimSpace(req.Complexity))
	if value == "" {
		return "", nil
	}
	if conf.PayloadKind != adobe.PayloadKindGPTImage26 {
		return "", adobe.NewParameterError("complexity is supported only by gpt-image-2.6-star-swirl and gpt-image-2.6-aurora", "complexity_model_not_supported", "complexity")
	}
	switch value {
	case "low", "medium", "high":
		return value, nil
	default:
		return "", adobe.NewParameterError("complexity must be low, medium, or high", "invalid_complexity", "complexity")
	}
}

// validateAdobeGIFParams applies the public GIF contract only after the model
// has been resolved. The older GPT Image 2/2.5 paths intentionally never enter
// this branch, so their output and option handling remain unchanged.
func validateAdobeGIFParams(req *OpenAIImagesRequest, conf adobe.ImageModelConf, outputFormat string) (adobeGIFOptions, error) {
	hasFrameParams := req != nil && (req.FrameCount != nil || req.FrameDurationMS != nil)
	if outputFormat != adobeOutputFormatGIF {
		if hasFrameParams {
			return adobeGIFOptions{}, adobe.NewParameterError(
				"frame_count and frame_duration_ms require output_format=gif", "gif_parameters_require_output_format", "output_format")
		}
		return adobeGIFOptions{}, nil
	}
	if conf.PayloadKind != adobe.PayloadKindGPTImage26 {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"output_format=gif is supported only by gpt-image-2.6-star-swirl and gpt-image-2.6-aurora", "gif_model_not_supported", "output_format")
	}
	if req == nil {
		return adobeGIFOptions{}, adobe.NewParameterError("GIF request is missing", "gif_parameters_invalid", "output_format")
	}
	if req.Stream {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"GIF output does not support streaming", "gif_stream_unsupported", "stream")
	}
	if req.PartialImages != nil && *req.PartialImages > 0 {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"GIF output does not support partial images", "gif_partial_images_unsupported", "partial_images")
	}
	if req.HasMask {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"GIF output does not support masks", "gif_mask_unsupported", "mask")
	}
	if req.IsEdits() || hasAdobeGIFReferenceInput(req) {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"GIF reference editing is not supported on the Adobe channel; use text-only /v1/images/generations for GIF output or png/jpeg for image edits", "gif_edit_unsupported", "image")
	}
	if req.OutputCompression != nil {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"GIF output does not support custom output compression", "gif_output_compression_unsupported", "output_compression")
	}
	autoSize := conf.SizePixels.Width == 0 && conf.SizePixels.Height == 0
	requestedSize := strings.ToLower(strings.TrimSpace(req.Size))
	if (requestedSize != "" && requestedSize != "auto" && requestedSize != adobeGIFSize) ||
		(!autoSize && (conf.SizePixels.Width != 896 || conf.SizePixels.Height != 896)) {
		return adobeGIFOptions{}, adobe.NewParameterError(
			fmt.Sprintf("GIF output requires size=auto or size=%s for Adobe GPT Image 2.6", adobeGIFSize), "gif_size_unsupported", "size")
	}
	frames := adobeGIFDefaultFrames
	if req.FrameCount != nil {
		frames = *req.FrameCount
	}
	if frames < adobeGIFMinFrameCount || frames > adobeGIFMaxFrameCount {
		return adobeGIFOptions{}, adobe.NewParameterError(
			fmt.Sprintf("frame_count must be between %d and %d for GIF output", adobeGIFMinFrameCount, adobeGIFMaxFrameCount), "invalid_frame_count", "frame_count")
	}
	if !autoSize && frames > adobeGIFMaxFixedFrames {
		return adobeGIFOptions{}, adobe.NewParameterError(
			"896x896 GIF output supports at most 9 frames within Adobe's sheet pixel budget; use size=auto for 10..16 frames", "gif_frame_size_budget_exceeded", "frame_count")
	}
	duration := adobeGIFDefaultDuration
	if req.FrameDurationMS != nil {
		duration = *req.FrameDurationMS
	}
	if duration < adobeGIFMinDurationMS {
		return adobeGIFOptions{}, adobe.NewParameterError(
			fmt.Sprintf("frame_duration_ms must be at least %d for GIF output", adobeGIFMinDurationMS), "invalid_frame_duration_ms", "frame_duration_ms")
	}
	// GIF stores a 16-bit delay in centiseconds. Match the Images API cap
	// before rounding so even very large integer inputs cannot overflow.
	if duration > adobeGIFMaxDurationMS {
		duration = adobeGIFMaxDurationMS
	}
	// Adobe/OpenAI encode GIF delays in 10 ms units. Normalize before building
	// the provider payload so the value sent upstream matches the public API.
	duration = ((duration + 5) / 10) * 10
	return adobeGIFOptions{FrameCount: frames, FrameDurationMS: duration}, nil
}

func hasAdobeGIFReferenceInput(req *OpenAIImagesRequest) bool {
	if len(req.Uploads) > 0 || len(req.InputImageURLs) > 0 {
		return true
	}
	// The shared generations parser intentionally ignores edit-only JSON
	// fields. Check them here so an unsupported GIF reference is not silently
	// converted into a paid text-only generation; other providers stay intact.
	if !req.Multipart {
		for _, field := range []string{"image", "images"} {
			value := gjson.GetBytes(req.Body, field)
			if value.Exists() && value.Type != gjson.Null && (!value.IsArray() || len(value.Array()) > 0) {
				return true
			}
		}
	}
	return false
}

// adobeImageResponseQuality 返回网关采用的质量标签，复用 payload 映射处理默认值和模型上限。
// 2.5 的 xhigh / max 仍提交 detailLevel=7，对外分别保留规范化的请求名称。
// 非 GPT Image 模型没有这个生成控制，不回显请求值，以免声称未生效的参数已生效。
func adobeImageResponseQuality(conf adobe.ImageModelConf, requested string) string {
	if !conf.IsGPTImage() {
		return ""
	}
	switch adobe.GPTImageDetailLevelFromQualityForVersion(requested, conf.UpstreamModelVersion) {
	case 7:
		if strings.EqualFold(strings.TrimSpace(requested), "xhigh") {
			return "xhigh"
		}
		return "max"
	case 5:
		return "high"
	case 3:
		return "medium"
	default:
		return "low"
	}
}

func applyAdobeOutputFormat(data []byte, format string, compression *int) []byte {
	converted, err := transcodeAdobeImage(data, format, compression)
	if err != nil {
		slog.Warn("adobe image transcode failed, using original bytes", "error", err, "output_format", format)
		return data
	}
	if len(converted) == 0 {
		return data
	}
	return converted
}

func validateAdobeOutputBytes(data []byte, format string) error {
	if strings.TrimSpace(format) != adobeOutputFormatGIF {
		return nil
	}
	if sniffAdobeImageFormat(data) != adobeOutputFormatGIF {
		return fmt.Errorf("upstream did not return a GIF image")
	}
	return nil
}

func adobeGIFOutputResolution(images [][]byte) (adobe.OutputResolution, error) {
	resolution := adobe.Resolution1K
	for _, data := range images {
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != adobeOutputFormatGIF || cfg.Width <= 0 || cfg.Height <= 0 {
			return "", fmt.Errorf("upstream returned an invalid GIF image")
		}
		tier := adobe.ResolutionFromSize(fmt.Sprintf("%dx%d", cfg.Width, cfg.Height))
		if tier == adobe.Resolution4K || (tier == adobe.Resolution2K && resolution == adobe.Resolution1K) {
			resolution = tier
		}
	}
	return resolution, nil
}

func transcodeAdobeImage(data []byte, format string, compression *int) ([]byte, error) {
	if format == "" || len(data) == 0 {
		return data, nil
	}
	src := sniffAdobeImageFormat(data)
	if format == adobeOutputFormatGIF {
		if src != adobeOutputFormatGIF {
			return nil, fmt.Errorf("upstream returned %q for requested gif", src)
		}
		return data, nil
	}
	if src == format && !adobeShouldRecompressJPEG(src, format, compression) {
		return data, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if format == adobeOutputFormatJPEG && imageHasTransparency(img) {
		return nil, fmt.Errorf("cannot convert transparent image to jpeg")
	}
	return encodeAdobeImage(img, format, compression)
}

func sniffAdobeImageFormat(data []byte) string {
	switch detectedImageContentType(data) {
	case "image/png":
		return adobeOutputFormatPNG
	case "image/jpeg":
		return adobeOutputFormatJPEG
	case "image/gif":
		return adobeOutputFormatGIF
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return adobeOutputFormatPNG
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return adobeOutputFormatJPEG
	}
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return adobeOutputFormatGIF
	}
	return ""
}

func adobeShouldRecompressJPEG(src, format string, compression *int) bool {
	return src == adobeOutputFormatJPEG && format == adobeOutputFormatJPEG &&
		compression != nil && *compression < adobeJPEGDefaultQuality
}

func encodeAdobeImage(img image.Image, format string, compression *int) ([]byte, error) {
	var buf bytes.Buffer
	switch format {
	case adobeOutputFormatPNG:
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	case adobeOutputFormatJPEG:
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: adobeJPEGQuality(compression)}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported output format %q", format)
	}
	return buf.Bytes(), nil
}

func adobeJPEGQuality(compression *int) int {
	if compression == nil {
		return adobeJPEGDefaultQuality
	}
	quality := *compression
	if quality < 1 {
		return 1
	}
	if quality > adobeJPEGDefaultQuality {
		return adobeJPEGDefaultQuality
	}
	return quality
}

func imageHasTransparency(img image.Image) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}
	return false
}
