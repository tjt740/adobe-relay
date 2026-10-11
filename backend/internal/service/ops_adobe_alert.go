package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpsAdobeAlertMail)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && strings.TrimSpace(value) == "") {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Adobe alert mail settings unavailable")
	}
	var config OpsAdobeAlertMail
	if json.Unmarshal([]byte(value), &config) != nil {
		return nil, fmt.Errorf("invalid Adobe alert mail settings")
	}
	if _, err := OpsAlertEmailRecipient(map[string]any{"email_recipient": config.Recipient}); err != nil || config.Recipient == "" || config.SMTP.Host == "" || config.SMTP.Port < 1 || config.SMTP.Port > 65535 {
		return nil, fmt.Errorf("invalid Adobe alert mail settings")
	}
	return &config, nil
}

// This port reads saved business telemetry only. It must never probe Adobe,
// refresh credentials, test accounts, or invoke the recovery service.
type OpsAdobeErrorRepository interface {
	GetAdobeErrorAccounts(context.Context, time.Time, time.Time, *int64) ([]OpsAdobeAccountErrors, error)
}

type OpsAdobeAccountErrors struct {
	AccountID   int64
	Name        string
	Status      string
	Count401    int64
	Count408    int64
	Auth401     int64
	Quota401    int64
	Entitled401 int64
	Unknown401  int64
	FirstSeen   time.Time
	LastSeen    time.Time
}

func (s *OpsAlertEvaluatorService) adobeErrorMetric(ctx context.Context, start, end time.Time, groupID *int64) (float64, string, error) {
	repo, ok := s.opsRepo.(OpsAdobeErrorRepository)
	if !ok {
		return 0, "", fmt.Errorf("Adobe error summary repository unavailable")
	}
	accounts, err := repo.GetAdobeErrorAccounts(ctx, start, end, groupID)
	if err != nil {
		return 0, "", err
	}
	return float64(len(accounts)), buildAdobeErrorAlertDescription(accounts, start, end), nil
}

func buildAdobeErrorAlertDescription(accounts []OpsAdobeAccountErrors, start, end time.Time) string {
	var accounts401, accounts408, count401, count408, auth, quota, entitled, unknown int64
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
	}
	zone := time.FixedZone("UTC+8", 8*60*60)
	var b strings.Builder
	fmt.Fprintf(&b, "Adobe 账号批量异常（仅统计已有业务记录）\n统计窗口：%s 至 %s（北京时间）\n受影响账号：%d 个（401/408 合并去重）\n401：%d 个账号，%d 条错误记录；408：%d 个账号，%d 条错误记录。\n401 原因：鉴权失败 %d 条、额度不足 %d 条、权益不足 %d 条、原因未确认 %d 条。\n", start.In(zone).Format("2006-01-02 15:04:05"), end.In(zone).Format("15:04:05"), len(accounts), accounts401, count401, accounts408, count408, auth, quota, entitled, unknown)
	b.WriteString("同一账号可能同时出现两种状态码；包含换号后请求成功之前的失败记录，不代表这些账号现在全部不可用。408 为临时上游故障；401 应先区分额度、权益及鉴权，告警不会触发重登。\n账号明细（当前状态仅供参考）：\n")
	for i, a := range accounts {
		if i >= 100 {
			fmt.Fprintf(&b, "其余 %d 个账号请在运维错误日志查看。\n", len(accounts)-i)
			break
		}
		name := strings.Join(strings.Fields(a.Name), " ")
		runes := []rune(name)
		if len(runes) > 80 {
			name = string(runes[:80]) + "…"
		}
		fmt.Fprintf(&b, "#%d %s | 401=%d，408=%d | 状态=%s | 最近错误 %s\n", a.AccountID, name, a.Count401, a.Count408, a.Status, a.LastSeen.In(zone).Format("15:04:05"))
	}
	b.WriteString("查看位置：Sub2 管理后台 → 运维监控 → 错误日志（选择对应时间范围）。")
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
