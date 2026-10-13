package service

import (
	"context"
	"fmt"
	"time"
)

func adobeHealthSourceLabel(source string) string {
	switch source {
	case "business":
		return "真实业务请求"
	case "okad":
		return "Cookie 推送后自动检测"
	case "test":
		return "手动 / 批量检测"
	default:
		return "升级前历史记录"
	}
}

// Explicit admin action: send a current factual report only to the dedicated
// configured recipient. It neither changes incident state nor fabricates errors.
func (s *OpsService) SendAdobeHealthReport(ctx context.Context) error {
	s.adobeReportMu.Lock()
	defer s.adobeReportMu.Unlock()
	if time.Since(s.adobeReportSentAt) < time.Minute {
		return fmt.Errorf("报表刚刚发送，请一分钟后再试")
	}
	if s.adobeReportEmail == nil {
		return fmt.Errorf("报表邮件服务不可用")
	}
	config, err := s.adobeReportEmail.getAdobeAlertMail(ctx)
	if err != nil {
		return err
	}
	if config == nil {
		return fmt.Errorf("尚未配置 Adobe 专属告警邮箱")
	}
	rules, err := s.opsRepo.ListAlertRules(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.Enabled && adobeQuietNotifications(rule) {
			return fmt.Errorf("系统仅在持续异常确认后自动发送完整报表，同一事件不重复发送")
		}
	}
	report, err := s.AdobeHealthReport(ctx)
	if err != nil {
		return err
	}
	event := &OpsAlertEvent{Status: "report", FiredAt: report.GeneratedAt, Dimensions: map[string]any{"adobe_health": report}}
	if err = s.adobeReportEmail.SendEmailWithConfig(&config.SMTP, config.Recipient, "[Sub2告警] Adobe 账号状态报表（人工发送）", buildAdobeHealthEmail(&OpsAlertRule{}, event)); err != nil {
		return fmt.Errorf("邮件服务未确认发送成功，请稍后重试")
	}
	s.adobeReportSentAt = time.Now()
	return nil
}

func adobeHealthStageLabel(stage string) string {
	switch stage {
	case "token":
		return "登录凭据验证阶段"
	case "prepare":
		return "准备请求阶段"
	case "upload":
		return "上传参考图阶段"
	case "submit":
		return "提交生成任务阶段"
	case "poll":
		return "等待生成结果阶段"
	case "download":
		return "下载结果阶段"
	default:
		return ""
	}
}
