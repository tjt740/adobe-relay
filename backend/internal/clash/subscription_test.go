package clash

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParseSubscription(t *testing.T) {
	body := []byte(`external-controller: 0.0.0.0:1
secret: attacker
rules: [MATCH,DIRECT]
listeners: [{name: evil, type: http, port: 1}]
proxies:
 - name: 剩余流量：997 GB
   type: vmess
   server: metadata.example.com
   port: 443
 - name: 日本
   type: vmess
   server: example.com
   port: "443"
   uuid: example
   network: ws
   ws-opts: {path: /ws}
`)
	nodes, err := parseSubscription(body)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "日本", nodes[0].Name)
	require.Equal(t, 443, nodes[0].Config["port"])
	require.NotContains(t, nodes[0].Config, "external-controller")
	for _, body := range []string{
		`proxies: []`, `proxy-providers: {remote: {url: https://example.com}}`,
		`proxies: [{name: "剩余流量：1 GB", type: ss, server: x, port: 443}]`,
		`proxies: [{name: A, type: direct, server: x, port: 1}]`,
		`proxies: [{name: A, type: ss, server: x, port: 0}]`,
		`proxies: [{name: A, type: ss, server: x, port: 1, dialer-proxy: DIRECT}]`,
		`proxies: [{name: A, type: trojan, server: x, port: 1, certificate: /etc/passwd}]`,
		`proxies: [{name: A, type: ss, server: x, port: 1}, {name: A, type: ss, server: x, port: 2}]`,
		`proxies: [[bad]]`,
	} {
		t.Run(body, func(t *testing.T) { _, err := parseSubscription([]byte(body)); require.Error(t, err) })
	}
	_, err = parseSubscription([]byte(strings.Repeat("x", maxBody+1)))
	require.Error(t, err)
}

func TestParseBase64URISubscription(t *testing.T) {
	raw := "vless://uuid@example.com:443?type=ws&security=tls&sni=example.com&host=ws.example.com&path=%2Fedge#%E5%89%A9%E4%BD%99%E6%B5%81%E9%87%8F%EF%BC%9A1%20GB\n" +
		"vless://uuid@example.com:443?type=ws&security=tls&sni=example.com&host=ws.example.com&path=%2Fedge#Japan\n" +
		"hysteria2://password@example.net:443/?insecure=false&sni=example.net&pinSHA256=abc#m\n"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(raw)))
	nodes, err := parseSubscription(body)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, "Japan", nodes[0].Name)
	require.Equal(t, "vless", nodes[0].Type)
	require.Equal(t, "/edge", nodes[0].Config["ws-opts"].(map[string]any)["path"])
	require.Equal(t, "hysteria2", nodes[1].Type)
	require.Equal(t, "password", nodes[1].Config["password"])
}

func TestParseTrojanURI(t *testing.T) {
	nodes, err := parseSubscription([]byte(base64.StdEncoding.EncodeToString([]byte("trojan://secret@example.com:443?allowInsecure=1&peer=cdn.example.com#Hong%20Kong\n"))))
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "trojan", nodes[0].Type)
	require.Equal(t, "secret", nodes[0].Config["password"])
	require.Equal(t, "cdn.example.com", nodes[0].Config["sni"])
	require.Equal(t, true, nodes[0].Config["skip-cert-verify"])
}

func TestFetchMultipleSubscriptions(t *testing.T) {
	responses := []string{
		base64.StdEncoding.EncodeToString([]byte("vless://one@example.com:443?type=ws#Japan\n")),
		base64.StdEncoding.EncodeToString([]byte("vless://two@example.com:443?type=ws#Japan\n")),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/one" {
			_, _ = w.Write([]byte(responses[0]))
			return
		}
		_, _ = w.Write([]byte(responses[1]))
	}))
	defer server.Close()
	nodes, err := fetchSubscription(context.Background(), server.Client(), server.URL+"/one\n"+server.URL+"/two")
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, "订阅1 · Japan", nodes[0].Name)
	require.Equal(t, "订阅2 · Japan", nodes[1].Name)
}

