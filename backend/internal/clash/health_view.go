package clash

import (
	"context"
	"fmt"
	"time"
)

// A full sweep must fit the admission window, even with many failed endpoints.
// The automatic worker has 24 probes and at most three 6s batches per 30s round.
func ProbeFreshness(nodes int) time.Duration {
	rounds := (nodes + 71) / 72
	return min(4*time.Minute, max(45*time.Second, time.Duration(rounds)*30*time.Second+15*time.Second))
}

type SourceView struct {
	ProxyIDs          []int64    `json:"proxy_ids"`
	BlockedProxyIDs   []int64    `json:"blocked_proxy_ids"`
	GenerationBlocked int        `json:"generation_blocked"`
	ID                string     `json:"id"`
	Index             int        `json:"index"`
	URLHint           string     `json:"url_hint"`
	Status            string     `json:"status"`
	FetchError        string     `json:"fetch_error,omitempty"`
	HTTPStatus        int        `json:"http_status,omitempty"`
	CheckedAt         time.Time  `json:"checked_at"`
	LastSuccessAt     time.Time  `json:"last_success_at"`
	TrafficKnown      bool       `json:"traffic_known"`
	RemainingBytes    *int64     `json:"remaining_bytes"`
	TotalBytes        *int64     `json:"total_bytes"`
	TrafficObservedAt *time.Time `json:"traffic_observed_at"`
	ExpiresAt         *time.Time `json:"expires_at"`
	Nodes             int        `json:"nodes"`
	Healthy           int        `json:"healthy"`
	Failed            int        `json:"failed"`
	Unknown           int        `json:"unknown"`
	BoundAccounts     int        `json:"bound_accounts"`
}
type GroupCapacity struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Total        int    `json:"total"`
	Enabled      int    `json:"enabled"`
	Available    int    `json:"available"`
	WaitingProxy int    `json:"waiting_proxy"`
}
type HealthView struct {
	CheckedAt time.Time       `json:"checked_at"`
	Sources   []SourceView    `json:"sources"`
	Groups    []GroupCapacity `json:"groups"`
}

