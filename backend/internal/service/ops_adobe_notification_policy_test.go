//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdobeConfirmedNotificationsHoldAndRecovery(t *testing.T) {
	now := time.Now().UTC()
	failure := now.Add(-time.Minute)
	r := &healthAlertRepo{report: &AdobeHealthReport{Total: 25, Pending: 5, RecentPending: 5, GeneratedAt: now}}
	for id := 1; id <= 5; id++ {
		r.report.Accounts = append(r.report.Accounts, AdobeHealthAccount{ID: int64(id), Pending: true, AlertPending: true, FailedAt: &failure, AlertFailedAt: &failure})
	}
	r.rule = &OpsAlertRule{ID: 1, Enabled: true, MetricType: OpsMetricAdobeErrorAccounts, Operator: ">=", Threshold: 5, WindowMinutes: 10, SustainedMinutes: 4, CooldownMinutes: 60, Filters: map[string]any{OpsAdobeAlertRatioFilter: 20, OpsAdobeQuietNotificationsFilter: true}}
	eval := NewOpsAlertEvaluatorService(&OpsService{settingRepo: newNotificationEmailMemorySettingRepo()}, r, nil, nil, nil, nil)
	tick := func() {
		if state := eval.ruleStates[1]; state != nil {
			state.LastEvaluatedAt = time.Now().Add(-2 * time.Minute)
		}
		eval.evaluateOnce(time.Minute)
	}
	tick()
	require.Zero(t, r.created)
	tick()
	require.Zero(t, r.created, "multiple calls do not simulate four elapsed minutes")
	eval.ruleStates[1].FirstBreachedAt = time.Now().Add(-4 * time.Minute)
	tick()
	require.Equal(t, 1, r.created)
	r.report.RecentPending = 0
	tick()
	require.NotNil(t, r.active, "quiet window cannot recover an incident")
	r.report.Pending = 2
	tick()
	require.NotNil(t, r.active, "below ten percent still requires successful evidence")
	r.err = errors.New("db unavailable")
	tick()
	require.NotNil(t, r.active)
	r.err = nil
	success := time.Now().Add(time.Second)
	r.report.Accounts[0].Pending = false
	r.report.Accounts[0].AlertPending = false
	r.report.Accounts[0].SuccessAt = &success
	r.report.Total = 20
	r.report.Pending = 2
	tick()
	require.NotNil(t, r.active, "exactly ten percent remains open")
	r.report.Total = 25
	tick()
	require.Nil(t, r.active)
	r.report.Pending = 5
	r.report.RecentPending = 5
	tick()
	eval.ruleStates[1].FirstBreachedAt = time.Now().Add(-4 * time.Minute)
	tick()
	tick()
	require.Equal(t, 1, r.created, "new event cooldown prevents rapid repeat notification")
	r.latest.FiredAt = time.Now().Add(-61 * time.Minute)
	tick()
	require.Equal(t, 2, r.created)
}

func TestAdobeConfirmationResetsOnRecoveryFaultGapAndRestart(t *testing.T) {
	s := NewOpsAlertEvaluatorService(nil, nil, nil, nil, nil, nil)
	now := time.Now()
	s.updateRuleBreaches(1, now, 2*time.Minute, true)
	s.updateRuleBreaches(1, now.Add(2*time.Minute), 2*time.Minute, true)
	require.False(t, s.adobeConfirmationReady(1, now.Add(2*time.Minute), 4))
	s.updateRuleBreaches(1, now.Add(4*time.Minute), 2*time.Minute, true)
	require.True(t, s.adobeConfirmationReady(1, now.Add(4*time.Minute), 4))
	s.updateRuleBreaches(1, now.Add(6*time.Minute), 2*time.Minute, false)
	require.False(t, s.adobeConfirmationReady(1, now.Add(6*time.Minute), 4))
	s.updateRuleBreaches(1, now.Add(8*time.Minute), 2*time.Minute, true)
	s.resetRuleState(1, now.Add(10*time.Minute))
	require.False(t, s.adobeConfirmationReady(1, now.Add(10*time.Minute), 4))
	s.updateRuleBreaches(1, now.Add(12*time.Minute), 2*time.Minute, true)
	s.updateRuleBreaches(1, now.Add(20*time.Minute), 2*time.Minute, true)
	require.False(t, s.adobeConfirmationReady(1, now.Add(20*time.Minute), 4))
	restarted := NewOpsAlertEvaluatorService(nil, nil, nil, nil, nil, nil)
	require.False(t, restarted.adobeConfirmationReady(1, now.Add(time.Hour), 4))
}

