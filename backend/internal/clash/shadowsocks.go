package clash

import (
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SIP002 uses percent-encoded method:password for AEAD-2022, and also
// permits Base64 userinfo for older ciphers. Preserve the password verbatim:
// a colon can separate SS-2022 identity keys, and '+' is not a space.
func parseShadowsocksURI(raw string) (map[string]any, bool, error) {
	invalid := errors.New("Shadowsocks 节点链接无效，请检查加密方式、密码、地址和端口")
	_, rest, _ := strings.Cut(raw, "://")
	var u *url.URL
	var err error
	if authority, _, _ := strings.Cut(rest, "#"); !strings.Contains(strings.SplitN(authority, "?", 2)[0], "@") {
		// Legacy links encode the entire method:password@host:port payload.
		payload, fragment, _ := strings.Cut(rest, "#")
		payload, query, _ := strings.Cut(payload, "?")
		decoded, decodeErr := decodeShadowsocksBase64(payload)
		if decodeErr != nil {
			return nil, false, invalid
		}
		at := strings.LastIndex(decoded, "@")
		if at < 0 {
			return nil, false, invalid
		}
		method, password, ok := strings.Cut(decoded[:at], ":")
		if !ok {
			return nil, false, invalid
		}
		name, decodeErr := url.PathUnescape(fragment)
		if decodeErr != nil {
			return nil, false, invalid
		}
		u = &url.URL{Scheme: "ss", Host: decoded[at+1:], User: url.UserPassword(method, password), RawQuery: query, Fragment: name}
	} else {
		// Some providers use standard Base64 (with '/') instead of Base64URL.
		// Escape that userinfo before net/url treats '/' as the start of a path.
		authority, _, _ := strings.Cut(rest, "#")
		authority, _, _ = strings.Cut(authority, "?")
		at := strings.LastIndex(authority, "@")
		if at >= 0 && !strings.Contains(authority[:at], ":") {
			userinfo, decodeErr := url.PathUnescape(authority[:at])
			if decodeErr != nil {
				return nil, false, invalid
			}
			raw = "ss://" + url.User(userinfo).String() + rest[at:]
		}
		u, err = url.Parse(raw)
		if err != nil {
			return nil, false, invalid
		}
	}
	if u.User == nil || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return nil, false, invalid
	}
	host, portText, err := net.SplitHostPort(u.Host)
	port, portErr := strconv.Atoi(portText)
	if err != nil || portErr != nil || host == "" || strings.ContainsAny(host, " /?#@\t\r\n") || port < 1 || port > 65535 {
		return nil, false, invalid
	}
	method := u.User.Username()
	password, plain := u.User.Password()
	if !plain {
		decoded, decodeErr := decodeShadowsocksBase64(method)
		if decodeErr != nil {
			return nil, false, invalid
		}
		var ok bool
		method, password, ok = strings.Cut(decoded, ":")
		if !ok {
			return nil, false, invalid
		}
	}
	if method == "" || password == "" || strings.ContainsAny(method, " :/@?#\t\r\n") || strings.ContainsAny(password, "\x00\r\n") {
		return nil, false, invalid
	}
	name := strings.TrimSpace(u.Fragment)
	if name == "" {
		name = host
	}
	if isSubscriptionMetadata(name) {
		return nil, false, nil
	}
	config := map[string]any{"name": name, "type": "ss", "server": host, "port": port, "cipher": method, "password": password}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query["plugin"]) > 1 {
		return nil, false, invalid
	}
	if plugin := query.Get("plugin"); plugin != "" {
		if err := shadowsocksPlugin(config, plugin); err != nil {
			return nil, false, err
		}
	}
	return config, true, nil
}

func decodeShadowsocksBase64(raw string) (string, error) {
	if strings.ContainsAny(raw, " \t\r\n") {
		return "", errors.New("invalid Shadowsocks Base64")
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := encoding.DecodeString(raw)
		if err == nil && utf8.Valid(decoded) {
			return string(decoded), nil
		}
	}
	return "", errors.New("invalid Shadowsocks Base64")
}

// Only translate known client plugins; silently dropping one changes the wire
// protocol. Unknown plugins/options fail without echoing credentials or URIs.
func shadowsocksPlugin(config map[string]any, raw string) error {
	invalid := errors.New("Shadowsocks 插件或参数不受支持；请使用 Clash YAML 订阅")
	parts := strings.Split(raw, ";")
	if strings.Contains(raw, "\\") {
		return invalid
	}
	options := map[string]any{}
	switch parts[0] {
	case "obfs-local", "simple-obfs", "obfs":
		config["plugin"] = "obfs"
	case "v2ray-plugin":
		config["plugin"] = "v2ray-plugin"
		options["mode"] = "websocket"
	default:
		return invalid
	}
	seen := map[string]bool{}
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		key, value, hasValue := strings.Cut(part, "=")
		if seen[key] {
			return invalid
		}
		seen[key] = true
		if config["plugin"] == "obfs" {
			switch key {
			case "obfs":
				if value != "http" && value != "tls" {
					return invalid
				}
				options["mode"] = value
			case "obfs-host":
				if value == "" {
					return invalid
				}
				options["host"] = value
			default:
				return invalid
			}
		} else {
			switch key {
			case "tls", "mux":
				if hasValue && value != "true" && value != "false" {
					return invalid
				}
				options[key] = !hasValue || value == "true"
			case "host", "path":
				if value == "" {
					return invalid
				}
				options[key] = value
			case "mode":
				if value != "websocket" {
					return invalid
				}
			default:
				return invalid
			}
		}
	}
	if config["plugin"] == "obfs" && options["mode"] == nil {
		return invalid
	}
	config["plugin-opts"] = options
	return nil
}
