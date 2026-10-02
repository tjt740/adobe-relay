package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// AdobeImageService 把一次 OpenAI 形状的出图请求翻译成 Adobe Firefly 直连调用，
// 再把产物翻译回 OpenAI 形状的响应。
//
// 它不做账号选择与 failover——那是 handler 的职责；本服务只负责「给定账号和 token
// 完成一次出图」，因此可以脱离 gin 与调度器单测。
type AdobeImageService struct {
	clients *adobeClientCache
	// resolveStorage 为 nil 或返回未启用时，响应回落 b64_json。
	resolveStorage ImageStorageResolver
	// inputClient 抓取图生图的源图；见 adobe_image_input.go 的 SSRF 说明。
	inputClient *http.Client
}

func NewAdobeImageService(resolveStorage ImageStorageResolver) *AdobeImageService {
	return &AdobeImageService{
		clients:        &adobeClientCache{},
		resolveStorage: resolveStorage,
		inputClient:    newAdobeInputImageClient(),
	}
}

// AdobeImageResult 是一次出图的完整产出。
type AdobeImageResult struct {
	// Body 是可直接写给客户端的 OpenAI 形状响应。
	Body []byte
	// Images 是转码后的原始图片字节，供 Gemini generateContent 信封使用。
	// Body 可能被对象存储改写成 url，Images 仍保留字节。
	Images [][]byte
	// Forward 交给 RecordUsage 记账；计费只认 ImageCount 与 ImageSize 两个字段。
	Forward *OpenAIForwardResult
	Timings AdobeImageTimings
}

// Wall-clock stage durations. Per-image timings overlap when n > 1 and must not
// be added together to estimate request latency.
type AdobeImageTimings struct {
	SourceUploadMS  int64                     `json:"source_upload_ms"`
	GenerationMS    int64                     `json:"generation_ms"`
	TranscodeMS     int64                     `json:"transcode_ms"`
	StorageMS       int64                     `json:"storage_ms"`
	EncodeMS        int64                     `json:"encode_ms"`
	Delivery        string                    `json:"delivery"`
	StorageFallback bool                      `json:"storage_fallback"`
	Images          []adobe.GenerationTimings `json:"images"`
}

// adobeImageDetachedTimeout 是提交之后（提交重试、轮询、下载、转存）脱离客户端连接的总时限。
// 覆盖 3 次提交各 60s、DefaultImageTimeout 轮询与下载，留出余量。
const adobeImageDetachedTimeout = 10 * time.Minute

// AdobeImageCall 是一次客户端请求在多次换号尝试之间共享的状态。
//
// 输入图 URL 只抓一次：换号时复用已取回的字节，只重新上传（上传得到的 image id 与账号绑定）。
type AdobeImageCall struct {
	Request *OpenAIImagesRequest
	// ChannelMappedModel 非空时替代 Request.Model 参与账号模型映射与计费模型。
	ChannelMappedModel string
	// AspectRatio 是 Gemini imageConfig.aspectRatio；空时 OpenAI Images 路径仍只靠 Size。
	AspectRatio string
	// ImageSize 是 Gemini imageConfig.imageSize（"1K"/"2K"/"4K"）；空时不覆盖 Size 推导。
	ImageSize string

	inputsOnce sync.Once
	inputs     []*adobeInputImage
	inputsErr  error
}

// NewAdobeImageCall 构造请求级出图状态。
func NewAdobeImageCall(req *OpenAIImagesRequest, channelMappedModel string) *AdobeImageCall {
	return &AdobeImageCall{Request: req, ChannelMappedModel: strings.TrimSpace(channelMappedModel)}
}

func (call *AdobeImageCall) imageRequest(upstreamModelID string) adobe.ImageRequest {
	req := adobe.ImageRequest{ModelID: upstreamModelID}
	if call == nil {
		return req
	}
	if call.Request != nil {
		req.Size = call.Request.Size
	}
	req.Ratio = strings.TrimSpace(call.AspectRatio)
	if size := strings.TrimSpace(call.ImageSize); size != "" {
		req.Resolution = adobe.OutputResolution(size)
	}
	return req
}

// inputImages 懒加载 JSON 体里的输入图 URL；首个调用的 ctx 决定抓取是否被取消。
func (call *AdobeImageCall) inputImages(ctx context.Context, client *http.Client) ([]*adobeInputImage, error) {
	call.inputsOnce.Do(func() {
		images := make([]*adobeInputImage, 0, len(call.Request.InputImageURLs))
		for _, rawURL := range call.Request.InputImageURLs {
			image, err := fetchAdobeInputImage(ctx, client, rawURL)
			if err != nil {
				// 取图失败是请求本身的问题，换账号也救不了。
				// 详细原因只进 Message（日志）；对外文案固定，避免把内网连通性等细节回显给调用方。
				call.inputsErr = adobeInputImageRequestError(err)
				return
			}
			images = append(images, image)
		}
		call.inputs = images
	})
	return call.inputs, call.inputsErr
}

