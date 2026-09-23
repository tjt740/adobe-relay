package clash

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type State struct {
	URL       string    `json:"url"`
	Revision  int64     `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
	NextPort  int       `json:"next_port"`
	Nodes     []Node    `json:"nodes"`
}

type View struct {
	Configured bool       `json:"configured"`
	Ready      bool       `json:"ready"`
	URL        string     `json:"url"`
	URLHint    string     `json:"url_hint"`
	Revision   int64      `json:"revision"`
	UpdatedAt  time.Time  `json:"updated_at"`
	Nodes      []NodeView `json:"nodes"`
}

type ImportRequest struct {
	URL      string   `json:"url"`
	Names    []string `json:"names"`
	Revision int64    `json:"revision"`
}

type Manager struct {
	controllerListen string
	db               *sql.DB
	controller       string
	secret           string
	host             string
	portStart        int
	client           *http.Client
	fetchClient      *http.Client
}

func NewManager(db *sql.DB) *Manager {
	start, _ := strconv.Atoi(os.Getenv("CLASH_PORT_START"))
	if start == 0 {
		start = 20000
	}
	host := os.Getenv("CLASH_PROXY_HOST")
	if host == "" {
		host = "mihomo"
	}
	listen := os.Getenv("CLASH_CONTROLLER_LISTEN")
	if listen == "" {
		listen = "0.0.0.0:9090"
	}
	return &Manager{controllerListen: listen, db: db, controller: strings.TrimRight(os.Getenv("CLASH_CONTROLLER_URL"), "/"), secret: os.Getenv("CLASH_CONTROLLER_SECRET"), host: host, portStart: start, client: &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, fetchClient: subscriptionClient()}
}

func (m *Manager) configured() bool {
	u, err := url.Parse(m.controller)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && m.secret != "" && m.host != "" && m.portStart > 0 && m.portStart <= 60000
}

func loadState(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, lock bool) (State, error) {
	query := "SELECT document FROM clash_subscription WHERE id=1"
	if lock {
		query += " FOR UPDATE"
	}
	var raw []byte
	if err := q.QueryRowContext(ctx, query).Scan(&raw); err != nil {
		return State{}, err
	}
	var s State
	err := json.Unmarshal(raw, &s)
	return s, err
}

func (m *Manager) view(s State) View {
	hint := ""
	if u, err := url.Parse(s.URL); err == nil && u.Host != "" {
		hint = u.Scheme + "://" + u.Host + "/••••"
	}
	v := View{Configured: m.configured(), URL: s.URL, URLHint: hint, Revision: s.Revision, UpdatedAt: s.UpdatedAt, Nodes: make([]NodeView, 0, len(s.Nodes))}
	for _, n := range s.Nodes {
		v.Nodes = append(v.Nodes, m.nodeView(n))
	}
	return v
}

// Only expose display metadata here; outbound and listener credentials stay server-side.
func (m *Manager) nodeView(n Node) NodeView {
	server, _ := n.Config["server"].(string)
	port, _ := strconv.Atoi(fmt.Sprint(n.Config["port"]))
	v := NodeView{Name: n.Name, Type: n.Type, Server: server, ServerPort: port, ProxyID: n.ProxyID, Enabled: n.Enabled}
	if n.ProxyID != 0 {
		v.ProxyHost, v.ProxyPort = m.host, n.Port
	}
	return v
}

func (m *Manager) Status(ctx context.Context) (View, error) {
	s, err := loadState(ctx, m.db, false)
	if err != nil {
		return View{}, err
	}
	v := m.view(s)
	if v.Configured {
		if len(s.Nodes) > 0 {
			v.Ready = m.matches(ctx, s)
		} else {
			v.Ready = m.request(ctx, http.MethodGet, "/version", nil, nil) == nil
		}
	}
	return v, nil
}

func (m *Manager) Preview(ctx context.Context, raw string) (View, error) {
	s, err := loadState(ctx, m.db, false)
	if err != nil {
		return View{}, err
	}
	if raw == "" {
		raw = s.URL
	}
	nodes, err := fetchSubscription(ctx, m.fetchClient, raw)
	if err != nil {
		return View{}, err
	}
	existing := map[string]Node{}
	for _, n := range s.Nodes {
		existing[n.Name] = n
	}
	v := m.view(s)
	v.Nodes = make([]NodeView, 0, len(nodes))
	for _, n := range nodes {
		old, ok := existing[n.Name]
		n.ProxyID, n.Port, n.Enabled = old.ProxyID, old.Port, !ok || old.Enabled
		v.Nodes = append(v.Nodes, m.nodeView(n))
	}
	return v, nil
}

func (m *Manager) IsManaged(ctx context.Context, id int64) (bool, error) {
	var yes bool
	err := m.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM clash_subscription, jsonb_array_elements(document->'nodes') n WHERE (n->>'proxy_id')::bigint=$1)`, id).Scan(&yes)
	return yes, err
}

