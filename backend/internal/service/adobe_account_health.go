package service

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
)

// Only allowlisted classifications are persisted. Raw upstream bodies, URLs,
// cookies and tokens must never enter the account status or email report.
type AdobeHealthObservation struct {
	ProxyID            *int64     `json:"proxy_id,omitempty"`
	SourceID           string     `json:"source_id,omitempty"`
	ProxyUpdatedAt     *time.Time `json:"proxy_updated_at,omitempty"`
	SubmissionRejected bool       `json:"submission_rejected,omitempty"`
	Success            bool       `json:"success"`
	Code               int        `json:"code"`
	Reason             string     `json:"reason"`
	Stage              string     `json:"stage"`
	Source             string     `json:"source"`
	Scope              string     `json:"scope"`
	StartedAt          time.Time  `json:"started_at"`
	CheckedAt          time.Time  `json:"checked_at"`
}

type AdobeHealthRecorder interface {
	RecordAdobeHealth(context.Context, *Account, AdobeHealthObservation) error
}

func adobeHealthObservation(start time.Time, source, model string, err error) AdobeHealthObservation {
	o := AdobeHealthObservation{Success: err == nil, Code: connectionTestStatusCode(err), Source: source, Scope: strings.TrimSpace(model), StartedAt: start.UTC(), CheckedAt: time.Now().UTC()}
	if len(o.Scope) > 200 {
		o.Scope = o.Scope[:200]
	}
	if err == nil {
		return o
	}
	o.Reason = "unknown"
	var failover *UpstreamFailoverError
	var relay *OpenAIImagesUpstreamError
	if errors.As(err, &failover) {
		o.Code = failover.StatusCode
	}
	if errors.As(err, &relay) {
		o.Code = relay.StatusCode
	}

	var op *adobe.OperationError
	if errors.As(err, &op) {
		o.SubmissionRejected = op.SubmissionRejected
		switch op.Stage {
		case "token", "prepare", "upload", "submit", "poll", "download":
			o.Stage = op.Stage
		}
	}
	var quota *adobe.QuotaExhaustedError
	var entitlement *adobe.NotEntitledError
	var auth *adobe.AuthError
	var content *adobe.ContentRejectedError
	var upstream *adobe.RequestError
	var network net.Error
	switch {
	case errors.As(err, &quota):
		o.Reason = "quota"
	case errors.As(err, &entitlement):
		o.Reason = "entitlement"
	case errors.As(err, &auth):
		o.Reason = "auth"
	case errors.As(err, &content):
		o.Reason = "content"
	case errors.Is(err, context.DeadlineExceeded):
		o.Reason = "timeout"
		o.Code = 408
	case errors.As(err, &network):
		if network.Timeout() {
			o.Reason = "timeout"
			o.Code = 408
		} else {
			o.Reason = "network"
		}
	case errors.As(err, &upstream):
		switch upstream.ErrorType {
		case adobe.ErrorTypeTimeout:
			o.Reason = "timeout"
			o.Code = 408
		case adobe.ErrorTypeProxy:
			o.Reason = "proxy"
		case adobe.ErrorTypeNetwork, adobe.ErrorTypeConnection:
			o.Reason = "network"
		default:
			if o.Code == 408 || o.Code == 429 || o.Code >= 500 {
				o.Reason = "upstream"
			}
		}
	default:
		if o.Code == 408 {
			o.Reason = "upstream"
			if failover == nil && relay == nil && !connectionTestHTTPCode.MatchString(err.Error()) {
				o.Reason = "timeout"
			}
		}
	}
	return o
}

type adobeRouteSourceKey struct{}

func recordAdobeHealth(ctx context.Context, repo AccountRepository, a *Account, start time.Time, source, model string, err error) {
	if a == nil || a.Platform != PlatformAdobe || a.IsSyntheticUITest() || errors.Is(err, context.Canceled) {
		return
	}
	recorder, ok := repo.(AdobeHealthRecorder)
	if !ok {
		return
	}
	persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	observation := adobeHealthObservation(start, source, model, err)
	observation.SourceID, _ = ctx.Value(adobeRouteSourceKey{}).(string)
	if e := recorder.RecordAdobeHealth(persist, a, observation); e != nil {
		slog.Warn("adobe_health_save_failed", "account_id", a.ID, "error", e)
	}
}

func (s *GatewayService) RecordAdobeHealth(ctx context.Context, a *Account, start time.Time, model string, err error) {
	recordAdobeHealth(ctx, s.accountRepo, a, start, "business", model, err)
}

func (s *GatewayService) AdobeGenerationRouteContext(ctx context.Context, a *Account) (context.Context, bool) {
	if repo, ok := s.accountRepo.(AdobeRouteReviewRepository); ok {
		source, ready, err := repo.AdobeTestRoute(ctx, a)
		return context.WithValue(ctx, adobeRouteSourceKey{}, source), err == nil && ready
	}
	return ctx, true
}

func AdobeHealthReason(reason string) (string, string) {
	switch reason {
	case "auth":
		return "登录凭据未通过验证", "先查看 Cookie 联动与验证结果；不要反复手动登录。"
	case "quota":
		return "账号额度不足", "检查剩余额度和额度重置时间。"
	case "entitlement":
		return "账号不支持所用模型", "检查套餐及模型权限，换 Cookie 通常无效。"
	case "timeout":
		return "本地等待超时", "先检查代理和网络，再查看上游耗时；不等于 Cookie 失效。"
	case "proxy":
		return "代理连接异常", "检查该账号绑定的代理是否可用。"
	case "network":
		return "网络连接失败", "检查代理、DNS 和上游连接。"
	case "upstream":
		return "上游暂时无法完成请求", "先查看是否有批量超时或上游拥堵，不要批量重登。"
	case "content":
		return "内容被上游拒绝", "调整请求内容，账号不一定有问题。"
	case "legacy":
		return "历史失败，等待成功验证", "旧记录未包含完整原因；下一次真实请求或检测成功后确认恢复。"
	default:
		return "失败原因待确认", "本次没有足够证据判断原因，请查看检测阶段和错误日志。"
	}
}

func adobeHealthTestModel(a *Account, model string) string {
	if strings.TrimSpace(model) == "" {
		if ids := adobe.ImageModelIDs(); len(ids) > 0 {
			model = ids[0]
		}
	}
	mapped := a.GetMappedModel(model)
	if resolved, err := adobe.ResolveImage(adobe.ImageRequest{ModelID: mapped}); err == nil {
		return resolved.Family
	}
	return mapped
}

func (s *AccountTestService) AccountRuntimeRevision(ctx context.Context) (string, error) {
	repo, ok := s.accountRepo.(interface {
		AccountRuntimeRevision(context.Context) (string, error)
	})
	if !ok {
		return "", errors.New("account runtime revision unavailable")
	}
	read, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return repo.AccountRuntimeRevision(read)
}
