package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"strings"
	"time"
)

const OpsMetricAdobeErrorAccounts = "adobe_401_408_accounts"

// Private transport: never expose this value in rule filters or API responses,
// and never reuse it for signup, password reset, reports, or other alert metrics.
const SettingKeyOpsAdobeAlertMail = "ops_adobe_alert_mail_private"

type OpsAdobeAlertMail struct {
	Recipient string     `json:"recipient"`
	SMTP      SMTPConfig `json:"smtp"`
}

func (s *EmailService) getAdobeAlertMail(ctx context.Context) (*OpsAdobeAlertMail, error) {
	return s.getAdobeAlertMailForRecipient(ctx, "")
}

// Additional transports remain inside the same private setting. Each rule must
// explicitly name its recipient; adding a mailbox never fans out an old rule.
func (s *EmailService) getAdobeAlertMailForRecipient(ctx context.Context, recipient string) (*OpsAdobeAlertMail, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpsAdobeAlertMail)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && strings.TrimSpace(value) == "") {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Adobe alert mail settings unavailable")
	}
	var config struct {
		OpsAdobeAlertMail
		Additional []OpsAdobeAlertMail `json:"additional"`
	}
	if json.Unmarshal([]byte(value), &config) != nil {
		return nil, fmt.Errorf("invalid Adobe alert mail settings")
	}
	selected := &config.OpsAdobeAlertMail
	if recipient = strings.TrimSpace(recipient); recipient != "" {
		selected = nil
		candidates := append([]OpsAdobeAlertMail{config.OpsAdobeAlertMail}, config.Additional...)
		for i := range candidates {
			if strings.EqualFold(strings.TrimSpace(candidates[i].Recipient), recipient) {
				if selected != nil {
					return nil, fmt.Errorf("ambiguous Adobe alert mail recipient")
				}
				selected = &candidates[i]
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("Adobe alert mail recipient is not configured")
		}
	}
	if _, err := OpsAlertEmailRecipient(map[string]any{"email_recipient": selected.Recipient}); err != nil || selected.Recipient == "" || selected.SMTP.Host == "" || selected.SMTP.Port < 1 || selected.SMTP.Port > 65535 {
		return nil, fmt.Errorf("invalid Adobe alert mail settings")
	}
	return selected, nil
}

// This port reads saved business/test telemetry only. It must never probe Adobe,
// refresh credentials, test accounts, or invoke the recovery service.
type OpsAdobeErrorRepository interface {
	GetAdobeErrorAccounts(context.Context, time.Time, time.Time, *int64) ([]OpsAdobeAccountErrors, error)
}

type OpsAdobeErrorPopulationRepository interface {
	GetAdobeErrorAccountPopulation(context.Context, time.Time, time.Time, *int64) ([]OpsAdobeAccountErrors, int64, error)
}

const OpsAdobeAlertRatioFilter = "adobe_error_ratio_percent"

// Missing keeps existing count-only rules compatible. Invalid configuration
// fails closed instead of unexpectedly reverting to the lower count threshold.
func OpsAdobeAlertRatioThreshold(filters map[string]any) (float64, bool, error) {
	if value, exists := filters[OpsAdobeQuietNotificationsFilter]; exists {
		if _, ok := value.(bool); !ok {
			return 0, false, fmt.Errorf("filters.%s must be a boolean", OpsAdobeQuietNotificationsFilter)
		}
	}
	value, exists := filters[OpsAdobeAlertRatioFilter]
	if !exists {
		return 0, false, nil
	}
	var percent float64
	switch v := value.(type) {
	case float64:
		percent = v
	case int:
		percent = float64(v)
	case int64:
		percent = float64(v)
	default:
		return 0, true, fmt.Errorf("filters.%s must be a number between 0 (exclusive) and 100", OpsAdobeAlertRatioFilter)
	}
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent <= 0 || percent > 100 {
		return 0, true, fmt.Errorf("filters.%s must be a number between 0 (exclusive) and 100", OpsAdobeAlertRatioFilter)
	}
	return percent, true, nil
}

type OpsAdobeAccountErrors struct {
	AccountID      int64     `json:"account_id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Count401       int64     `json:"count_401"`
	Count408       int64     `json:"count_408"`
	Auth401        int64     `json:"auth_401"`
	Quota401       int64     `json:"quota_401"`
	Entitled401    int64     `json:"entitled_401"`
	Unknown401     int64     `json:"unknown_401"`
	BusinessErrors int64     `json:"business_errors"`
	TestErrors     int64     `json:"test_errors"`
	TestTimeouts   int64     `json:"test_timeouts"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
}

