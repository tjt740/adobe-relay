package clash

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestShadowsocksSubscriptionEncodings(t *testing.T) {
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	password := "密钥:+/@%?=#"
	for outerIndex, outer := range encodings {
		for innerIndex, inner := range encodings {
			t.Run(fmt.Sprintf("%d-%d", outerIndex, innerIndex), func(t *testing.T) {
				userinfo := inner.EncodeToString([]byte("aes-256-gcm:" + password))
				raw := "ss://" + userinfo + "@example.com:443/#%E6%97%A5%E6%9C%AC+SS"
				nodes, err := parseSubscription([]byte(outer.EncodeToString([]byte(raw))))
				require.NoError(t, err)
				require.Len(t, nodes, 1)
				require.Equal(t, "日本+SS", nodes[0].Name)
				require.Equal(t, "ss", nodes[0].Type)
				require.Equal(t, password, nodes[0].Config["password"])
				require.Equal(t, "aes-256-gcm", nodes[0].Config["cipher"])
			})
		}
	}
}

func TestShadowsocks2022PreservesIdentityKeys(t *testing.T) {
	for _, cipher := range []string{"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305"} {
		t.Run(cipher, func(t *testing.T) {
			size := 32
			if cipher == "2022-blake3-aes-128-gcm" {
				size = 16
			}
			key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xfb", size)))
			for _, password := range []string{key, key + ":" + key} {
				u := url.URL{Scheme: "ss", User: url.UserPassword(cipher, password), Host: "[2001:db8::1]:8443", Fragment: "新加坡"}
				nodes, err := parseSubscription([]byte(base64.StdEncoding.EncodeToString([]byte(u.String()))))
				require.NoError(t, err)
				require.Len(t, nodes, 1)
				require.Equal(t, "2001:db8::1", nodes[0].Config["server"])
				require.Equal(t, 8443, nodes[0].Config["port"])
				require.Equal(t, cipher, nodes[0].Config["cipher"])
				require.Equal(t, password, nodes[0].Config["password"])
				nodes[0].Enabled, nodes[0].ProxyID = true, 10
				cfg := runtimeConfig(State{Nodes: nodes})
				out := cfg["proxies"].([]map[string]any)[0]
				require.Equal(t, password, out["password"])
				require.Equal(t, cipher, out["cipher"])
			}
		})
	}
}

func TestShadowsocksLegacyAndPlain(t *testing.T) {
	password := "secret:a@b+/%"
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		for _, host := range []string{"example.com:443", "[2001:db8::2]:443"} {
			raw := "ss://" + encoding.EncodeToString([]byte("aes-128-gcm:"+password+"@"+host)) + "#Legacy%20SS"
			cfg, ok, err := parseProxyURI(raw)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, password, cfg["password"])
			require.Equal(t, "Legacy SS", cfg["name"])
		}
	}
	cfg, ok, err := parseProxyURI("SS://aes-128-gcm:a+b%2F%252F%3A@example.com:443")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "a+b/%2F:", cfg["password"])
	require.Equal(t, "example.com", cfg["name"])
}

func TestShadowsocksPluginTranslation(t *testing.T) {
	for _, tc := range []struct {
		plugin string
		name   string
		opts   map[string]any
	}{
		{"obfs-local;obfs=tls;obfs-host=cdn.example.com", "obfs", map[string]any{"mode": "tls", "host": "cdn.example.com"}},
		{"simple-obfs;obfs=http", "obfs", map[string]any{"mode": "http"}},
		{"v2ray-plugin;tls;host=cdn.example.com;path=/ws?a=b;mode=websocket;mux=false", "v2ray-plugin", map[string]any{"mode": "websocket", "tls": true, "host": "cdn.example.com", "path": "/ws?a=b", "mux": false}},
	} {
		cfg, ok, err := parseProxyURI("ss://aes-128-gcm:secret@example.com:443/?plugin=" + url.QueryEscape(tc.plugin))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, tc.name, cfg["plugin"])
		require.Equal(t, tc.opts, cfg["plugin-opts"])
	}
}

