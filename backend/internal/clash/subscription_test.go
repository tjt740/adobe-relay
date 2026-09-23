package clash

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSubscription(t *testing.T) {
	body := []byte(`external-controller: 0.0.0.0:1
secret: attacker
rules: [MATCH,DIRECT]
listeners: [{name: evil, type: http, port: 1}]
proxies:
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