func (s *OpsAlertEvaluatorService) adobeErrorMetric(ctx context.Context, start, end time.Time, groupID *int64, filters map[string]any) (value float64, detail string, ratioEnabled, ratioBreached bool, err error) {
	percent, enabled, err := OpsAdobeAlertRatioThreshold(filters)
	if err != nil {
		return 0, "", false, false, err
	}
	if enabled {
		repo, ok := s.opsRepo.(OpsAdobeErrorPopulationRepository)
		if !ok {
			return 0, "", true, false, fmt.Errorf("Adobe account population repository unavailable")
		}
		accounts, total, err := repo.GetAdobeErrorAccountPopulation(ctx, start, end, groupID)
		if err != nil {
			return 0, "", true, false, err
		}
		if total < 0 || int64(len(accounts)) > total {
			return 0, "", true, false, fmt.Errorf("inconsistent Adobe account population")
		}
		ratio := float64(0)
		if total > 0 {
			ratio = float64(len(accounts)) * 100 / float64(total)
		}
		detail = buildAdobeErrorAlertDescription(accounts, start, end)
		detail += fmt.Sprintf("\n异常占比：%d / %d = %.2f%%；占比达到 %g%% 且满足账号数量门槛时触发，低于 %g%% 恢复。分母为统计范围内全部未删除的 Adobe 账号，包含暂停和异常账号。", len(accounts), total, ratio, percent, percent)
		return float64(len(accounts)), detail, true, total > 0 && float64(len(accounts))*100 >= float64(total)*percent, nil
	}
	repo, ok := s.opsRepo.(OpsAdobeErrorRepository)
	if !ok {
		return 0, "", false, false, fmt.Errorf("Adobe error summary repository unavailable")
	}
	accounts, err := repo.GetAdobeErrorAccounts(ctx, start, end, groupID)
	if err != nil {
		return 0, "", false, false, err
	}
	return float64(len(accounts)), buildAdobeErrorAlertDescription(accounts, start, end), false, false, nil
}

func buildAdobeErrorAlertDescription(accounts []OpsAdobeAccountErrors, start, end time.Time) string {
	var accounts401, accounts408, count401, count408, auth, quota, entitled, unknown int64
	var businessErrors, testErrors, testTimeouts int64
	for _, a := range accounts {
		if a.Count401 > 0 {
			accounts401++
		}
		if a.Count408 > 0 {
			accounts408++
		}
		count401 += a.Count401
		count408 += a.Count408
		auth += a.Auth401
		quota += a.Quota401
		entitled += a.Entitled401
		unknown += a.Unknown401
		businessErrors += a.BusinessErrors
		testErrors += a.TestErrors
		testTimeouts += a.TestTimeouts
	}
	zone := time.FixedZone("UTC+8", 8*60*60)
	var b strings.Builder
	fmt.Fprintf(&b, "Adobe 账号批量异常（已有业务及账号检测记录）\n统计窗口：%s 至 %s（北京时间）\n受影响账号：%d 个（401/408 合并去重）\n401：%d 个账号，%d 条错误记录；408：%d 个账号，%d 条错误记录。\n401 原因：鉴权失败 %d 条、额度不足 %d 条、权益不足 %d 条、原因未确认 %d 条。\n", start.In(zone).Format("2006-01-02 15:04:05"), end.In(zone).Format("15:04:05"), len(accounts), accounts401, count401, accounts408, count408, auth, quota, entitled, unknown)
	fmt.Fprintf(&b, "来源：业务请求 %d 条；账号检测（含批量）%d 条，其中检测超时 %d 条。检测超时按 408 归类，不等于 Adobe 明确返回 HTTP 408。\n", businessErrors, testErrors, testTimeouts)
	b.WriteString("同一账号可同时出现在两种状态码和多个来源中，触发阈值按账号合并去重。包含换号成功或后续检测恢复之前的失败，不代表这些账号现在全部不可用。408/超时应先排查临时上游故障或网络；401 应先区分额度、权益及鉴权，告警不会触发检测、重登或 Cookie 刷新。\n账号明细（当前状态仅供参考）：\n")
	for i, a := range accounts {
		if i >= 100 {
			fmt.Fprintf(&b, "其余 %d 个账号未在邮件中展开，请结合业务错误日志及账号检测结果排查。\n", len(accounts)-i)
			break
		}
		name := strings.Join(strings.Fields(a.Name), " ")
		runes := []rune(name)
		if len(runes) > 80 {
			name = string(runes[:80]) + "…"
		}
		fmt.Fprintf(&b, "#%d %s | 401=%d，408=%d | 业务=%d，检测=%d（检测超时=%d） | 状态=%s | 最近错误 %s\n", a.AccountID, name, a.Count401, a.Count408, a.BusinessErrors, a.TestErrors, a.TestTimeouts, a.Status, a.LastSeen.In(zone).Format("15:04:05"))
	}
	b.WriteString("查看位置：业务请求在 Sub2 管理后台 → 运维监控 → 错误日志；账号检测在账号管理查看最新检测结果（可能已被后续检测更新，以本邮件窗口记录为准）。")
	return b.String()
}

// A per-rule recipient prevents enabling unrelated global alert emails.
func OpsAlertEmailRecipient(filters map[string]any) (string, error) {
	value, exists := filters["email_recipient"]
	if !exists {
		return "", nil
	}
	address, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("filters.email_recipient must be an email address")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return "", nil
	}
	parsed, err := mail.ParseAddress(address)
	if err != nil || len(address) > 254 || parsed.Address != address {
		return "", fmt.Errorf("filters.email_recipient must be an email address")
	}
	return address, nil
}
