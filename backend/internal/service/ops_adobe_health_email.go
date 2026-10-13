package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func buildAdobeHealthEmail(rule *OpsAlertRule, event *OpsAlertEvent) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body style="margin:0;background:#f1f5f9;font-family:Arial,'Microsoft YaHei',sans-serif;color:#1e293b"><div style="max-width:760px;margin:24px auto;background:#fff;border:1px solid #e2e8f0;border-radius:16px;overflow:hidden"><div style="padding:28px;background:#0f172a;color:white"><p style="margin:0 0 8px;color:#94a3b8;font-size:12px">Sub2 · Adobe 账号运行报告</p><h1 style="font-size:23px;margin:0">`)
	title := "账号出现异常，请查看下面的处理建议"
	if event.Status == "report" {
		title = "你要的账号状态报表"
	} else if event.Status == OpsAlertStatusResolved {
		title = "异常比例已回落，已有实际成功记录"
	}
	b.WriteString(title + `</h1></div><div style="padding:24px">`)
	r := adobeReportFromEvent(event)
	if r == nil {
		b.WriteString(`<p>这是一条旧版历史告警，记录的是当时发生过的错误，不能据此判断现在还有多少账号异常。</p><details><summary>展开历史记录</summary><p style="font-size:12px;line-height:1.7">` + strings.ReplaceAll(htmlEscape(event.Description), "\n", "<br>") + `</p></details>`)
	} else {
		zone := time.FixedZone("UTC+8", 8*3600)
		fmt.Fprintf(&b, `<p style="color:#64748b;font-size:12px">数据时间：%s（北京时间）</p><p style="font-size:17px;line-height:1.7"><strong>%d 个账号仍有失败待验证，占全部 %d 个账号的 %.2f%%。</strong><br>这些账号有 401 或 408 记录，尚未取得对应的后续成功证据；不等于所有账号现在都无法使用。</p>`, r.GeneratedAt.In(zone).Format("2006-01-02 15:04:05"), r.Pending, r.Total, r.Ratio)
		fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="12" style="background:#f8fafc;text-align:center;border:1px solid #e2e8f0;border-radius:10px"><tr><td><strong style="font-size:25px;color:#b45309">%d</strong><br>仍待成功验证</td><td><strong style="font-size:25px;color:#047857">%d</strong><br>已有恢复证据</td><td><strong style="font-size:25px">%d</strong><br>尚无完整记录</td></tr></table>`, r.Pending, r.Recovered, r.Unknown)
		fmt.Fprintf(&b, `<p style="font-size:12px;line-height:1.7;color:#64748b">最近 %s–%s，%d 个账号出现过错误，其中 %d 个仍待验证。阈值仍为 %.0f%%，并至少 %d 个账号。分母包含全部未删除 Adobe 账号（含暂停账号）。历史报错数和当前待验证数不相加。新告警按最近窗口内仍待验证的账号触发；已触发的告警会继续等待成功证据。</p>`, r.WindowStart.In(zone).Format("15:04"), r.GeneratedAt.In(zone).Format("15:04"), r.Historical, r.RecentPending, r.Threshold, r.Minimum)
		b.WriteString(`<h2 style="font-size:17px;margin-top:28px">先做什么</h2><table width="100%" cellpadding="10" cellspacing="0" style="border-collapse:collapse;font-size:13px;text-align:left"><tr style="background:#f1f5f9"><th>问题</th><th>账号数</th><th>建议</th></tr>`)
		for _, reason := range []string{"upstream", "timeout", "auth", "quota", "entitlement", "proxy", "network", "content", "legacy", "unknown"} {
			ids := map[int64]bool{}
			for _, a := range r.Accounts {
				if a.Pending && a.Reason == reason {
					ids[a.ID] = true
				}
			}
			if len(ids) == 0 {
				continue
			}
			label, advice := AdobeHealthReason(reason)
			fmt.Fprintf(&b, `<tr><td style="border-bottom:1px solid #e2e8f0">%s</td><td style="border-bottom:1px solid #e2e8f0">%d</td><td style="border-bottom:1px solid #e2e8f0">%s</td></tr>`, label, len(ids), advice)
		}
		b.WriteString(`</table><p style="font-size:12px;color:#64748b">下表也列出其他失败，只有 401 / 408 计入 20% 告警。一个账号可能涉及多个模型和原因，分类数量不直接相加。408 是请求超时或上游临时故障的线索，不能直接认定 Cookie 失效。</p><h2 style="font-size:17px;margin-top:28px">需要关注的账号</h2><table width="100%" cellpadding="9" cellspacing="0" style="border-collapse:collapse;font-size:12px;text-align:left;word-break:break-word"><tr style="background:#f1f5f9"><th>账号</th><th>问题 / 来源</th><th>最近失败<br>后续成功</th></tr>`)
		shown := 0
		totalRows := 0
		for _, a := range r.Accounts {
			if !a.Pending {
				continue
			}
			totalRows++
			if shown >= 12 {
				continue
			}
			shown++
			reason, _ := AdobeHealthReason(a.Reason)
			source := adobeHealthSourceLabel(a.Source)
			if label := adobeHealthStageLabel(a.Stage); label != "" {
				source += " · " + label
			}
			codeLabel := ""
			if a.Code > 0 {
				codeLabel = fmt.Sprintf("（HTTP %d）", a.Code)
			}
			if a.Reason == "timeout" {
				codeLabel = "（本地超时，按 408 统计）"
			}
			failed := "—"
			success := "尚未确认"
			if a.FailedAt != nil {
				failed = a.FailedAt.In(zone).Format("01-02 15:04:05")
			}
			if a.SuccessAt != nil {
				success = a.SuccessAt.In(zone).Format("01-02 15:04:05") + "（不足以确认本次恢复）"
			}
			fmt.Fprintf(&b, `<tr><td style="border-bottom:1px solid #e2e8f0">#%d %s<br><span style="color:#64748b">%s</span></td><td style="border-bottom:1px solid #e2e8f0">%s%s<br>%s</td><td style="border-bottom:1px solid #e2e8f0">%s<br>%s</td></tr>`, a.ID, htmlEscape(a.Name), htmlEscape(a.Scope), reason, codeLabel, source, failed, success)
		}
		if shown == 0 {
			b.WriteString(`<tr><td colspan="3">当前没有失败待验证的记录；未检测账号仍不能视为健康。</td></tr>`)
		}
		b.WriteString(`</table>`)
		if totalRows > shown {
			fmt.Fprintf(&b, `<p>另有 %d 条记录，请在后台查看完整报表。</p>`, totalRows-shown)
		}
		b.WriteString(`<div style="margin-top:24px;padding:16px;background:#ecfdf5;border-radius:10px;font-size:13px;line-height:1.8"><strong>什么时候才算恢复？</strong><br>必须有失败之后开始的真实业务请求或账号检测成功；模型相关故障需对应模型成功。仅更新 Cookie、重置状态、时间过去或没有新请求，都不算恢复。比例低于阈值且有实际恢复证据，告警才会自动结束。</div>`)
	}
	b.WriteString(`<p style="margin-top:24px;font-size:12px;color:#64748b">查看完整报表：Sub2 管理后台 → 运维监控 → Adobe 账号健康。<br>本报表只读取已有记录，不会额外检测账号、刷新 Cookie 或触发登录。</p></div></div></body></html>`)
	return b.String()
}

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
