// Package clash manages a dedicated Mihomo instance and persistent proxy assignments.
package clash

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const maxBody = 8 << 20
const maxNodes = 512

type Node struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Config   map[string]any `json:"config"`
	ProxyID  int64          `json:"proxy_id"`
	Port     int            `json:"port"`
	Password string         `json:"password"`
	Enabled  bool           `json:"enabled"`
}

type NodeView struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
	ProxyID    int64  `json:"proxy_id"`
	ProxyHost  string `json:"proxy_host"`
	ProxyPort  int    `json:"proxy_port"`
	Enabled    bool   `json:"enabled"`
}

func parseSubscription(body []byte) ([]Node, error) {
	if len(body) > maxBody {
		return nil, errors.New("订阅内容超过 8 MiB")
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	// Decode only outbound nodes. Never execute subscription rules, providers, listeners or scripts.
	if err := yaml.Unmarshal(body, &doc); err == nil && len(doc.Proxies) > 0 {
		return normalizeProxyConfigs(doc.Proxies)
	}
	decoded, err := decodeBase64Subscription(body)
	if err != nil {
		return nil, errors.New("订阅没有 proxies 节点，请使用 Clash YAML 或 Base64 节点订阅链接")
	}
	configs := make([]map[string]any, 0)
	for _, line := range strings.Split(string(decoded), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		config, ok, err := parseProxyURI(line)
		if err != nil {
			return nil, err
		}
		if ok {
			configs = append(configs, config)
		}
	}
	if len(configs) == 0 {
		return nil, errors.New("订阅没有可识别的节点")
	}
	return normalizeProxyConfigs(configs)
}

func decodeBase64Subscription(body []byte) ([]byte, error) {
	raw := strings.TrimSpace(string(body))
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") {
		return nil, errors.New("不是 Base64 订阅")
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(raw)
		if err == nil && strings.Contains(string(decoded), "://") {
			return decoded, nil
		}
	}
	return nil, errors.New("不是 Base64 节点订阅")
}

func parseProxyURI(raw string) (map[string]any, bool, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, false, errors.New("节点链接无效")
	}
	name := strings.TrimSpace(u.Fragment)
	if name == "" {
		name = u.Hostname()
	}
	if isSubscriptionMetadata(name) {
		return nil, false, nil
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, false, fmt.Errorf("节点 %s 的地址或端口无效", name)
	}
	query := u.Query()
	config := map[string]any{"name": name, "server": u.Hostname(), "port": port}
	switch strings.ToLower(u.Scheme) {
	case "vless":
		config["type"] = "vless"
		if u.User == nil || u.User.Username() == "" {
			return nil, false, fmt.Errorf("节点 %s 缺少 VLESS UUID", name)
		}
		config["uuid"] = u.User.Username()
		if network := query.Get("type"); network != "" {
			config["network"] = network
		}
		if security := query.Get("security"); security != "" {
			config["tls"] = security == "tls"
		}
		if sni := query.Get("sni"); sni != "" {
			config["servername"] = sni
		}
		if fp := query.Get("fp"); fp != "" {
			config["client-fingerprint"] = fp
		}
		if flow := query.Get("flow"); flow != "" {
			config["flow"] = flow
		}
		if query.Get("insecure") == "1" || strings.EqualFold(query.Get("allowInsecure"), "true") {
			config["skip-cert-verify"] = true
		}
		if pbk := query.Get("pbk"); pbk != "" {
			config["reality-opts"] = map[string]any{"public-key": pbk, "short-id": query.Get("sid")}
		}
		switch network := query.Get("type"); network {
		case "ws":
			opts := map[string]any{}
			if path := query.Get("path"); path != "" {
				opts["path"] = path
			}
			if host := query.Get("host"); host != "" {
				opts["headers"] = map[string]any{"Host": host}
			}
			config["ws-opts"] = opts
		case "grpc":
			if serviceName := query.Get("serviceName"); serviceName != "" {
				config["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
			}
		}
	case "hysteria2", "hy2":
		config["type"] = "hysteria2"
		if u.User == nil || u.User.Username() == "" {
			return nil, false, fmt.Errorf("节点 %s 缺少 Hysteria2 密码", name)
		}
		config["password"] = u.User.Username()
		if password, ok := u.User.Password(); ok {
			config["password"] = password
		}
		if sni := query.Get("sni"); sni != "" {
			config["sni"] = sni
		}
		if query.Get("insecure") == "1" || strings.EqualFold(query.Get("insecure"), "true") {
			config["skip-cert-verify"] = true
		}
		if fingerprint := query.Get("pinSHA256"); fingerprint != "" {
			config["fingerprint"] = fingerprint
		}
		if ports := query.Get("mport"); ports != "" {
			config["ports"] = ports
		}
	default:
		return nil, false, fmt.Errorf("节点 %s 的协议不受支持：%s", name, u.Scheme)
	}
	return config, true, nil
}