func TestShadowsocksRejectsMalformedWithoutCredentials(t *testing.T) {
	for _, raw := range []string{
		"ss://", "ss://!!!", "ss://missing@example.com:443", "ss://" + base64.StdEncoding.EncodeToString([]byte("no-separator")) + "@example.com:443",
		"ss://aes-128-gcm:PRIVATE@example.com", "ss://aes-128-gcm:PRIVATE@:443", "ss://aes-128-gcm:PRIVATE@example.com:0",
		"ss://aes-128-gcm:PRIVATE@example.com:65536", "ss://aes-128-gcm:PRIVATE@example.com:abc", "ss://aes-128-gcm:PRIVATE@example.com:443/path",
		"ss://aes-128-gcm:PRIVATE%xx@example.com:443", "ss://:PRIVATE@example.com:443", "ss://aes-128-gcm:@example.com:443",
		"ss://aes-128-gcm:PRIVATE%0A@example.com:443", "ss://aes-128-gcm:PRIVATE@example.com:443?plugin=unknown",
		"ss://aes-128-gcm:PRIVATE@example.com:443?plugin=v2ray-plugin;server", "ss://aes-128-gcm:PRIVATE@example.com:443?plugin=%xx",
		"ss://aes-128-gcm:PRIVATE@example.com:443?plugin=obfs&plugin=obfs", "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:PRIVATE@host:443/path")),
	} {
		cfg, ok, err := parseProxyURI(raw)
		require.Error(t, err, "malformed fixture must fail")
		require.Nil(t, cfg)
		require.False(t, ok)
		require.NotContains(t, err.Error(), "PRIVATE")
		require.NotContains(t, err.Error(), raw)
	}
	for _, plugin := range []string{"obfs-local", "obfs-local;obfs=quic", "obfs-local;obfs=tls;obfs=tls", "obfs-local;obfs=tls;file=PRIVATE", "v2ray-plugin;server", "v2ray-plugin;mode=quic", "v2ray-plugin;tls=maybe", "v2ray-plugin;path=", `v2ray-plugin;host=a\;PRIVATE`} {
		_, _, err := parseProxyURI("ss://aes-128-gcm:PRIVATE@example.com:443/?plugin=" + url.QueryEscape(plugin))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "PRIVATE")
	}
}

func TestShadowsocksMixedSubscriptionAndDuplicateRules(t *testing.T) {
	raw := "ss://aes-128-gcm:secret@example.com:443#剩余流量：1GB\nss://aes-128-gcm:secret@example.com:443#SS\nvless://uuid@example.net:443#VLESS\ntrojan://secret@example.org:443#Trojan\nhy2://secret@example.edu:443#HY2\n"
	nodes, err := parseSubscription([]byte(base64.StdEncoding.EncodeToString([]byte(raw))))
	require.NoError(t, err)
	require.Len(t, nodes, 4)
	_, err = parseSubscription([]byte(base64.StdEncoding.EncodeToString([]byte(raw + "ss://aes-128-gcm:secret@example.com:444#SS"))))
	require.ErrorContains(t, err, "重复")
}

func TestShadowsocksPublicPreviewFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/shadowsocks-base64.txt")
	require.NoError(t, err)
	nodes, err := parseSubscription(body)
	require.NoError(t, err)
	require.Len(t, nodes, 3)
	for _, n := range nodes {
		require.Equal(t, "ss", n.Type)
		require.Equal(t, "example.invalid", n.Config["server"])
		require.True(t, strings.HasPrefix(n.Config["cipher"].(string), "2022-blake3-"))
	}
}

// Optional private fixture: never log node names, servers, credentials or URLs.
// Re-encode the provider's SS YAML nodes as a Base64 URI subscription so the
// same real fields exercise both import paths without touching production.
func TestShadowsocksPrivateSubscription(t *testing.T) {
	path := os.Getenv("CLASH_SS_PRIVATE_SUBSCRIPTION_FILE")
	if path == "" {
		t.Skip("private subscription fixture not configured")
	}
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	nodes, err := parseSubscription(body)
	if err != nil {
		t.Fatal("private subscription parsing failed")
	}
	var lines []string
	var expected []Node
	for _, n := range nodes {
		if n.Type != "ss" {
			continue
		}
		u := url.URL{Scheme: "ss", User: url.UserPassword(n.Config["cipher"].(string), n.Config["password"].(string)), Host: fmt.Sprintf("%s:%v", n.Config["server"], n.Config["port"]), Fragment: fmt.Sprintf("private-%d", len(lines))}
		lines = append(lines, u.String())
		expected = append(expected, n)
	}
	if len(lines) == 0 {
		t.Logf("private fixture parsed: total=%d ss=0; SS conversion not applicable", len(nodes))
		return
	}
	parsed, err := parseSubscription([]byte(base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))))
	if err != nil || len(parsed) != len(expected) {
		t.Fatal("private Base64 conversion parsing failed")
	}
	counts := map[string]int{}
	for i, n := range parsed {
		for _, key := range []string{"server", "port", "cipher", "password"} {
			if fmt.Sprint(n.Config[key]) != fmt.Sprint(expected[i].Config[key]) {
				t.Fatal("private node field changed")
			}
		}
		counts[fmt.Sprint(n.Config["cipher"])]++
	}
	t.Logf("private fixture: total=%d ss=%d ciphers=%v", len(nodes), len(parsed), counts)
	if output := os.Getenv("CLASH_SS_PRIVATE_OUTPUT"); output != "" {
		configs := make([]map[string]any, len(parsed))
		for i, n := range parsed {
			configs[i] = n.Config
		}
		encoded, err := yaml.Marshal(map[string]any{"proxies": configs})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(output, encoded, 0600))
	}
}