func TestPublicSubscriptionConfig(t *testing.T) {
	s := State{Nodes: []Node{
		{Name: "Japan", Enabled: true, Config: map[string]any{"name": "Japan", "type": "vless", "server": "jp.example.com", "port": 443}},
		{Name: "Disabled", Enabled: false, Config: map[string]any{"name": "Disabled", "type": "vless", "server": "off.example.com", "port": 443}},
	}}
	body, err := yaml.Marshal(publicSubscriptionConfig(s))
	require.NoError(t, err)
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
		Rules   []string         `yaml:"rules"`
	}
	require.NoError(t, yaml.Unmarshal(body, &doc))
	require.Len(t, doc.Proxies, 1)
	require.Equal(t, "Japan", doc.Proxies[0]["name"])
	require.NotContains(t, string(body), "Disabled")
	require.Equal(t, []string{"MATCH,全部节点"}, doc.Rules)
}

func TestSubscriptionURLSafety(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "https://user:secret@example.com/", "https://example.com/#token", ""} {
		require.Error(t, validateURL(raw))
	}
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "fc00::1", "224.0.0.1"} {
		require.False(t, publicIP(net.ParseIP(ip)), ip)
	}
	require.True(t, publicIP(net.ParseIP("1.1.1.1")))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("private subscription endpoint must not be reached")
	}))
	defer server.Close()
	_, err := fetchSubscription(context.Background(), subscriptionClient(), server.URL+"/private-token")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-token")
}

func TestRuntimeUsesFixedAuthenticatedEndpointsAndRejectsRetiredNodes(t *testing.T) {
	s := State{Nodes: []Node{
		{Name: "A", Type: "ss", ProxyID: 8, Port: 20001, Password: "secret", Enabled: true, Config: map[string]any{"name": "A", "type": "ss"}},
		{Name: "B", ProxyID: 9, Port: 20002, Password: "other", Enabled: false},
	}}
	cfg := runtimeConfig(s)
	proxies := cfg["proxies"].([]map[string]any)
	require.Len(t, proxies, 1)
	require.Equal(t, "sub2api-node-8", proxies[0]["name"])
	listeners := cfg["listeners"].([]map[string]any)
	require.Equal(t, "sub2api-node-8", listeners[0]["proxy"])
	require.Equal(t, "REJECT", listeners[1]["proxy"])
	require.Equal(t, []string{"MATCH,REJECT"}, cfg["rules"])
	require.NotEmpty(t, listeners[0]["users"])
	require.Equal(t, "A", s.Nodes[0].Config["name"], "generation must not mutate persisted nodes")
	m := NewManager(nil)
	s.URL = "https://example.com/private-token?secret=value"
	raw, err := json.Marshal(m.view(s))
	require.NoError(t, err)
	require.Equal(t, s.URL, m.view(s).URL, "authenticated administrators can view the saved URL")
	require.NotContains(t, string(raw), "password")
	require.NotContains(t, string(raw), "config\"")
}

func TestAdminNodeDisplayMetadata(t *testing.T) {
	m := NewManager(nil)
	m.host = "mihomo"
	n := Node{Name: "Japan", Type: "vmess", ProxyID: 7, Port: 20001, Enabled: true, Password: "listener-secret",
		Config: map[string]any{"server": "jp.example.com", "port": 443, "uuid": "outbound-secret", "network": "ws"}}
	// Persisted JSON normalizes port numbers to float64.
	persisted, err := json.Marshal(n)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(persisted, &n))
	view := m.nodeView(n)
	require.Equal(t, "jp.example.com", view.Server)
	require.Equal(t, 443, view.ServerPort)
	require.Equal(t, "mihomo", view.ProxyHost)
	require.Equal(t, 20001, view.ProxyPort)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "listener-secret")
	require.NotContains(t, string(raw), "outbound-secret")
	require.NotContains(t, string(raw), "uuid")
	n.Config, n.Enabled = nil, false
	retired := m.nodeView(n)
	require.Empty(t, retired.Server)
	require.Zero(t, retired.ServerPort)
	require.Equal(t, 20001, retired.ProxyPort)
}