func isSubscriptionMetadata(name string) bool {
	for _, prefix := range []string{"剩余流量：", "距离下次重置剩余：", "套餐到期："} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func normalizeProxyConfigs(configs []map[string]any) ([]Node, error) {
	if len(configs) > maxNodes {
		return nil, fmt.Errorf("订阅最多支持 %d 个节点", maxNodes)
	}
	supported := map[string]bool{"ss": true, "ssr": true, "vmess": true, "vless": true, "trojan": true, "hysteria": true, "hysteria2": true, "tuic": true, "http": true, "socks5": true, "anytls": true}
	seen := make(map[string]bool)
	nodes := make([]Node, 0, len(configs))
	for i, cfg := range configs {
		name, _ := cfg["name"].(string)
		typ, _ := cfg["type"].(string)
		host, _ := cfg["server"].(string)
		name = strings.TrimSpace(name)
		if name == "" || len([]rune(name)) > 90 || seen[name] {
			return nil, fmt.Errorf("第 %d 个节点名称为空、重复或超过 90 字符", i+1)
		}
		if !supported[typ] {
			return nil, fmt.Errorf("节点 %s 的协议不受支持：%s", name, typ)
		}
		port, err := strconv.Atoi(fmt.Sprint(cfg["port"]))
		if host == "" || err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("节点 %s 的地址或端口无效", name)
		}
		// Chained nodes need external groups and can silently change the intended exit.
		for _, key := range []string{"dialer-proxy", "interface-name", "routing-mark"} {
			if _, ok := cfg[key]; ok {
				return nil, fmt.Errorf("节点 %s 含不支持的字段 %s", name, key)
			}
		}
		if unsafeFileReference(cfg) {
			return nil, fmt.Errorf("节点 %s 引用了本地文件；请使用内联证书", name)
		}
		cfg["name"], cfg["port"] = name, port
		seen[name] = true
		nodes = append(nodes, Node{Name: name, Type: typ, Config: cfg})
	}
	return nodes, nil
}

func unsafeFileReference(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if key == "ca" || key == "ca-file" || key == "certificate-file" || key == "private-key-file" {
				return true
			}
			if key == "certificate" || key == "private-key" {
				s, _ := item.(string)
				if !strings.Contains(s, "-----BEGIN ") {
					return true
				}
			}
			if unsafeFileReference(item) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if unsafeFileReference(item) {
				return true
			}
		}
	}
	return false
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("请输入有效的 HTTP/HTTPS 订阅链接")
	}
	if len(raw) > 4096 {
		return errors.New("订阅链接过长")
	}
	return nil
}

func publicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !(&net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}).Contains(ip)
}

func subscriptionClient() *http.Client {
	transport := &http.Transport{TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 20 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, errors.New("订阅域名解析失败")
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("订阅链接不能指向内网地址")
			}
		}
		for _, ip := range ips {
			c, e := (&net.Dialer{Timeout: 8 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return c, nil
			}
		}
		return nil, errors.New("无法连接订阅服务器")
	}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("订阅重定向次数过多")
		}
		return validateURL(req.URL.String())
	}}
}

func fetchSubscription(ctx context.Context, client *http.Client, raw string) ([]Node, error) {
	urls := strings.Fields(raw)
	if len(urls) == 0 {
		return nil, errors.New("请输入至少一个订阅链接")
	}
	if len(urls) > 8 {
		return nil, errors.New("最多支持 8 个订阅链接")
	}
	all := make([]Node, 0)
	for index, subscriptionURL := range urls {
		if err := validateURL(subscriptionURL); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, subscriptionURL, nil)
		if err != nil {
			return nil, errors.New("订阅链接无效")
		}
		req.Header.Set("User-Agent", "clash.meta/sub2api")
		res, err := client.Do(req)
		if err != nil {
			return nil, errors.New("获取订阅失败，请检查链接、网络或稍后重试")
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
		res.Body.Close()
		if readErr != nil {
			return nil, errors.New("读取订阅失败")
		}
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("订阅服务器返回 HTTP %d", res.StatusCode)
		}
		nodes, err := parseSubscription(body)
		if err != nil {
			return nil, err
		}
		if len(urls) > 1 {
			prefix := fmt.Sprintf("订阅%d · ", index+1)
			for i := range nodes {
				name := []rune(prefix + nodes[i].Name)
				if len(name) > 90 {
					name = name[:90]
				}
				nodes[i].Name = string(name)
				nodes[i].Config["name"] = nodes[i].Name
			}
		}
		all = append(all, nodes...)
		if len(all) > maxNodes {
			return nil, fmt.Errorf("合并后的订阅节点超过 %d 个", maxNodes)
		}
	}
	return all, nil
}
