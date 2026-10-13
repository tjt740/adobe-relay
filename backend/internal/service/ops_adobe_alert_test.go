//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
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
	total          int64
	populationErr  error
}

func (r *adobeAlertTestRepo) GetAdobeErrorAccountPopulation(context.Context, time.Time, time.Time, *int64) ([]OpsAdobeAccountErrors, int64, error) {
	r.queries++
	return r.accounts, r.total, r.populationErr
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
	r := &adobeAlertTestRepo{rule: &OpsAlertRule{ID: 1, Name: "Adobe 401/408", Enabled: true, Severity: "P1", MetricType: OpsMetricAdobeErrorAccounts, Operator: ">=", Threshold: 5, WindowMinutes: 10, CooldownMinutes: 5, NotifyEmail: true, Filters: map[string]any{"email_recipient": "owner@example.test"}}}
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
		r.accounts = append(r.accounts, OpsAdobeAccountErrors{AccountID: int64(i), Name: "fixture", Count408: 7, TestErrors: 7, TestTimeouts: 2, Status: "active", LastSeen: time.Now()})
	}
	evaluate()
	require.Zero(t, r.created, "28 errors from 4 accounts do not meet the 5-account threshold")
	eval.evaluateOnce(time.Minute)
	require.Equal(t, 1, r.queries, "global ticks must not query this metric more frequently than every two minutes")
	eval.ruleStates[r.rule.ID].LastEvaluatedAt = time.Now().Add(-119500 * time.Millisecond)
	eval.evaluateOnce(time.Minute)
	require.Equal(t, 2, r.queries, "scheduler jitter must not defer a two-minute evaluation for another minute")
	r.accounts = append(r.accounts, OpsAdobeAccountErrors{AccountID: 5, Name: "<script>alert(1)</script>", Count401: 2, Auth401: 1, Quota401: 1, Count408: 1, BusinessErrors: 3, Status: "error", LastSeen: time.Now()})
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
	require.Contains(t, r.active.Description, "业务请求 3 条；账号检测（含批量）28 条，其中检测超时 8 条")
	newEvaluator().evaluateOnce(2 * time.Minute)
	require.EqualValues(t, 1, smtp.messageCount(), "restart must not resend an active delivered incident")
	r.accounts = nil
	evaluate()
	require.Nil(t, r.active)
	for i := 0; i < 5; i++ {
		r.accounts = append(r.accounts, OpsAdobeAccountErrors{AccountID: int64(i + 1), Count408: 1})
	}
	r.latest.FiredAt = time.Now().Add(-4 * time.Minute)
	evaluate()
	require.Equal(t, 1, r.created, "a new incident inside the cooldown remains quiet")
	r.latest.FiredAt = time.Now().Add(-5*time.Minute - time.Second)
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