// Import validates and applies the full candidate before committing the database. Failed
// runtime updates are compensated with the old document, including after request cancellation.
func (m *Manager) Import(ctx context.Context, req ImportRequest) (View, error) {
	if !m.configured() {
		return View{}, errors.New("Clash 运行服务尚未配置，请按部署文档启动 Mihomo")
	}
	if len(req.Names) > maxNodes {
		return View{}, errors.New("选择的节点过多")
	}
	snapshot, err := loadState(ctx, m.db, false)
	if err != nil {
		return View{}, err
	}
	raw := strings.TrimSpace(req.URL)
	if raw == "" {
		raw = snapshot.URL
	}
	fetched, err := fetchSubscription(ctx, m.fetchClient, raw)
	if err != nil {
		return View{}, err
	}
	selected := make(map[string]bool, len(req.Names))
	for _, n := range req.Names {
		if selected[n] {
			return View{}, errors.New("节点选择重复")
		}
		selected[n] = true
	}
	byName := map[string]Node{}
	for _, n := range fetched {
		byName[n.Name] = n
	}
	for n := range selected {
		if _, ok := byName[n]; !ok {
			return View{}, errors.New("订阅节点已变化，请重新预览后导入")
		}
	}
	// Once external changes start, finish the bounded transaction even if the HTTP
	// caller disconnects. Otherwise cancellation releases the row lock mid-restore.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
	defer cancel()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return View{}, err
	}
	defer tx.Rollback()
	previous, err := loadState(ctx, tx, true)
	if err != nil {
		return View{}, err
	}
	if previous.Revision != req.Revision {
		return View{}, errors.New("订阅已被其他操作更新，请重新预览")
	}
	next := State{URL: raw, Revision: previous.Revision + 1, UpdatedAt: time.Now().UTC(), NextPort: previous.NextPort, Nodes: make([]Node, 0)}
	if next.NextPort == 0 {
		next.NextPort = m.portStart
	}
	oldByName := map[string]Node{}
	for _, n := range previous.Nodes {
		oldByName[n.Name] = n
	}
	for _, n := range fetched {
		if !selected[n.Name] {
			continue
		}
		if old, ok := oldByName[n.Name]; ok {
			n.ProxyID, n.Port, n.Password = old.ProxyID, old.Port, old.Password
		} else {
			if next.NextPort > min(m.portStart+4095, 65535) {
				return View{}, errors.New("Clash 节点端口已分配完毕")
			}
			n.Port = next.NextPort
			next.NextPort++
			secret := make([]byte, 24)
			if _, err = rand.Read(secret); err != nil {
				return View{}, err
			}
			n.Password = hex.EncodeToString(secret)
			err = tx.QueryRowContext(ctx, `INSERT INTO proxies(name,protocol,host,port,username,password,status) VALUES($1,'http',$2,$3,'clash',$4,'active') RETURNING id`, "Clash · "+n.Name, m.host, n.Port, n.Password).Scan(&n.ProxyID)
			if err != nil {
				return View{}, err
			}
		}
		n.Enabled = true
		next.Nodes = append(next.Nodes, n)
		delete(oldByName, n.Name)
	}
	// Keep retired assignments as rejecting listeners. A stale account snapshot can never
	// accidentally use a newly allocated node or fall through to a direct connection.
	for _, old := range previous.Nodes {
		if _, ok := oldByName[old.Name]; ok {
			old.Enabled = false
			old.Config = nil
			next.Nodes = append(next.Nodes, old)
		}
	}
	for _, n := range next.Nodes {
		status := "inactive"
		if n.Enabled {
			status = "active"
		}
		result, e := tx.ExecContext(ctx, `UPDATE proxies SET status=$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL`, status, n.ProxyID)
		if e != nil {
			return View{}, e
		}
		count, e := result.RowsAffected()
		if e != nil || count != 1 {
			return View{}, errors.New("Clash 代理记录已被删除，请恢复数据库记录后重试")
		}
	}

	if len(next.Nodes) > 0 {
		ids := make([]int64, 0, len(next.Nodes))
		for _, n := range next.Nodes {
			ids = append(ids, n.ProxyID)
		}
		rawIDs, _ := json.Marshal(ids)
		rows, e := tx.QueryContext(ctx, `UPDATE accounts SET extra=COALESCE(extra,'{}'::jsonb)-'upstream_billing_probe'-'ollama_cloud_usage_snapshot',updated_at=NOW() WHERE deleted_at IS NULL AND proxy_id IN (SELECT jsonb_array_elements_text($1::jsonb)::bigint) RETURNING id`, string(rawIDs))
		if e != nil {
			return View{}, e
		}
		accountIDs := make([]int64, 0)
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return View{}, e
			}
			accountIDs = append(accountIDs, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return View{}, e
		}
		for start := 0; start < len(accountIDs); start += 500 {
			payload, _ := json.Marshal(map[string]any{"account_ids": accountIDs[start:min(start+500, len(accountIDs))]})
			if _, e = tx.ExecContext(ctx, `INSERT INTO scheduler_outbox(event_type,payload) VALUES('account_bulk_changed',$1)`, string(payload)); e != nil {
				return View{}, e
			}
		}
	}
	payload, err := json.Marshal(next)
	if err != nil {
		return View{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE clash_subscription SET document=$1,updated_at=NOW() WHERE id=1`, string(payload)); err != nil {
		return View{}, err
	}
	// Save in the transaction first; the externally visible change is committed only after apply.
	if err = m.apply(ctx, next); err != nil {
		_ = tx.Rollback()
		m.restorePersisted()
		return View{}, err
	}
	if err = tx.Commit(); err != nil {
		m.restorePersisted()
		return View{}, errors.New("保存订阅失败，已尝试恢复原节点")
	}
	v := m.view(next)
	v.Ready = true
	return v, nil
}

// After a failed apply/commit, read the authoritative document under a fresh
// lock. A commit error can be ambiguous, and another import may have completed.
func (m *Manager) restorePersisted() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	s, err := loadState(ctx, tx, true)
	if err == nil {
		_ = m.apply(ctx, s)
	}
}

func marker(s State) string {
	b, _ := json.Marshal(s)
	sum := sha256.Sum256(b)
	return "sub2api-revision-" + hex.EncodeToString(sum[:12])
}

func runtimeConfig(s State) map[string]any {
	proxies := make([]map[string]any, 0)
	listeners := make([]map[string]any, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		target := "REJECT"
		if n.Enabled {
			cfg := make(map[string]any, len(n.Config))
			for k, v := range n.Config {
				cfg[k] = v
			}
			target = fmt.Sprintf("sub2api-node-%d", n.ProxyID)
			cfg["name"] = target
			proxies = append(proxies, cfg)
		}
		listeners = append(listeners, map[string]any{"name": fmt.Sprintf("sub2api-in-%d", n.ProxyID), "type": "mixed", "listen": "0.0.0.0", "port": n.Port, "udp": false, "proxy": target, "users": []map[string]string{{"username": "clash", "password": n.Password}}})
	}
	return map[string]any{"mode": "rule", "log-level": "warning", "ipv6": true, "allow-lan": true, "proxies": proxies, "listeners": listeners, "proxy-groups": []any{map[string]any{"name": marker(s), "type": "select", "proxies": []string{"REJECT"}}}, "rules": []string{"MATCH,REJECT"}}
}

func (m *Manager) request(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.controller+path, reader)
	if err != nil {
		return errors.New("Clash 控制地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+m.secret)
	req.Header.Set("Content-Type", "application/json")
	res, err := m.client.Do(req)
	if err != nil {
		return errors.New("无法连接 Clash 运行服务，请检查 Mihomo 状态")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Clash 配置或请求被拒绝（HTTP %d），请检查节点格式和控制密钥", res.StatusCode)
	}
	if out != nil {
		if err = json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
			return errors.New("Clash 返回无效响应")
		}
	}
	return nil
}

func (m *Manager) matches(ctx context.Context, s State) bool {
	var out struct {
		Proxies map[string]json.RawMessage `json:"proxies"`
	}
	if m.request(ctx, http.MethodGet, "/proxies", nil, &out) != nil {
		return false
	}
	_, ok := out.Proxies[marker(s)]
	return ok && m.listenersReady(ctx, s) == nil
}

func (m *Manager) apply(ctx context.Context, s State) error {
	cfg := runtimeConfig(s)
	cfg["external-controller"] = m.controllerListen
	cfg["secret"] = m.secret
	payload, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err = m.request(ctx, http.MethodPut, "/configs?force=true", map[string]string{"payload": string(payload)}, nil); err != nil {
		return err
	}
	return m.listenersReady(ctx, s)
}

func (m *Manager) listenersReady(ctx context.Context, s State) error {
	// Mihomo may report a successful config reload even if a listener fails to bind.
	for _, n := range s.Nodes {
		conn, e := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(m.host, strconv.Itoa(n.Port)))
		if e != nil {
			return fmt.Errorf("Clash 节点入口 %d 未就绪，请检查端口冲突及服务网络", n.Port)
		}
		_ = conn.Close()
	}
	return nil
}

// Run restores persisted nodes after either process restarts and repairs lost runtime config.
// The database row lock also serializes restores with imports across app instances.
func (m *Manager) Run(ctx context.Context) {
	if !m.configured() {
		return
	}
	reconcile := func() {
		attempt, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()
		tx, err := m.db.BeginTx(attempt, nil)
		if err != nil {
			return
		}
		defer tx.Rollback()
		s, err := loadState(attempt, tx, true)
		if err != nil || s.Revision == 0 {
			return
		}
		if !m.matches(attempt, s) {
			_ = m.apply(attempt, s)
		}
	}
	reconcile()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reconcile()
		}
	}
}
