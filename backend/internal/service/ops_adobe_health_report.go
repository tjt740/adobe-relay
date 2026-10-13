package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type AdobeHealthAccount struct {
	AlertPending  bool       `json:"alert_pending"`
	AlertFailedAt *time.Time `json:"alert_failed_at,omitempty"`
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Scope         string     `json:"scope"`
	Code          int        `json:"code"`
	Reason        string     `json:"reason"`
	Stage         string     `json:"stage"`
	Source        string     `json:"source"`
	FailedAt      *time.Time `json:"failed_at,omitempty"`
	SuccessAt     *time.Time `json:"success_at,omitempty"`
	Pending       bool       `json:"pending"`
}

type AdobeHealthReport struct {
	GeneratedAt   time.Time            `json:"generated_at"`
	WindowStart   time.Time            `json:"window_start"`
	Total         int64                `json:"total"`
	Pending       int                  `json:"pending"`
	RecentPending int                  `json:"recent_pending"`
	Recovered     int                  `json:"recovered"`
	Unknown       int                  `json:"unknown"`
	Historical    int                  `json:"historical"`
	Ratio         float64              `json:"ratio"`
	Threshold     float64              `json:"threshold"`
	Minimum       int                  `json:"minimum"`
	Accounts      []AdobeHealthAccount `json:"accounts"`
}

type AdobeHealthReportRepository interface {
	GetAdobeHealthReport(context.Context, time.Time, time.Time, *int64) (*AdobeHealthReport, error)
}

func (r *AdobeHealthReport) Summary() string {
	return fmt.Sprintf("%d 个账号仍有 401/408 失败待验证，占全部 %d 个账号的 %.2f%%。最近窗口内 %d 个账号报过错；已有 %d 个账号取得后续成功记录。没有新请求不代表恢复。", r.Pending, r.Total, r.Ratio, r.Historical, r.Recovered)
}

func (r *AdobeHealthReport) HasRecoverySince(at time.Time) bool {
	for _, a := range r.Accounts {
		if !a.Pending && a.FailedAt != nil && a.SuccessAt != nil && a.SuccessAt.After(at) {
			return true
		}
	}
	return false
}

func adobeReportFromEvent(e *OpsAlertEvent) *AdobeHealthReport {
	if e == nil || e.Dimensions == nil {
		return nil
	}
	b, err := json.Marshal(e.Dimensions["adobe_health"])
	if err != nil {
		return nil
	}
	var r AdobeHealthReport
	if json.Unmarshal(b, &r) != nil || r.GeneratedAt.IsZero() {
		return nil
	}
	return &r
}

func (s *OpsService) AdobeHealthReport(ctx context.Context) (*AdobeHealthReport, error) {
	repo, ok := s.opsRepo.(AdobeHealthReportRepository)
	if !ok {
		return nil, fmt.Errorf("Adobe health report unavailable")
	}
	rules, err := s.opsRepo.ListAlertRules(ctx)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if !rule.Enabled || rule.MetricType != OpsMetricAdobeErrorAccounts {
			continue
		}
		ratio, enabled, err := OpsAdobeAlertRatioThreshold(rule.Filters)
		if err != nil {
			return nil, err
		}
		if !enabled {
			continue
		}
		_, groupID, _ := parseOpsAlertRuleScope(rule.Filters)
		now := time.Now().UTC()
		r, err := repo.GetAdobeHealthReport(ctx, now.Add(-time.Duration(rule.WindowMinutes)*time.Minute), now, groupID)
		if err != nil {
			return nil, err
		}
		r.Threshold = ratio
		r.Minimum = int(rule.Threshold)
		return r, nil
	}
	return nil, fmt.Errorf("no enabled Adobe ratio rule")
}

func adobeIncidentMembers(r *AdobeHealthReport) []int64 {
	ids := []int64{}
	seen := map[int64]bool{}
	for _, a := range r.Accounts {
		if a.AlertPending && !seen[a.ID] {
			ids = append(ids, a.ID)
			seen[a.ID] = true
		}
	}
	return ids
}
func adobeIncidentRecovered(r *AdobeHealthReport, e *OpsAlertEvent) bool {
	raw, err := json.Marshal(e.Dimensions["adobe_incident_accounts"])
	if err != nil {
		return false
	}
	var ids []int64
	if json.Unmarshal(raw, &ids) != nil {
		return false
	}
	members := map[int64]bool{}
	for _, id := range ids {
		members[id] = true
	}
	for _, a := range r.Accounts {
		if members[a.ID] && !a.AlertPending && a.AlertFailedAt != nil && a.SuccessAt != nil && a.SuccessAt.After(e.FiredAt) {
			return true
		}
	}
	return false
}