func (m *Manager) Health(ctx context.Context) (HealthView, error) {
	v := HealthView{CheckedAt: time.Now().UTC(), Sources: []SourceView{}, Groups: []GroupCapacity{}}
	s, err := loadState(ctx, m.db, false)
	if err != nil {
		return v, err
	}
	normalizeSources(&s)
	type sample struct {
		status   string
		failures int
		checked  time.Time
	}
	samples := map[int64]sample{}
	rows, err := m.db.QueryContext(ctx, `SELECT h.proxy_id,h.status,h.failures,h.checked_at FROM proxy_failover_health h JOIN proxies p ON p.id=h.proxy_id WHERE p.deleted_at IS NULL AND p.status='active' AND (p.expires_at IS NULL OR p.expires_at>NOW()) AND h.checked_at>=p.updated_at`)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var id int64
		var h sample
		if err = rows.Scan(&id, &h.status, &h.failures, &h.checked); err != nil {
			rows.Close()
			return v, err
		}
		samples[id] = h
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	bound := map[int64]int{}
	rows, err = m.db.QueryContext(ctx, `SELECT proxy_id,count(*) FROM accounts WHERE deleted_at IS NULL AND proxy_id IS NOT NULL GROUP BY proxy_id`)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var id int64
		var n int
		if err = rows.Scan(&id, &n); err != nil {
			rows.Close()
			return v, err
		}
		bound[id] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	blocked := map[int64]bool{}
	rows, err = m.db.QueryContext(ctx, `SELECT p.id FROM proxies p WHERE
     EXISTS(SELECT 1 FROM adobe_route_exclusions e WHERE e.proxy_id=p.id)
     OR EXISTS(SELECT 1 FROM adobe_route_health h WHERE h.proxy_id=p.id AND h.proxy_updated_at=p.updated_at AND h.retry_after IS NOT NULL)`)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return v, err
		}
		blocked[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	for _, src := range s.Sources {
		sv := SourceView{ProxyIDs: []int64{}, BlockedProxyIDs: []int64{}, ID: src.ID, Index: src.Index, URLHint: src.URLHint, Status: "checking", FetchError: src.FetchError, HTTPStatus: src.HTTPStatus, CheckedAt: src.CheckedAt, LastSuccessAt: src.LastSuccessAt}
		if t := src.Traffic; t != nil {
			sv.TrafficObservedAt = &t.ObservedAt
			if t.Total > 0 && time.Since(t.ObservedAt) <= 15*time.Minute {
				remaining := t.remaining()
				total := t.Total
				sv.RemainingBytes = &remaining
				sv.TotalBytes = &total
				sv.TrafficKnown = true
			}
			if t.ExpiresAt > 0 {
				expiry := time.Unix(t.ExpiresAt, 0)
				sv.ExpiresAt = &expiry
			}
		}
		for _, node := range s.Nodes {
			if node.SourceID == src.ID && node.ProxyID > 0 {
				sv.ProxyIDs = append(sv.ProxyIDs, node.ProxyID)
				if blocked[node.ProxyID] {
					sv.BlockedProxyIDs = append(sv.BlockedProxyIDs, node.ProxyID)
					sv.GenerationBlocked++
				}
			}
			if !node.Enabled || node.SourceID != src.ID {
				continue
			}
			sv.Nodes++
			sv.BoundAccounts += bound[node.ProxyID]
			h, ok := samples[node.ProxyID]
			if !ok || time.Since(h.checked) > ProbeFreshness(len(s.Nodes)) {
				sv.Unknown++
				continue
			}
			switch {
			case h.status == "healthy":
				sv.Healthy++
			case h.status == "unhealthy" && h.failures >= 2:
				sv.Failed++
			default:
				sv.Unknown++
			}
		}
		switch {
		case src.BlockReason(time.Now()) != "":
			sv.Status = src.BlockReason(time.Now())
		case sv.Nodes > 0 && sv.Failed == sv.Nodes:
			sv.Status = "unavailable"
		case src.FetchError != "":
			sv.Status = "fetch_failed"
		case sv.Failed > 0:
			sv.Status = "degraded"
		case sv.Healthy > 0:
			sv.Status = "healthy"
		case !src.CheckedAt.IsZero() && time.Since(src.CheckedAt) > 15*time.Minute:
			sv.Status = "stale"
		}
		v.Sources = append(v.Sources, sv)
	}
	rows, err = m.db.QueryContext(ctx, `SELECT g.id,g.name,count(a.id),count(a.id) FILTER(WHERE a.schedulable),
 count(a.id) FILTER(WHERE a.status='active' AND a.schedulable AND (a.expires_at IS NULL OR a.auto_pause_on_expired IS NOT TRUE OR a.expires_at>NOW()) AND COALESCE(a.temp_unschedulable_until,NOW())<=NOW() AND COALESCE(a.overload_until,NOW())<=NOW() AND COALESCE(a.rate_limit_reset_at,NOW())<=NOW()),
 count(a.id) FILTER(WHERE a.proxy_auto_paused)
 FROM groups g JOIN account_groups ag ON ag.group_id=g.id JOIN accounts a ON a.id=ag.account_id AND a.deleted_at IS NULL
 WHERE g.deleted_at IS NULL AND g.platform='adobe' AND g.status='active' GROUP BY g.id ORDER BY g.id`)
	if err != nil {
		return v, fmt.Errorf("group capacity unavailable: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var g GroupCapacity
		if err = rows.Scan(&g.ID, &g.Name, &g.Total, &g.Enabled, &g.Available, &g.WaitingProxy); err != nil {
			return v, err
		}
		v.Groups = append(v.Groups, g)
	}
	return v, rows.Err()
}
