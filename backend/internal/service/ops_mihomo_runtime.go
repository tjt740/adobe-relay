package service

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

const (
	OpsMetricMihomoMemory  = "mihomo_memory_percent"
	OpsMetricMihomoFaults  = "mihomo_abnormal_events"
	OpsMetricMihomoMonitor = "mihomo_monitor_unavailable"
)
const mihomoSnapshotMaxAge = 90 * time.Second

type mihomoRuntimeSnapshot struct {
	SchemaVersion int        `json:"schema_version"`
	ObservedAt    time.Time  `json:"observed_at"`
	SampleOK      bool       `json:"sample_ok"`
	Running       bool       `json:"running"`
	MemoryCurrent uint64     `json:"memory_current_bytes"`
	MemoryLimit   uint64     `json:"memory_limit_bytes"`
	LastOOMAt     *time.Time `json:"last_oom_at,omitempty"`
	LastRestartAt *time.Time `json:"last_restart_at,omitempty"`
}

// The host publishes a credential-free atomic file on the existing data mount.
// Never grant the application access to Docker or host cgroups for monitoring.
func readMihomoRuntimeSnapshot(path string, now time.Time) (*mihomoRuntimeSnapshot, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 16*1024+1))
	var s mihomoRuntimeSnapshot
	if d.Decode(&s) != nil {
		return nil, false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, false
	}
	if s.SchemaVersion != 1 || !s.SampleOK || !s.Running || s.MemoryLimit == 0 || s.ObservedAt.IsZero() || now.Sub(s.ObservedAt) > mihomoSnapshotMaxAge || s.ObservedAt.After(now.Add(5*time.Second)) {
		return nil, false
	}
	return &s, true
}
func mihomoMetricFromSnapshot(metric, path string, now time.Time) (float64, bool) {
	s, ok := readMihomoRuntimeSnapshot(path, now)
	if metric == OpsMetricMihomoMonitor {
		if !ok {
			return 1, true
		}
		return 0, true
	}
	if !ok {
		return 0, false
	} // Missing telemetry must never resolve an active memory alarm.
	switch metric {
	case OpsMetricMihomoMemory:
		return float64(s.MemoryCurrent) * 100 / float64(s.MemoryLimit), true
	case OpsMetricMihomoFaults:
		value := 0.0
		for _, at := range []*time.Time{s.LastOOMAt, s.LastRestartAt} {
			if at != nil && !at.After(now.Add(5*time.Second)) && now.Sub(*at) <= 5*time.Minute {
				value++
			}
		}
		return value, true
	}
	return 0, false
}
func currentMihomoMetric(metric string, now time.Time) (float64, bool) {
	path := os.Getenv("MIHOMO_RUNTIME_SNAPSHOT")
	if path == "" {
		path = "/app/data/mihomo-runtime.json"
	}
	return mihomoMetricFromSnapshot(metric, path, now)
}
