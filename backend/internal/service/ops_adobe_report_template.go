package service

import (
	"bytes"
	_ "embed"
	"html/template"
	"time"
)

//go:embed ops_adobe_report_email.html
var adobeReportEmailTemplate string

type adobeReportEmailView struct {
	Report                                          *AdobeHealthReport
	Heading                                         string
	Legacy                                          string
	Accounts401, Accounts408                        int
	Errors401, Errors408, Business, Tests, Timeouts int64
	Auth, Quota, Entitlement, Unknown               int64
	TriggerRatio                                    float64
}

var adobeReportTemplate = template.Must(template.New("adobe").Funcs(template.FuncMap{
	"stamp": func(t time.Time) string { return t.In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02 15:04:05") },
	"at": func(t *time.Time) string {
		if t == nil {
			return "未记录"
		}
		return t.In(time.FixedZone("UTC+8", 8*3600)).Format("01-02 15:04:05")
	},
	"reason": func(v string) string { label, _ := AdobeHealthReason(v); return label },
	"stage":  adobeHealthStageLabel,
	"source": adobeHealthSourceLabel,
}).Parse(adobeReportEmailTemplate))

func buildAdobeHealthEmail(rule *OpsAlertRule, event *OpsAlertEvent) string {
	v := adobeReportEmailView{Report: adobeReportFromEvent(event), Heading: "账号异常报表"}
	if event.Status == "report" {
		v.Heading = "账号运行报表"
	}
	if event.Status == OpsAlertStatusResolved {
		v.Heading = "账号异常回落报表"
	}
	if v.Report == nil {
		v.Legacy = event.Description
	} else {
		if v.Report.RecoveryThreshold == 0 {
			v.Report.RecoveryThreshold = v.Report.Threshold
		}
		if v.Report.Total > 0 {
			v.TriggerRatio = float64(v.Report.RecentPending) * 100 / float64(v.Report.Total)
		}
		for _, a := range v.Report.WindowAccounts {
			if a.Count401 > 0 {
				v.Accounts401++
			}
			if a.Count408 > 0 {
				v.Accounts408++
			}
			v.Errors401 += a.Count401
			v.Errors408 += a.Count408
			v.Business += a.BusinessErrors
			v.Tests += a.TestErrors
			v.Timeouts += a.TestTimeouts
			v.Auth += a.Auth401
			v.Quota += a.Quota401
			v.Entitlement += a.Entitled401
			v.Unknown += a.Unknown401
		}
	}
	var b bytes.Buffer
	if err := adobeReportTemplate.Execute(&b, v); err != nil {
		return "账号报表生成失败，请查看后台记录。"
	}
	return b.String()
}
