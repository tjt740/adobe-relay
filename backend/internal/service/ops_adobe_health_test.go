//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/adobe"
	"github.com/stretchr/testify/require"
)

type healthAlertRepo struct {
	adobeAlertTestRepo
	report *AdobeHealthReport
	err    error
}

func (r *healthAlertRepo) GetAdobeHealthReport(context.Context, time.Time, time.Time, *int64) (*AdobeHealthReport, error) {
	return r.report, r.err
}

func TestAdobeHealthAlertRequiresSuccessForRecovery(t *testing.T) {
	now := time.Now().UTC()
	failure := now.Add(-time.Minute)
	r := &healthAlertRepo{report: &AdobeHealthReport{Total: 25, Pending: 5, RecentPending: 5, GeneratedAt: now}}
	for id := 1; id <= 5; id++ {
		r.report.Accounts = append(r.report.Accounts, AdobeHealthAccount{ID: int64(id), Pending: true, AlertPending: true, FailedAt: &failure, AlertFailedAt: &failure})
	}
	r.rule = &OpsAlertRule{ID: 1, Enabled: true, MetricType: OpsMetricAdobeErrorAccounts, Operator: ">=", Threshold: 5, WindowMinutes: 10, CooldownMinutes: 5, Filters: map[string]any{OpsAdobeAlertRatioFilter: 20}}
	settings := newNotificationEmailMemorySettingRepo()
	eval := NewOpsAlertEvaluatorService(&OpsService{settingRepo: settings}, r, nil, nil, nil, nil)
	evaluate := func() {
		if state := eval.ruleStates[1]; state != nil {
			state.LastEvaluatedAt = time.Now().Add(-3 * time.Minute)
		}
		eval.evaluateOnce(time.Minute)
	}
	evaluate()
	require.Equal(t, 1, r.created)
	require.NotNil(t, r.active)
	r.report.RecentPending = 0
	evaluate()
	require.NotNil(t, r.active, "quiet window is not recovery")
	r.report.Pending = 0
	r.report.Total = 100
	evaluate()
	require.NotNil(t, r.active, "population changes alone are not recovery")
	r.err = errors.New("db unavailable")
	evaluate()
	require.NotNil(t, r.active)
	r.err = nil
	success := time.Now().Add(time.Second)
	r.report.Accounts = append(r.report.Accounts, AdobeHealthAccount{ID: 99, FailedAt: &failure, AlertFailedAt: &failure, SuccessAt: &success})
	evaluate()
	require.NotNil(t, r.active, "unrelated account success is not recovery evidence for this incident")
	r.report.Pending = 5
	r.report.Total = 25
	r.report.Accounts[0].Pending = false
	r.report.Accounts[0].AlertPending = false
	r.report.Accounts[0].SuccessAt = &success
	evaluate()
	require.NotNil(t, r.active, "exactly 20 percent remains firing")
	r.report.Pending = 4
	evaluate()
	require.Nil(t, r.active, "below threshold with an incident member's success resolves")
}

func TestAdobeHealthReasonsNeverPersistUpstreamSecrets(t *testing.T) {
	secret := "cookie=private; Bearer token-secret; https://user:password@proxy"
	cases := []struct {
		err    error
		reason string
		code   int
	}{
		{adobe.NewAuthError(secret, 401), "auth", 401},
		{adobe.NewUpstreamTemporaryError(secret, 408, adobe.ErrorTypeStatus), "upstream", 408},
		{adobe.NewUpstreamTemporaryError(secret, 0, adobe.ErrorTypeProxy), "proxy", 0},
		{context.DeadlineExceeded, "timeout", 408},
		{&UpstreamFailoverError{StatusCode: 408}, "upstream", 408},
		{&OpenAIImagesUpstreamError{StatusCode: 408}, "upstream", 408},
		{errors.New("HTTP 408"), "upstream", 408},
		{errors.New(secret), "unknown", 0},
	}
	for _, tc := range cases {
		o := adobeHealthObservation(time.Now(), "okad", "model", tc.err)
		require.Equal(t, tc.reason, o.Reason)
		require.Equal(t, tc.code, o.Code)
		raw, _ := json.Marshal(o)
		require.NotContains(t, string(raw), secret)
	}
}

func TestAdobeHealthEmailReadableEscapedAndBounded(t *testing.T) {
	now := time.Now()
	report := &AdobeHealthReport{GeneratedAt: now, WindowStart: now.Add(-10 * time.Minute), Total: 108, Pending: 39, Historical: 39, Ratio: 36.11, Threshold: 20, Minimum: 5}
	for i := 0; i < 60; i++ {
		report.Accounts = append(report.Accounts, AdobeHealthAccount{ID: int64(i + 1), Name: "<script>unsafe</script>", Code: 408, Reason: "upstream", Source: "business", FailedAt: &now, Pending: true})
	}
	body := buildOpsAlertEmailBody(&OpsAlertRule{MetricType: OpsMetricAdobeErrorAccounts}, &OpsAlertEvent{Dimensions: map[string]any{"adobe_health": report}})
	require.Contains(t, body, "39 个账号仍有失败待验证")
	require.Contains(t, body, "36.11%")
	require.Contains(t, body, "20%")
	require.Contains(t, body, "先做什么")
	require.Contains(t, body, "另有 10 条")
	require.NotContains(t, body, "<script>")
	require.Contains(t, body, "&lt;script&gt;")
	require.NotContains(t, body, "Metric</b>")
	require.Equal(t, 50, strings.Count(body, "unsafe"))
	if path := os.Getenv("ADOBE_HEALTH_EMAIL_PREVIEW"); path != "" {
		require.NoError(t, os.WriteFile(path, []byte(body), 0600))
	}
}

func TestAdobeHealthLegacyIncidentKeepsRecoveredMembers(t *testing.T) {
	r := &AdobeHealthReport{Accounts: []AdobeHealthAccount{{ID: 2, AlertPending: true}}}
	e := &OpsAlertEvent{Description: "历史报告\n#1 account | 408=1\n#2 account | 401=1"}
	require.ElementsMatch(t, []int64{1, 2}, adobeInitialIncidentMembers(r, e))
}
