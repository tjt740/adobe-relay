package service

import (
	"context"
	"time"
)

const OpsAdobeQuietNotificationsFilter = "adobe_confirmed_notifications"

func adobeQuietNotifications(rule *OpsAlertRule) bool {
	if rule == nil || rule.MetricType != OpsMetricAdobeErrorAccounts {
		return false
	}
	if value, exists := rule.Filters[OpsAdobeQuietNotificationsFilter]; exists {
		enabled, _ := value.(bool)
		return enabled
	}
	// Existing Adobe percentage rules become quiet immediately on upgrade,
	// before the persisted rule settings are updated by deployment verification.
	_, configured := rule.Filters[OpsAdobeAlertRatioFilter]
	return configured
}

func configureAdobeHealthReport(r *AdobeHealthReport, rule *OpsAlertRule, threshold float64) {
	r.Threshold, r.Minimum = threshold, int(rule.Threshold)
	r.RecoveryThreshold = threshold
	r.ConfirmationMinutes = rule.SustainedMinutes
	r.NotificationCooldownMinutes = rule.CooldownMinutes
	r.QuietNotifications = adobeQuietNotifications(rule)
	if r.QuietNotifications {
		r.RecoveryThreshold = min(10, threshold/2)
		r.ConfirmationMinutes = max(4, rule.SustainedMinutes)
		r.NotificationCooldownMinutes = max(60, rule.CooldownMinutes)
	}
}

// Receipts are persisted before contacting SMTP. A process crash with an
// uncertain delivery outcome stays reserved for operator inspection; it must
// not create another email on restart. Explicit failures retry after cooldown.
type AdobeAlertDeliveryRepository interface {
	ClaimAdobeAlertEmail(context.Context, int64, int64, time.Time, time.Duration) (bool, error)
	FinishAdobeAlertEmail(context.Context, int64, bool) error
}

func (s *OpsAlertEvaluatorService) adobeConfirmationReady(ruleID int64, now time.Time, minutes int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ruleStates[ruleID]
	return state != nil && state.ConsecutiveBreaches >= 3 && !state.FirstBreachedAt.IsZero() && now.Add(time.Second).Sub(state.FirstBreachedAt) >= time.Duration(max(4, minutes))*time.Minute
}
