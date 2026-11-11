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

func TestMihomoMemoryTelemetryDoesNotConfuseMissingWithRecovered(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "sample.json")
	s := mihomoRuntimeSnapshot{SchemaVersion: 1, ObservedAt: now, SampleOK: true, Running: true, MemoryCurrent: 900, MemoryLimit: 1000}
	put := func() {
		b, e := json.Marshal(s)
		require.NoError(t, e)
		require.NoError(t, os.WriteFile(path, b, 0600))
	}
	put()
	v, ok := mihomoMetricFromSnapshot(OpsMetricMihomoMemory, path, now)
	require.True(t, ok)
	require.Equal(t, 90.0, v)
	v, ok = mihomoMetricFromSnapshot(OpsMetricMihomoMonitor, path, now)
	require.True(t, ok)
	require.Zero(t, v)
	recent := now.Add(-time.Minute)
	s.LastOOMAt = &recent
	s.LastRestartAt = &recent
	put()
	v, ok = mihomoMetricFromSnapshot(OpsMetricMihomoFaults, path, now)
	require.True(t, ok)
	require.Equal(t, 2.0, v)
	for _, bad := range []string{"stale", "future", "failed", "stopped", "no-limit"} {
		t.Run(bad, func(t *testing.T) {
			old := s
			defer func() { s = old }()
			switch bad {
			case "stale":
				s.ObservedAt = now.Add(-91 * time.Second)
			case "future":
				s.ObservedAt = now.Add(time.Minute)
			case "failed":
				s.SampleOK = false
			case "stopped":
				s.Running = false
			case "no-limit":
				s.MemoryLimit = 0
			}
			put()
			_, valid := mihomoMetricFromSnapshot(OpsMetricMihomoMemory, path, now)
			require.False(t, valid)
			value, valid := mihomoMetricFromSnapshot(OpsMetricMihomoMonitor, path, now)
			require.True(t, valid)
			require.Equal(t, 1.0, value)
		})
	}
	require.NoError(t, os.WriteFile(path, []byte(`{"schema_version":1}`), 0600))
	_, ok = readMihomoRuntimeSnapshot(path, now)
	require.False(t, ok)
	require.NoError(t, os.Remove(path))
	v, ok = mihomoMetricFromSnapshot(OpsMetricMihomoMonitor, path, now)
	require.True(t, ok)
	require.Equal(t, 1.0, v)
}

func TestMihomoEvaluatorUsesRealtimeNotTruncatedUsageWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.json")
	now := time.Now().UTC()
	raw, _ := json.Marshal(mihomoRuntimeSnapshot{SchemaVersion: 1, ObservedAt: now, SampleOK: true, Running: true, MemoryCurrent: 85, MemoryLimit: 100})
	require.NoError(t, os.WriteFile(path, raw, 0600))
	t.Setenv("MIHOMO_RUNTIME_SNAPSHOT", path)
	var evaluator *OpsAlertEvaluatorService
	value, ok := evaluator.computeRuleMetric(context.Background(), &OpsAlertRule{MetricType: OpsMetricMihomoMemory}, nil, now.Add(-time.Minute), now.Truncate(time.Minute), "", nil)
	require.True(t, ok)
	require.Equal(t, 85.0, value)
}
