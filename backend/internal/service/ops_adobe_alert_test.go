//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type adobeAlertTestRepo struct {
	OpsRepository
	rule           *OpsAlertRule
	active, latest *OpsAlertEvent
	accounts       []OpsAdobeAccountErrors
	created        int
	queries        int
}

func (r *adobeAlertTestRepo) GetAdobeErrorAccounts(context.Context, time.Time, time.Time, *int64) ([]OpsAdobeAccountErrors, error) {
	r.queries++
	return r.accounts, nil
}
func (r *adobeAlertTestRepo) ListAlertRules(context.Context) ([]*OpsAlertRule, error) {
	return []*OpsAlertRule{r.rule}, nil
}
func (r *adobeAlertTestRepo) GetLatestSystemMetrics(context.Context, int) (*OpsSystemMetricsSnapshot, error) {
	return nil, nil
}
func (r *adobeAlertTestRepo) GetActiveAlertEvent(context.Context, int64) (*OpsAlertEvent, error) {
	return r.active, nil
}
func (r *adobeAlertTestRepo) GetLatestAlertEvent(context.Context, int64) (*OpsAlertEvent, error) {
	return r.latest, nil
}
func (r *adobeAlertTestRepo) CreateAlertEvent(_ context.Context, e *OpsAlertEvent) (*OpsAlertEvent, error) {
	r.created++
	e.ID = int64(r.created)
	r.active = e
	r.latest = e
	return e, nil
}
func (r *adobeAlertTestRepo) UpdateAlertEventEmailSent(_ context.Context, _ int64, sent bool) error {
	r.active.EmailSent = sent
	return nil
}
func (r *adobeAlertTestRepo) UpdateAlertEventStatus(_ context.Context, _ int64, status string, at *time.Time) error {
	r.active.Status = status
	r.active.ResolvedAt = at
	r.active = nil
	return nil
}
func (r *adobeAlertTestRepo) UpsertJobHeartbeat(context.Context, *OpsUpsertJobHeartbeatInput) error {
	return nil
}

func TestAdobeAlertEmailThresholdCooldownRestartAndFailedSend(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{"alert":{"enabled":true,"recipients":[],"rate_limit_per_hour":10}}`))
	// Start without SMTP to prove delivery can recover on a later evaluation.
	r := &adobeAlertTestRepo{rule: &OpsAlertRule{ID: 1, Name: "Adobe 401/408", Enabled: true, Severity: "P1", MetricType: OpsMetricAdobeErrorAccounts, Operator: ">=", Threshold: 5, WindowMinutes: 10, CooldownMinutes: 30, NotifyEmail: true, Filters: map[string]any{"email_recipient": "owner@example.test"}}}
	ops := &OpsService{settingRepo: settings}
	email := NewEmailService(settings, nil)
	newEvaluator := func() *OpsAlertEvaluatorService { return NewOpsAlertEvaluatorService(ops, r, email, nil, nil, nil) }
	eval := newEvaluator()
	evaluate := func() {
		if state := eval.ruleStates[r.rule.ID]; state != nil {
			state.LastEvaluatedAt = time.Now().Add(-2 * time.Minute)
		}
		eval.evaluateOnce(time.Minute)
	}
	for i := 1; i <= 4; i++ {
		r.accounts = append(r.accounts, OpsAdobeAccountErrors{AccountID: int64(i), Name: "fixture", Count408: 7, Status: "active", LastSeen: time.Now()})
	}
	evaluate()
	require.Zero(t, r.created, "28 errors from 4 accounts do not meet the 5-account threshold")
	eval.evaluateOnce(time.Minute)
	require.Equal(t, 1, r.queries, "global ticks must not query this metric more frequently than every two minutes")
	eval.ruleStates[r.rule.ID].LastEvaluatedAt = time.Now().Add(-119500 * time.Millisecond)
	eval.evaluateOnce(time.Minute)
	require.Equal(t, 2, r.queries, "scheduler jitter must not defer a two-minute evaluation for another minute")
	r.accounts = append(r.accounts, OpsAdobeAccountErrors{AccountID: 5, Name: "<script>alert(1)</script>", Count401: 2, Auth401: 1, Quota401: 1, Count408: 1, Status: "error", LastSeen: time.Now()})
	evaluate()
	require.Equal(t, 1, r.created)
	require.False(t, r.active.EmailSent)
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(ctx, smtp.settings()))
	evaluate()
	require.True(t, r.active.EmailSent)
	require.EqualValues(t, 1, smtp.messageCount())
	require.Contains(t, r.active.Description, "受影响账号：5 个")
	require.Contains(t, r.active.Description, "401：1 个账号，2 条")
	require.Contains(t, r.active.Description, "408：5 个账号，29 条")
	newEvaluator().evaluateOnce(2 * time.Minute)
	require.EqualValues(t, 1, smtp.messageCount(), "restart must not resend an active delivered incident")
	r.accounts = nil
	evaluate()
	require.Nil(t, r.active)
	for i := 0; i < 5; i++ {
		r.accounts = append(r.accounts, OpsAdobeAccountErrors{AccountID: int64(i + 1), Count408: 1})
	}
	evaluate()
	require.Equal(t, 1, r.created, "a new incident inside the cooldown remains quiet")
	r.latest.FiredAt = time.Now().Add(-31 * time.Minute)
	evaluate()
	require.Equal(t, 2, r.created)
	require.EqualValues(t, 2, smtp.messageCount())
}

func TestAdobeAlertDescriptionBoundedAndEscaped(t *testing.T) {
	var rows []OpsAdobeAccountErrors
	for i := 0; i < 120; i++ {
		rows = append(rows, OpsAdobeAccountErrors{AccountID: int64(i + 1), Name: "<tag>\n" + strings.Repeat("x", 200), Count401: 1, LastSeen: time.Now()})
	}
	desc := buildAdobeErrorAlertDescription(rows, time.Now().Add(-10*time.Minute), time.Now())
	require.Contains(t, desc, "受影响账号：120 个")
	require.Contains(t, desc, "其余 20 个账号")
	body := buildOpsAlertEmailBody(&OpsAlertRule{}, &OpsAlertEvent{Description: desc})
	require.NotContains(t, body, "<tag>")
	require.Contains(t, body, "&lt;tag&gt;")
	require.Contains(t, body, "<br>")
}