func TestAdobeQuietNotificationsRejectManualReport(t *testing.T) {
	settings := newNotificationEmailMemorySettingRepo()
	r := &adobeAlertTestRepo{rule: &OpsAlertRule{ID: 9, Enabled: true, MetricType: OpsMetricAdobeErrorAccounts, Filters: map[string]any{OpsAdobeQuietNotificationsFilter: true}}}
	// Guard precedes report generation and SMTP, including healthy reports.
	s := &OpsService{settingRepo: settings, opsRepo: r, adobeReportEmail: NewEmailService(settings, nil)}
	require.NoError(t, settings.Set(context.Background(), SettingKeyOpsAdobeAlertMail, `{"recipient":"owner@example.test","smtp":{"host":"localhost","port":25}}`))
	err := s.SendAdobeHealthReport(context.Background())
	require.ErrorContains(t, err, "同一事件不重复发送")
}

type adobeReceiptTestRepo struct {
	adobeAlertTestRepo
	claimed, finished bool
	claims            int
}

func (r *adobeReceiptTestRepo) ClaimAdobeAlertEmail(context.Context, int64, int64, time.Time, time.Duration) (bool, error) {
	if r.claimed {
		return false, nil
	}
	r.claimed = true
	r.claims++
	return true, nil
}
func (r *adobeReceiptTestRepo) FinishAdobeAlertEmail(_ context.Context, _ int64, sent bool) error {
	r.finished = sent
	return nil
}

func TestAdobeQuietNotificationsOnlySendsFreshProblemOnce(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{"alert":{"enabled":true,"recipients":[],"rate_limit_per_hour":10}}`))
	smtp := startNotificationEmailTestSMTPServer(t)
	privateSettings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, privateSettings.SetMultiple(ctx, smtp.settings()))
	config, err := NewEmailService(privateSettings, nil).GetSMTPConfig(ctx)
	require.NoError(t, err)
	private, err := json.Marshal(OpsAdobeAlertMail{Recipient: "owner@example.test", SMTP: *config})
	require.NoError(t, err)
	require.NoError(t, settings.Set(ctx, SettingKeyOpsAdobeAlertMail, string(private)))
	rule := &OpsAlertRule{ID: 9, MetricType: OpsMetricAdobeErrorAccounts, NotifyEmail: true, Filters: map[string]any{OpsAdobeAlertRatioFilter: 20, "email_recipient": "owner@example.test"}}
	require.True(t, adobeQuietNotifications(rule), "quiet protection applies immediately during upgrade")
	report := &AdobeHealthReport{GeneratedAt: time.Now(), Total: 25, RecentPending: 4, Minimum: 5, Threshold: 20}
	event := &OpsAlertEvent{ID: 1, RuleID: 9, Status: OpsAlertStatusFiring, Dimensions: map[string]any{"adobe_health": report}}
	repo := &adobeReceiptTestRepo{}
	newEval := func() *OpsAlertEvaluatorService {
		return NewOpsAlertEvaluatorService(&OpsService{settingRepo: settings}, repo, NewEmailService(settings, nil), nil, nil, nil)
	}
	eval := newEval()
	require.False(t, eval.maybeSendAlertEmail(ctx, nil, rule, event))
	require.Zero(t, repo.claims)
	report.RecentPending = 5
	report.GeneratedAt = time.Now().Add(-6 * time.Minute)
	require.False(t, eval.maybeSendAlertEmail(ctx, nil, rule, event))
	require.Zero(t, repo.claims)
	report.GeneratedAt = time.Now()
	event.Status = OpsAlertStatusResolved
	require.False(t, eval.maybeSendAlertEmail(ctx, nil, rule, event))
	require.Zero(t, repo.claims)
	event.Status = OpsAlertStatusFiring
	require.True(t, eval.maybeSendAlertEmail(ctx, nil, rule, event))
	require.True(t, repo.finished)
	require.False(t, newEval().maybeSendAlertEmail(ctx, nil, rule, event), "receipt prevents duplicate even when event object still says unsent")
	require.EqualValues(t, 1, smtp.messageCount())
	require.Equal(t, 1, repo.claims)
}