// Generate 用指定账号完成一次出图（单次调用，不跨换号复用输入图）。
func (s *AdobeImageService) Generate(
	ctx context.Context, account *Account, token string, req *OpenAIImagesRequest,
) (*AdobeImageResult, error) {
	return s.GenerateCall(ctx, account, token, NewAdobeImageCall(req, ""))
}

// GenerateCall 用指定账号完成一次出图。
//
// 错误一律是 adobe 包的类型化错误或本包的 *adobe.RequestError（终态），
// 交由 classifyAdobeError 翻译成网关语义。
//
// ctx 取消只影响提交之前的阶段（取图、上传）：一旦提交，上游已开始消耗 credits，
// 之后的轮询与下载脱离客户端连接，保证产物能交回 handler 记账。
func (s *AdobeImageService) GenerateCall(
	ctx context.Context, account *Account, token string, call *AdobeImageCall,
) (*AdobeImageResult, error) {
	var req *OpenAIImagesRequest
	if call != nil {
		req = call.Request
	}
	if account == nil {
		return nil, adobe.NewRequestError("adobe account is required")
	}
	if req == nil {
		return nil, adobe.NewRequestError("images request is required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, adobe.NewAuthError("adobe access token is empty", http.StatusUnauthorized)
	}
	n, outputFormat, err := validateAdobeImageParams(req)
	if err != nil {
		return nil, err
	}
	startTime := time.Now()

	requestedModel := strings.TrimSpace(req.Model)
	if call.ChannelMappedModel != "" {
		requestedModel = call.ChannelMappedModel
	}
	upstreamModelID := account.GetMappedModel(requestedModel)
	conf, err := adobe.ResolveImage(call.imageRequest(upstreamModelID))
	if err != nil {
		return nil, &adobe.OperationError{Stage: "prepare", Err: err}
	}

	client := s.clients.clientForAccount(account)
	var timings AdobeImageTimings
	stageStarted := time.Now()
	sourceImageIDs, err := s.uploadSourceImages(ctx, client, token, call)
	timings.SourceUploadMS = time.Since(stageStarted).Milliseconds()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		// 客户端在提交前离开：还没花 credits，直接放弃。
		return nil, err
	}

	upstreamCtx, cancelUpstream := context.WithTimeout(context.WithoutCancel(ctx), adobeImageDetachedTimeout)
	defer cancelUpstream()
	stageStarted = time.Now()
	images, imageTimings, err := generateAdobeImages(upstreamCtx, client, token, account.GetCredential("arp_session_id"), adobe.ImagePayloadOptions{
		Prompt:               req.Prompt,
		AspectRatio:          conf.AspectRatio,
		OutputResolution:     conf.OutputResolution,
		UpstreamModelID:      conf.UpstreamModelID,
		UpstreamModelVersion: conf.UpstreamModelVersion,
		PayloadKind:          conf.PayloadKind,
		SizePixels:           conf.SizePixels,
		QualityLevel:         req.Quality,
		SourceImageIDs:       sourceImageIDs,
		Edit:                 req.IsEdits(),
		Background:           req.Background,
	}, n)
	timings.GenerationMS = time.Since(stageStarted).Milliseconds()
	timings.Images = imageTimings
	if err != nil {
		return nil, err
	}
	stageStarted = time.Now()
	var conversions errgroup.Group
	conversions.SetLimit(adobeFanOutConcurrency)
	for i := range images {
		conversions.Go(func() error {
			images[i] = applyAdobeOutputFormat(images[i], outputFormat, req.OutputCompression)
			return nil
		})
	}
	_ = conversions.Wait() // Conversion failures already fall back to original bytes.
	timings.TranscodeMS = time.Since(stageStarted).Milliseconds()

	requestID := uuid.NewString()
	body, err := s.buildResponseBody(upstreamCtx, requestID, images, adobeImageResponseQuality(conf, req.Quality), req.ResponseFormat, &timings)
	if err != nil {
		return nil, err
	}

	return &AdobeImageResult{
		Body:    body,
		Images:  images,
		Timings: timings,
		Forward: &OpenAIForwardResult{
			RequestID:  requestID,
			Model:      requestedModel,
			ImageCount: n,
			// 计费档位：adobe 的 OutputResolution 取值就是 "1K"/"2K"/"4K"，
			// ClassifyImageBillingTier 直接认这三个字面量。
			ImageSize:        string(conf.OutputResolution),
			UpstreamModel:    conf.ModelID,
			UpstreamEndpoint: adobe.ImageSubmitURL,
			Duration:         time.Since(startTime),
		},
	}, nil
}

