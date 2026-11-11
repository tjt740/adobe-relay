package clash

import (
	"bytes"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Opt-in real TCP round trips using two disposable Mihomo processes on loopback.
// No production controller, subscription credentials or external server is used.
func TestShadowsocksMihomoTransport(t *testing.T) {
	binary := os.Getenv("CLASH_TEST_MIHOMO_BINARY")
	if binary == "" {
		t.Skip("set CLASH_TEST_MIHOMO_BINARY for isolated transport verification")
	}
	freePort := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := l.Addr().(*net.TCPAddr).Port
		require.NoError(t, l.Close())
		return port
	}
	start := func(t *testing.T, config map[string]any, port int) *exec.Cmd {
		t.Helper()
		dir := t.TempDir()
		data, err := yaml.Marshal(config)
		require.NoError(t, err)
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(path, data, 0600))
		cmd := exec.Command(binary, "-d", dir, "-f", path)
		var log bytes.Buffer
		cmd.Stdout, cmd.Stderr = &log, &log
		require.NoError(t, cmd.Start())
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		require.Eventually(t, func() bool {
			c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 100*time.Millisecond)
			if err != nil {
				return false
			}
			_ = c.Close()
			return true
		}, 10*time.Second, 50*time.Millisecond, "isolated Mihomo listener did not start")
		return cmd
	}
	for _, cipher := range []string{"aes-128-gcm", "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305"} {
		t.Run(cipher, func(t *testing.T) {
			keySize := 32
			if cipher == "2022-blake3-aes-128-gcm" {
				keySize = 16
			}
			key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, keySize))
			serverPort, clientPort := freePort(), freePort()
			server := start(t, map[string]any{"mode": "rule", "log-level": "silent", "listeners": []map[string]any{{"name": "ss-in", "type": "shadowsocks", "listen": "127.0.0.1", "port": serverPort, "cipher": cipher, "password": key}}, "rules": []string{"MATCH,DIRECT"}}, serverPort)
			u := url.URL{Scheme: "ss", User: url.UserPassword(cipher, key), Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(serverPort)), Fragment: "roundtrip"}
			nodes, err := parseSubscription([]byte(base64.StdEncoding.EncodeToString([]byte(u.String()))))
			require.NoError(t, err)
			nodes[0].Enabled, nodes[0].ProxyID, nodes[0].Port, nodes[0].Password = true, 1, clientPort, "test-listener"
			config := runtimeConfig(State{Nodes: nodes})
			config["listeners"].([]map[string]any)[0]["listen"] = "127.0.0.1"
			config["log-level"] = "silent"
			start(t, config, clientPort)
			const marker = "ss2022-isolated-roundtrip"
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, marker) }))
			defer target.Close()
			proxyURL := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(clientPort)), User: url.UserPassword("clash", "test-listener")}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			response, err := client.Get(target.URL)
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.Equal(t, marker, string(body))
			// A second request must fail after removing the SS hop: no direct fallback.
			require.NoError(t, server.Process.Kill())
			_ = server.Wait()
			response, err = client.Get(target.URL)
			if err == nil {
				defer response.Body.Close()
				require.NotEqual(t, http.StatusOK, response.StatusCode)
			}
		})
	}
}
