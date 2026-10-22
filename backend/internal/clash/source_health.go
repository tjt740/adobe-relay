package clash

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Source health is separate from the runtime revision: monitoring must never
// reload listeners, replace node definitions, or publish subscription secrets.
type Source struct {
	NodeCount     int            `json:"node_count"`
	ID            string         `json:"id"`
	Index         int            `json:"index"`
	URLHint       string         `json:"url_hint"`
	CheckedAt     time.Time      `json:"checked_at"`
	LastSuccessAt time.Time      `json:"last_success_at"`
	Failures      int            `json:"failures"`
	FetchError    string         `json:"fetch_error,omitempty"`
	HTTPStatus    int            `json:"http_status,omitempty"`
	Traffic       *SourceTraffic `json:"traffic,omitempty"`
}
type SourceTraffic struct {
	Upload     int64     `json:"upload"`
	Download   int64     `json:"download"`
	Total      int64     `json:"total"`
	ExpiresAt  int64     `json:"expires_at"`
	ObservedAt time.Time `json:"observed_at"`
}

func (t *SourceTraffic) remaining() int64 {
	if t == nil || t.Total <= 0 {
		return 0
	}
	return max(0, max(0, t.Total-t.Upload)-t.Download)
}
func (s Source) BlockReason(now time.Time) string {
	if s.Traffic == nil {
		return ""
	}
	if s.Traffic.ExpiresAt > 0 && !time.Unix(s.Traffic.ExpiresAt, 0).After(now) {
		return "expired"
	}
	if s.Traffic.Total > 0 && s.Traffic.remaining() == 0 {
		return "exhausted"
	}
	return ""
}
func sourceID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:12])
}
func sourceHint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/••••"
}
func parseSourceTraffic(raw string, now time.Time) *SourceTraffic {
	values := map[string]int64{}
	for _, part := range strings.Split(raw, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(pair[0]))
		switch key {
		case "upload", "download", "total", "expire":
		default:
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(pair[1]), 10, 64)
		if err != nil || n < 0 {
			return nil
		}
		if _, duplicate := values[key]; duplicate {
			return nil
		}
		values[key] = n
	}
	// Missing counters are unknown, never an implicit zero balance.
	_, u := values["upload"]
	_, d := values["download"]
	_, t := values["total"]
	if values["expire"] > 253402300799 {
		return nil
	}
	if !u || !d || !t {
		if values["expire"] > 0 {
			return &SourceTraffic{ExpiresAt: values["expire"], ObservedAt: now}
		}
		return nil
	}
	return &SourceTraffic{Upload: values["upload"], Download: values["download"], Total: values["total"], ExpiresAt: values["expire"], ObservedAt: now}
}
func normalizeSources(s *State) {
	old := map[string]Source{}
	for _, src := range s.Sources {
		old[src.ID] = src
	}
	urls := strings.Fields(s.URL)
	sources := make([]Source, 0, len(urls))
	for i, raw := range urls {
		src := old[sourceID(raw)]
		src.ID = sourceID(raw)
		src.Index = i + 1
		src.URLHint = sourceHint(raw)
		sources = append(sources, src)
	}
	s.Sources = sources
	for i, n := range s.Nodes {
		if n.OriginalName == "" {
			s.Nodes[i].OriginalName = n.Name
			if len(urls) > 1 {
				if _, name, ok := strings.Cut(n.Name, " · "); ok && strings.HasPrefix(n.Name, "订阅") {
					s.Nodes[i].OriginalName = name
				}
			}
		}
		if n.SourceID != "" {
			continue
		}
		index := 0
		if len(urls) > 1 {
			_, _ = fmt.Sscanf(n.Name, "订阅%d · ", &index)
		} else if len(urls) == 1 {
			index = 1
		}
		if index > 0 && index <= len(sources) {
			s.Nodes[i].SourceID = sources[index-1].ID
		}
	}
}
func fetchFeed(ctx context.Context, client *http.Client, raw string, index int) ([]Node, Source, error) {
	now := time.Now().UTC()
	src := Source{ID: sourceID(raw), Index: index, URLHint: sourceHint(raw), CheckedAt: now}
	fail := func(code string) ([]Node, Source, error) {
		src.FetchError = code
		return nil, src, fmt.Errorf("订阅 %d（%s）获取失败：%s", index, src.URLHint, code)
	}
	if err := validateURL(raw); err != nil {
		return fail("invalid_url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return fail("invalid_url")
	}
	req.Header.Set("User-Agent", "clash.meta/sub2api")
	res, err := client.Do(req)
	if err != nil {
		return fail("connection_failed")
	}
	defer res.Body.Close()
	src.HTTPStatus = res.StatusCode
	if res.StatusCode != http.StatusOK {
		return fail("http_error")
	}
	src.Traffic = parseSourceTraffic(res.Header.Get("subscription-userinfo"), now)
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return fail("read_failed")
	}
	nodes, err := parseSubscription(body)
	if err != nil {
		return fail("invalid_subscription")
	}
	src.NodeCount = len(nodes)
	src.LastSuccessAt = now
	for i := range nodes {
		nodes[i].SourceID = src.ID
		nodes[i].OriginalName = nodes[i].Name
	}
	return nodes, src, nil
}
func fetchSubscriptionDetails(ctx context.Context, client *http.Client, raw string) ([]Node, []Source, error) {
	urls, err := subscriptionURLs(raw)
	if err != nil {
		return nil, nil, err
	}
	batch, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	jobs := make([]feedJob, len(urls))
	sources := make([]Source, len(urls))
	bySource := make([][]Node, len(urls))
	for i, u := range urls {
		jobs[i] = feedJob{url: u, index: i + 1}
		sources[i] = Source{ID: sourceID(u), Index: i + 1, URLHint: sourceHint(u), FetchError: "not_checked"}
	}
	total := 0
	tooMany := false
	fetchFeeds(batch, client, jobs, func(r feedResult) {
		total += len(r.nodes)
		if total > maxNodes {
			tooMany = true
			cancel()
			return
		}
		sources[r.src.Index-1] = r.src
		bySource[r.src.Index-1] = r.nodes
	})
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if tooMany {
		return nil, nil, fmt.Errorf("合并后的订阅节点超过 %d 个", maxNodes)
	}
	all := []Node{}
	for i, nodes := range bySource {
		if len(urls) > 1 {
			for j := range nodes {
				name := []rune(fmt.Sprintf("订阅%d · %s", i+1, nodes[j].Name))
				if len(name) > 90 {
					name = name[:90]
				}
				nodes[j].Name = string(name)
				nodes[j].Config["name"] = nodes[j].Name
			}
		}
		all = append(all, nodes...)
	}
	return all, sources, nil
}
func mergeSourceObservation(old, observed Source) Source {
	if observed.FetchError == "not_checked" {
		old.Index, old.URLHint = observed.Index, observed.URLHint
		return old
	}
	observed.Failures = 0
	if observed.FetchError != "" {
		observed.Failures = min(old.Failures+1, 100)
		observed.LastSuccessAt = old.LastSuccessAt
	}
	// A missing header/failing refresh cannot prove a previously exhausted
	// subscription was renewed. Keep the dated observation until replaced.
	if observed.Traffic == nil {
		observed.Traffic = old.Traffic
	} else if old.Traffic != nil {
		if old.BlockReason(time.Now()) == "exhausted" && observed.Traffic.Total <= 0 {
			// Unknown/unlimited counters do not prove a known exhausted finite
			// subscription was replenished. Require fresh positive balance evidence.
			preserved := *old.Traffic
			if observed.Traffic.ExpiresAt > 0 {
				preserved.ExpiresAt = observed.Traffic.ExpiresAt
			}
			observed.Traffic = &preserved
		}
		if old.BlockReason(time.Now()) == "expired" && observed.Traffic.ExpiresAt == 0 {
			preserved := *observed.Traffic
			preserved.ExpiresAt = old.Traffic.ExpiresAt
			observed.Traffic = &preserved
		}
	}
	return observed
}
func sourceRefreshDue(src Source, now time.Time) bool {
	wait := 5 * time.Minute
	if src.Failures > 0 {
		wait = time.Duration(min(15, src.Failures)) * time.Minute
	}
	return src.CheckedAt.IsZero() || now.Sub(src.CheckedAt) >= wait
}
func (m *Manager) monitorSources(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		attempt, cancel := context.WithTimeout(ctx, 25*time.Second)
		_ = m.refreshSources(attempt)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (m *Manager) refreshSources(ctx context.Context) error {
	return m.refreshSourcesWithin(ctx, 20*time.Second)
}

func (m *Manager) refreshSourcesWithin(ctx context.Context, budget time.Duration) error {
	// A session lock elects one monitor without keeping a SQL transaction open
	// during network IO. Imports remain protected by the revision/URL comparison.
	conn, err := m.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var leader bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(240202617)`).Scan(&leader); err != nil || !leader {
		return err
	}
	defer func() {
		unlock, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		var released bool
		if e := conn.QueryRowContext(unlock, `SELECT pg_advisory_unlock(240202617)`).Scan(&released); e != nil || !released {
			// Do not return a session with an uncertain advisory lock to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	s, err := loadState(ctx, conn, false)
	if err != nil {
		return err
	}
	normalizeSources(&s)
	urls := strings.Fields(s.URL)
	jobs := []feedJob{}
	for i, src := range s.Sources {
		if sourceRefreshDue(src, time.Now()) {
			jobs = append(jobs, feedJob{url: urls[i], index: i + 1})
		}
	}
	// Oldest observations go first. Unfinished sources stay due for the next pass.
	sort.SliceStable(jobs, func(i, j int) bool {
		return s.Sources[jobs[i].index-1].CheckedAt.Before(s.Sources[jobs[j].index-1].CheckedAt)
	})
	observations := map[string]Source{}
	batch, cancel := context.WithTimeout(ctx, budget)
	fetchFeeds(batch, m.fetchClient, jobs, func(r feedResult) {
		old := s.Sources[r.src.Index-1]
		observations[r.src.ID] = mergeSourceObservation(old, r.src)
	})
	cancel()
	if len(observations) == 0 {
		return nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := loadState(ctx, tx, true)
	if err != nil {
		return err
	}
	if current.URL != s.URL || current.Revision != s.Revision {
		return nil
	}
	normalizeSources(&current)
	for i, src := range current.Sources {
		if v, ok := observations[src.ID]; ok {
			current.Sources[i] = v
		}
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return err
	}
	// No runtime reload and no configuration revision increment for telemetry.
	if _, err = tx.ExecContext(ctx, `UPDATE clash_subscription SET document=$1 WHERE id=1`, string(raw)); err != nil {
		return err
	}
	return tx.Commit()
}

// ProxySources exposes only source identity and admission, never feed URLs or
// node credentials. It also supports old documents before their first refresh.
type ProxySource struct {
	TrafficAvailable bool
	ID               string
	Blocked          bool
	Reason           string
}

func LoadProxySources(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (map[int64]ProxySource, error) {
	rows, err := q.QueryContext(ctx, `SELECT document FROM clash_subscription WHERE id=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int64]ProxySource{}
	if !rows.Next() {
		return result, rows.Err()
	}
	var raw []byte
	if err = rows.Scan(&raw); err != nil {
		return nil, err
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	normalizeSources(&s)
	sources := map[string]Source{}
	for _, src := range s.Sources {
		sources[src.ID] = src
	}
	for _, n := range s.Nodes {
		reason := sources[n.SourceID].BlockReason(time.Now())
		src := sources[n.SourceID]
		known := src.Traffic != nil && src.Traffic.Total > 0 && src.Traffic.remaining() > 0 && time.Since(src.Traffic.ObservedAt) <= 15*time.Minute
		result[n.ProxyID] = ProxySource{ID: n.SourceID, Blocked: reason != "", Reason: reason, TrafficAvailable: known}
	}
	return result, rows.Err()
}
