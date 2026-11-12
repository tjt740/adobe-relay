package clash

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"unicode/utf8"
)

// Settings use source identity rather than row position. They are independent
// of the runtime revision, so editing them never reloads listeners.
type SettingsRequest struct {
	Priorities       map[string]int    `json:"priorities"`
	Notes            map[string]string `json:"notes"`
	Revision         int64             `json:"revision"`
	SettingsRevision *int64            `json:"settings_revision"`
}

func prioritiesByURL(s State) map[string]int {
	priorities := make(map[string]int)
	for _, u := range strings.Fields(s.URL) {
		priorities[u] = s.Priorities[sourceID(u)]
	}
	return priorities
}

func validatePriorities(priorities map[string]int) error {
	size := 0
	for u, priority := range priorities {
		size += len(u)
		if priority < 0 || priority > 100 {
			return errors.New("订阅优先级必须是 0–100 的整数")
		}
	}
	if size > 64<<10 {
		return errors.New("订阅优先级内容过长")
	}
	return nil
}

func notesByURL(s State) map[string]string {
	notes := make(map[string]string)
	for _, u := range strings.Fields(s.URL) {
		notes[u] = s.Notes[sourceID(u)]
	}
	return notes
}

func validateNotes(notes map[string]string) error {
	size := 0
	for u, note := range notes {
		size += len(u) + len(note)
		if !utf8.ValidString(note) || utf8.RuneCountInString(note) > 200 || strings.ContainsAny(note, "\r\n\x00") {
			return errors.New("订阅备注最多 200 个字符，不能包含换行")
		}
	}
	if size > 256<<10 {
		return errors.New("订阅备注内容过长")
	}
	return nil
}

func checkSettingsRevision(s State, notes map[string]string, priorities map[string]int, revision *int64) error {
	if (notes != nil || priorities != nil) && (revision == nil || s.SettingsRevision != *revision) {
		return errors.New("订阅设置已被其他操作更新，请重新打开后编辑")
	}
	return nil
}

func updatedSettings(previous State, urls []string, edits map[string]string, priorityEdits map[string]int) (map[string]string, map[string]int, int64) {
	notes := make(map[string]string)
	priorities := make(map[string]int)
	for _, u := range urls {
		priority := previous.Priorities[sourceID(u)]
		if edit, ok := priorityEdits[u]; ok {
			priority = edit
		}
		if priority != 0 {
			priorities[sourceID(u)] = priority
		}
		note := previous.Notes[sourceID(u)]
		if edit, ok := edits[u]; ok {
			note = strings.TrimSpace(edit)
		}
		if note != "" {
			notes[sourceID(u)] = note
		}
	}
	revision := previous.SettingsRevision
	if !maps.Equal(previous.Notes, notes) || !maps.Equal(previous.Priorities, priorities) {
		revision++
	}
	return notes, priorities, revision
}

func (m *Manager) UpdateSettings(ctx context.Context, req SettingsRequest) (View, error) {
	if err := validateNotes(req.Notes); err != nil {
		return View{}, err
	}
	if err := validatePriorities(req.Priorities); err != nil {
		return View{}, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return View{}, err
	}
	defer tx.Rollback()
	s, err := loadState(ctx, tx, true)
	if err != nil {
		return View{}, err
	}
	if s.Revision != req.Revision {
		return View{}, errors.New("订阅链接已变化，请重新打开后编辑设置")
	}
	if err = checkSettingsRevision(s, req.Notes, req.Priorities, req.SettingsRevision); err != nil {
		return View{}, err
	}
	known := notesByURL(s)
	for u := range req.Notes {
		if _, ok := known[u]; !ok {
			return View{}, errors.New("新增订阅的设置请随导入一起保存")
		}
	}
	for u := range req.Priorities {
		if _, ok := known[u]; !ok {
			return View{}, errors.New("新增订阅的设置请随导入一起保存")
		}
	}
	s.Notes, s.Priorities, s.SettingsRevision = updatedSettings(s, strings.Fields(s.URL), req.Notes, req.Priorities)
	payload, err := json.Marshal(s)
	if err != nil {
		return View{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE clash_subscription SET document=$1 WHERE id=1`, string(payload)); err != nil {
		return View{}, err
	}
	if err = tx.Commit(); err != nil {
		return View{}, err
	}
	return m.view(s), nil
}

// Append downloads only URLs absent from the saved document. Failed additions
// are returned to the caller but not saved, so they remain eligible for retry.
// Existing source observations and nodes are copied at commit under the lock.
func (m *Manager) fetchNewSubscriptions(ctx context.Context, previous State, raw string) (string, []Node, []Source, []Source, error) {
	incoming, err := subscriptionURLs(raw)
	if err != nil {
		return "", nil, nil, nil, err
	}
	known := make(map[string]bool)
	urls := strings.Fields(previous.URL)
	for _, u := range urls {
		known[u] = true
	}
	for _, u := range incoming {
		if !known[u] {
			urls = append(urls, u)
		}
	}
	// Validate the deduplicated saved list, not a duplicated copy sent by a
	// client that includes existing links along with its additions.
	if _, err = subscriptionURLs(strings.Join(urls, "\n")); err != nil {
		return "", nil, nil, nil, err
	}
	if len(urls) == len(known) {
		return "", nil, nil, nil, errors.New("没有新增订阅链接；更新已有节点请使用全部刷新")
	}
	nodes, sources, err := fetchSubscriptionURLs(ctx, m.fetchClient, urls, known)
	if err != nil {
		return "", nil, nil, nil, err
	}
	normalizeSources(&previous)
	oldSources := make(map[string]Source)
	for _, src := range previous.Sources {
		oldSources[src.ID] = src
	}
	savedURLs := make([]string, 0, len(urls))
	savedSources, skipped := []Source{}, []Source{}
	successes := 0
	for i, u := range urls {
		src := sources[i]
		if known[u] {
			src = oldSources[sourceID(u)]
		} else if src.FetchError != "" {
			skipped = append(skipped, src)
			continue
		} else {
			successes++
		}
		savedURLs = append(savedURLs, u)
		src.Index = len(savedURLs)
		savedSources = append(savedSources, src)
	}
	if successes == 0 {
		return "", nil, nil, nil, errors.New("新增订阅均未成功获取，现有订阅和节点未改动；请检测新链接后重试")
	}
	// Reserve every existing name, including disabled legacy entries. Never let a
	// new provider reuse another source's node name or proxy identity.
	combined := append(append([]Node(nil), previous.Nodes...), nodes...)
	combined, _, err = preserveFailedSources(previous, combined, nil)
	if err != nil {
		return "", nil, nil, nil, err
	}
	return strings.Join(savedURLs, "\n"), combined[len(previous.Nodes):], savedSources, skipped, nil
}
