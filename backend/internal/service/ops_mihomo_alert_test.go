//go:build unit

package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMihomoAlertsFireRecoverAndNeverEmailWhenSiteOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.json")
	t.Setenv("MIHOMO_RUNTIME_SNAPSHOT", path)
	settings := newNotificationEmailMemorySettingRepo()
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(context.Background(), smtp.settings()))
	require.NoError(t, settings.Set(context.Background(), SettingKeyOpsEmailNotificationConfig, `{"alert":{"enabled":true,"recipients":["owner@example.test"],"rate_limit_per_hour":10}}`))
	for _, metric := range []string{OpsMetricMihomoMemory, OpsMetricMihomoFaults, OpsMetricMihomoMonitor} {
		t.Run(metric, func(t *testing.T) {
			r := &adobeAlertTestRepo{rule: &OpsAlertRule{ID: 1, Name: "Mihomo", Enabled: true, Severity: "P1", MetricType: metric, Operator: ">=", Threshold: 1, WindowMinutes: 1, NotifyEmail: false}}
			if metric == OpsMetricMihomoMemory {
				r.rule.Threshold = 85
			}
			eval := NewOpsAlertEvaluatorService(&OpsService{settingRepo: settings}, r, NewEmailService(settings, nil), nil, nil, nil)
			now := time.Now().UTC()
			s := mihomoRuntimeSnapshot{SchemaVersion: 1, ObservedAt: now, SampleOK: true, Running: true, MemoryCurrent: 90, MemoryLimit: 100}
			write := func() {
				b, e := json.Marshal(s)
				require.NoError(t, e)
				require.NoError(t, os.WriteFile(path, b, 0600))
			}
			if metric == OpsMetricMihomoFaults {
				s.LastOOMAt = &now
			}
			if metric == OpsMetricMihomoMonitor {
				s.ObservedAt = now.Add(-2 * time.Minute)
			}
			write()
			eval.evaluateOnce(time.Minute)
			require.Equal(t, 1, r.created)
			require.NotNil(t, r.active)
			require.False(t, r.active.EmailSent)
			if metric == OpsMetricMihomoMemory {
				s.ObservedAt = now.Add(-2 * time.Minute)
				write()
				eval.evaluateOnce(time.Minute)
				require.NotNil(t, r.active, "missing telemetry cannot resolve a firing memory alarm")
			}
			s.ObservedAt = time.Now().UTC()
			s.MemoryCurrent = 10
			s.LastOOMAt = nil
			write()
			eval.evaluateOnce(time.Minute)
			require.Nil(t, r.active)
		})
	}
	require.Zero(t, smtp.messageCount())
}