func TestAdobeAlertDedicatedTransportDoesNotEnableOtherMail(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{"alert":{"enabled":true,"recipients":[]}}`))
	smtp := startNotificationEmailTestSMTPServer(t)
	transportSettings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, transportSettings.SetMultiple(ctx, smtp.settings()))
	transport, err := NewEmailService(transportSettings, nil).GetSMTPConfig(ctx)
	require.NoError(t, err)
	private, err := json.Marshal(OpsAdobeAlertMail{Recipient: "owner@example.test", SMTP: *transport})
	require.NoError(t, err)
	require.NoError(t, settings.Set(ctx, SettingKeyOpsAdobeAlertMail, string(private)))
	email := NewEmailService(settings, nil)
	rule := &OpsAlertRule{ID: 1, MetricType: OpsMetricAdobeErrorAccounts, NotifyEmail: true, Filters: map[string]any{"email_recipient": "owner@example.test"}}
	event := &OpsAlertEvent{ID: 1}
	r := &adobeAlertTestRepo{active: event}
	eval := NewOpsAlertEvaluatorService(&OpsService{settingRepo: settings}, r, email, nil, nil, nil)
	require.True(t, eval.maybeSendAlertEmail(ctx, nil, rule, event))
	require.EqualValues(t, 1, smtp.messageCount())
	_, err = email.GetSMTPConfig(ctx)
	require.ErrorIs(t, err, ErrEmailNotConfigured, "global mail settings must remain unchanged")
	event.EmailSent = false
	rule.MetricType = "error_rate"
	require.False(t, eval.maybeSendAlertEmail(ctx, nil, rule, event), "other alerts must not use this transport")
	rule.MetricType = OpsMetricAdobeErrorAccounts
	rule.Filters["email_recipient"] = "someone-else@example.test"
	require.False(t, eval.maybeSendAlertEmail(ctx, nil, rule, event), "dedicated transport must not send to another recipient")
	require.EqualValues(t, 1, smtp.messageCount())
}

func adobeAlertFixtureRows(n int) []OpsAdobeAccountErrors {
	rows := make([]OpsAdobeAccountErrors, n)
	for i := range rows {
		rows[i] = OpsAdobeAccountErrors{AccountID: int64(i + 1), Count408: 1, TestErrors: 1}
	}
	return rows
}

func TestAdobeAlertRatioRecoveryDoesNotReopenBelowTwentyPercent(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{"alert":{"enabled":true,"recipients":[]}}`))
	smtp := startNotificationEmailTestSMTPServer(t)
	r := &adobeAlertTestRepo{total: 204, rule: &OpsAlertRule{ID: 1, Name: "Adobe", Enabled: true, MetricType: OpsMetricAdobeErrorAccounts, Operator: ">=", Threshold: 5, WindowMinutes: 10, CooldownMinutes: 5, NotifyEmail: true, Filters: map[string]any{OpsAdobeQuietNotificationsFilter: false, OpsAdobeAlertRatioFilter: float64(20), "email_recipient": "owner@example.test"}}}
	newEvaluator := func() *OpsAlertEvaluatorService {
		return NewOpsAlertEvaluatorService(&OpsService{settingRepo: settings}, r, NewEmailService(settings, nil), nil, nil, nil)
	}
	eval := newEvaluator()
	evaluate := func(n int) {
		r.accounts = adobeAlertFixtureRows(n)
		if state := eval.ruleStates[1]; state != nil {
			state.LastEvaluatedAt = time.Now().Add(-2 * time.Minute)
		}
		eval.evaluateOnce(time.Minute)
	}
	for _, n := range []int{0, 4, 5, 10, 40, 40} {
		evaluate(n)
		require.Zero(t, r.created)
	}
	evaluate(41)
	require.Equal(t, 1, r.created)
	require.False(t, r.active.EmailSent, "delivery failure keeps the incident pending")
	require.NoError(t, settings.SetMultiple(ctx, smtp.settings()))
	evaluate(41)
	require.Equal(t, 1, r.created, "retry uses the same incident")
	require.True(t, r.active.EmailSent)
	require.EqualValues(t, 1, smtp.messageCount())
	require.Contains(t, r.active.Description, "41 / 204 = 20.10%")
	require.Contains(t, r.active.Description, "包含暂停和异常账号")
	evaluate(60)
	evaluate(41)
	require.EqualValues(t, 1, smtp.messageCount(), "ongoing incidents stay quiet")
	// Failed population queries must not falsely resolve a real incident.
	r.populationErr = errors.New("database unavailable")
	evaluate(0)
	require.NotNil(t, r.active)
	r.populationErr = nil
	evaluate(40)
	require.Nil(t, r.active)
	r.latest.FiredAt = time.Now().Add(-6 * time.Minute)
	for i := 0; i < 3; i++ {
		evaluate(40)
		require.Equal(t, 1, r.created)
	}
	eval = newEvaluator()
	evaluate(40)
	require.Equal(t, 1, r.created, "restart must not turn 40/204 into a new incident")
	r.latest.FiredAt = time.Now().Add(-4 * time.Minute)
	evaluate(41)
	require.Equal(t, 1, r.created, "new crossing respects the five-minute cooldown")
	r.latest.FiredAt = time.Now().Add(-5*time.Minute - time.Second)
	evaluate(41)
	require.Equal(t, 2, r.created)
	require.EqualValues(t, 2, smtp.messageCount())
	// The floor gates new events, not recovery: 4/10 is still above 20%.
	r.total = 10
	evaluate(4)
	require.NotNil(t, r.active)
	evaluate(2)
	require.NotNil(t, r.active, "exactly 20% is not recovery")
	evaluate(1)
	require.Nil(t, r.active)
	r.latest.FiredAt = time.Now().Add(-6 * time.Minute)
	evaluate(4)
	require.Equal(t, 2, r.created, "at least five accounts are required to open an incident")
	evaluate(5)
	require.Equal(t, 3, r.created)
	r.total = 0
	evaluate(0)
	require.Nil(t, r.active, "empty population recovers without division by zero")
}

func TestAdobeAlertRatioStrictBoundaryAndInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		total    int64
		affected int
		breached bool
	}{
		{100, 19, false}, {100, 20, true}, {204, 40, false}, {204, 41, true}, {206, 41, false}, {206, 42, true}, {0, 0, false},
	} {
		r := &adobeAlertTestRepo{total: tc.total, accounts: adobeAlertFixtureRows(tc.affected)}
		s := NewOpsAlertEvaluatorService(nil, r, nil, nil, nil, nil)
		count, _, enabled, breached, err := s.adobeErrorMetric(context.Background(), time.Now().Add(-10*time.Minute), time.Now(), nil, map[string]any{OpsAdobeQuietNotificationsFilter: false, OpsAdobeAlertRatioFilter: 20})
		require.NoError(t, err)
		require.True(t, enabled)
		require.EqualValues(t, tc.affected, count)
		require.Equal(t, tc.breached, breached)
	}
	for _, v := range []any{nil, "20", true, float64(0), float64(-1), float64(101)} {
		_, _, err := OpsAdobeAlertRatioThreshold(map[string]any{OpsAdobeQuietNotificationsFilter: false, OpsAdobeAlertRatioFilter: v})
		require.Error(t, err)
	}
	r := &adobeAlertTestRepo{total: 0, accounts: adobeAlertFixtureRows(1)}
	_, _, _, _, err := NewOpsAlertEvaluatorService(nil, r, nil, nil, nil, nil).adobeErrorMetric(context.Background(), time.Now().Add(-time.Minute), time.Now(), nil, map[string]any{OpsAdobeQuietNotificationsFilter: false, OpsAdobeAlertRatioFilter: 20})
	require.Error(t, err, "inconsistent snapshots must not generate false recovery")
}