// generateAdobeImages 把 OpenAI 的 n 拆成 n 次 Firefly 任务（每次上游仍是 n=1）。
// 全成或全败：任一张最终失败则整单失败。
func generateAdobeImages(
	ctx context.Context,
	client *adobe.Client,
	token string,
	arpSessionID string,
	opts adobe.ImagePayloadOptions,
	n int,
) ([][]byte, []adobe.GenerationTimings, error) {
	images := make([][]byte, n)
	timings := make([]adobe.GenerationTimings, n)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(adobeFanOutConcurrency)
	baseSeed := adobe.SeedNow()
	for i := range n {
		group.Go(func() error {
			itemOpts := opts
			seed := baseSeed + i
			itemOpts.Seed = &seed
			generated, err := client.GenerateImage(groupCtx, adobe.GenerateImageInput{
				Token:        token,
				ARPSessionID: arpSessionID,
				Options:      itemOpts,
			})
			if err != nil {
				return err
			}
			if generated == nil || len(generated.Bytes) == 0 {
				// 上游声称完成却没给出图片：按临时故障处理，交给 failover 换号，而不是写空响应。
				return adobe.NewUpstreamTemporaryError("adobe returned an empty image", 0, adobe.ErrorTypeStatus)
			}
			images[i] = generated.Bytes
			timings[i] = generated.Timings
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, nil, err
	}
	return images, timings, nil
}

// uploadSourceImages 把图生图的源图上传到 Adobe，返回可放进 payload 的 image id。
// 文生图时返回 nil。OpenAI mask 不上传：3p Image Edit 抓包没有 usage=mask blob，
// 真 mask 由 handler 优先转到 API key 中转号。
func (s *AdobeImageService) uploadSourceImages(
	ctx context.Context, client *adobe.Client, token string, call *AdobeImageCall,
) ([]string, error) {
	req := call.Request
	if len(req.Uploads) == 0 && len(req.InputImageURLs) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(req.Uploads)+len(req.InputImageURLs))
	// multipart 上传的图已经是现成字节，直接转发。
	for _, upload := range req.Uploads {
		if len(upload.Data) == 0 {
			continue
		}
		id, err := client.UploadImage(ctx, token, upload.Data, upload.ContentType)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	// JSON 体里的 URL 需要先取回字节；同一请求换号时复用已取回的字节。
	images, err := call.inputImages(ctx, s.inputClient)
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		id, err := client.UploadImage(ctx, token, image.Data, image.ContentType)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return ids, nil
}

// buildResponseBody 构造 OpenAI 形状的响应。
//
// URL delivery uploads existing bytes directly. Base64 is encoded only if needed
// by the client or as a fallback when storage is disabled/unavailable.
func (s *AdobeImageService) buildResponseBody(ctx context.Context, requestID string, images [][]byte, quality, responseFormat string, timings *AdobeImageTimings) ([]byte, error) {
	if len(images) == 0 {
		return nil, adobe.NewRequestError("adobe returned an empty image")
	}
	for _, image := range images {
		if len(image) == 0 {
			return nil, adobe.NewRequestError("adobe returned an empty image")
		}
	}
	var urls []string
	if !strings.EqualFold(strings.TrimSpace(responseFormat), "b64_json") && s.resolveStorage != nil {
		started := time.Now()
		if uploader, enabled := s.resolveStorage(); enabled && uploader != nil {
			var err error
			urls, err = uploader.UploadImages(ctx, requestID, images)
			if err != nil {
				// Already generated/charged: retain every image on storage failure.
				timings.StorageFallback = true
				urls = nil
			}
		}
		timings.StorageMS = time.Since(started).Milliseconds()
	}
	started := time.Now()
	defer func() { timings.EncodeMS = time.Since(started).Milliseconds() }()
	data := make([]any, len(images))
	timings.Delivery = "b64_json"
	if len(urls) == len(images) {
		timings.Delivery = "storage_url"
		for i, url := range urls {
			data[i] = map[string]any{"url": url}
		}
	} else {
		for i, image := range images {
			data[i] = map[string]any{"b64_json": base64.StdEncoding.EncodeToString(image)}
		}
	}
	payload := map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
	}
	if quality != "" {
		payload["quality"] = quality
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, adobe.NewRequestError(fmt.Sprintf("encode images response: %v", err))
	}

	return body, nil
}
