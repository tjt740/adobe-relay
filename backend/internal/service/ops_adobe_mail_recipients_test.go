//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdobeMailRecipientSelectionPreservesPrimary(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	primary := OpsAdobeAlertMail{Recipient: "first@example.test", SMTP: SMTPConfig{Host: "primary", Port: 465, Password: "first-secret"}}
	second := OpsAdobeAlertMail{Recipient: "second@example.test", SMTP: SMTPConfig{Host: "secondary", Port: 465, Password: "second-secret"}}
	save := func(extra []OpsAdobeAlertMail) {
		raw, err := json.Marshal(struct {
			OpsAdobeAlertMail
			Additional []OpsAdobeAlertMail `json:"additional"`
		}{primary, extra})
		require.NoError(t, err)
		require.NoError(t, settings.Set(ctx, SettingKeyOpsAdobeAlertMail, string(raw)))
	}
	save([]OpsAdobeAlertMail{second})
	service := NewEmailService(settings, nil)
	got, err := service.getAdobeAlertMail(ctx)
	require.NoError(t, err)
	require.Equal(t, primary, *got)
	got, err = service.getAdobeAlertMailForRecipient(ctx, " SECOND@example.test ")
	require.NoError(t, err)
	require.Equal(t, second, *got)
	_, err = service.getAdobeAlertMailForRecipient(ctx, "other@example.test")
	require.Error(t, err)
	save([]OpsAdobeAlertMail{second, second})
	_, err = service.getAdobeAlertMailForRecipient(ctx, second.Recipient)
	require.ErrorContains(t, err, "ambiguous")
	got, err = service.getAdobeAlertMailForRecipient(ctx, primary.Recipient)
	require.NoError(t, err)
	require.Equal(t, primary, *got)
	second.SMTP.Host = ""
	save([]OpsAdobeAlertMail{second})
	_, err = service.getAdobeAlertMailForRecipient(ctx, second.Recipient)
	require.Error(t, err)
	got, err = service.getAdobeAlertMail(ctx)
	require.NoError(t, err)
	require.Equal(t, primary, *got)
	_, err = service.GetSMTPConfig(ctx)
	require.ErrorIs(t, err, ErrEmailNotConfigured)
}

func TestAdobeQuietNotificationUsesOnlyRecipientTransport(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{"alert":{"enabled":true,"recipients":[],"rate_limit_per_hour":10}}`))
	firstSMTP := startNotificationEmailTestSMTPServer(t)
	secondSMTP := startNotificationEmailTestSMTPServer(t)
	transport := func(server *notificationEmailTestSMTPServer, recipient string) OpsAdobeAlertMail {
		repo := newNotificationEmailMemorySettingRepo()
		require.NoError(t, repo.SetMultiple(ctx, server.settings()))
		config, err := NewEmailService(repo, nil).GetSMTPConfig(ctx)
		require.NoError(t, err)
		return OpsAdobeAlertMail{Recipient: recipient, SMTP: *config}
	}
	primary := transport(firstSMTP, "first@example.test")
	second := transport(secondSMTP, "second@example.test")
	raw, err := json.Marshal(struct {
		OpsAdobeAlertMail
		Additional []OpsAdobeAlertMail `json:"additional"`
	}{primary, []OpsAdobeAlertMail{second}})
	require.NoError(t, err)
	require.NoError(t, settings.Set(ctx, SettingKeyOpsAdobeAlertMail, string(raw)))
	for i, recipient := range []string{primary.Recipient, second.Recipient} {
		rule := &OpsAlertRule{ID: int64(9 + i), MetricType: OpsMetricAdobeErrorAccounts, NotifyEmail: true, Filters: map[string]any{OpsAdobeAlertRatioFilter: 20, "email_recipient": recipient}}
		report := &AdobeHealthReport{GeneratedAt: time.Now(), Total: 25, RecentPending: 5, Minimum: 5, Threshold: 20}
		event := &OpsAlertEvent{ID: int64(101 + i), RuleID: rule.ID, Status: OpsAlertStatusFiring, Dimensions: map[string]any{"adobe_health": report}}
		repo := &adobeReceiptTestRepo{}
		evaluator := func() *OpsAlertEvaluatorService {
			return NewOpsAlertEvaluatorService(&OpsService{settingRepo: settings}, repo, NewEmailService(settings, nil), nil, nil, nil)
		}
		require.True(t, evaluator().maybeSendAlertEmail(ctx, nil, rule, event))
		require.False(t, evaluator().maybeSendAlertEmail(ctx, nil, rule, event), "restart must not repeat this recipient's event")
		require.True(t, repo.finished)
		require.Equal(t, 1, repo.claims)
		if i == 0 {
			require.EqualValues(t, 0, secondSMTP.messageCount(), "adding a recipient must not broadcast the original rule")
		}
	}
	require.EqualValues(t, 1, firstSMTP.messageCount())
	require.EqualValues(t, 1, secondSMTP.messageCount())
	require.Contains(t, strings.SplitN(firstSMTP.lastMessage(), "\r\n\r\n", 2)[0], "To: <first@example.test>")
	require.Contains(t, strings.SplitN(secondSMTP.lastMessage(), "\r\n\r\n", 2)[0], "To: <second@example.test>")
	require.NotContains(t, firstSMTP.lastMessage(), "second@example.test")
	require.NotContains(t, secondSMTP.lastMessage(), "first@example.test")
}
