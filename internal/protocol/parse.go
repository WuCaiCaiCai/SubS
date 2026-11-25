// Package protocol parses share links into the unified node config
// representation described in 协议解析与配置生成规范.md.
package protocol

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Config is the unified internal node representation persisted in node.config.
type Config struct {
	Protocol  string         `json:"protocol"`
	Host      string         `json:"host"`
	Port      int            `json:"port"`
	Auth      map[string]any `json:"auth"`
	Transport map[string]any `json:"transport"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// Result carries a parsed config plus the display name carried by the URI fragment.
type Result struct {
	Name   string
	Config Config
}

// ParseLine parses a single share link. Unknown schemes return an error that
// never embeds credentials, so it is safe to surface in the UI.
func ParseLine(line string) (Result, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Result{}, fmt.Errorf("空行")
	}
	scheme, rest, ok := strings.Cut(line, "://")
	if !ok {
		return Result{}, fmt.Errorf("不是有效的分享链接")
	}
	u, err := url.Parse(line)
	if err != nil {
		return Result{}, fmt.Errorf("链接格式错误")
	}
	name := ""
	if u.Fragment != "" {
		if decoded, e := url.PathUnescape(u.Fragment); e == nil {
			name = decoded
		}
	}
	var cfg Config
	switch strings.ToLower(scheme) {
	case "hy2", "hysteria2":
		cfg, err = parseHy2(u)
	case "vless":
		cfg, err = parseVLESS(u)
	case "tuic":
		cfg, err = parseTUIC(u)
	case "anytls":
		cfg, err = parseAnyTLS(u)
	case "mieru":
		cfg, err = parseMieru(u)
	default:
		_ = rest
		return Result{}, fmt.Errorf("不支持的协议: %s", scheme)
	}
	if err != nil {
		return Result{}, err
	}
	if name == "" {
		name = cfg.Host
	}
	return Result{Name: name, Config: cfg}, nil
}

func splitHostPort(u *url.URL) (string, int, error) {
	host := u.Hostname()
	portStr := u.Port()
	if host == "" {
		return "", 0, fmt.Errorf("缺少服务器地址")
	}
	if portStr == "" {
		return "", 0, fmt.Errorf("缺少端口")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("端口无效")
	}
	return host, port, nil
}

func qBool(q url.Values, key string) bool {
	v := strings.ToLower(q.Get(key))
	return v == "1" || v == "true"
}

func parseHy2(u *url.URL) (Config, error) {
	host, port, err := splitHostPort(u)
	if err != nil {
		return Config{}, err
	}
	password := ""
	if u.User != nil {
		password, _ = url.PathUnescape(u.User.Username())
		if p, ok := u.User.Password(); ok {
			password = p
		}
	}
	if password == "" {
		return Config{}, fmt.Errorf("缺少密码")
	}
	q := u.Query()
	cfg := Config{
		Protocol:  "hy2",
		Host:      host,
		Port:      port,
		Auth:      map[string]any{"password": password},
		Transport: map[string]any{"sni": q.Get("sni"), "insecure": qBool(q, "insecure") || qBool(q, "allowInsecure")},
	}
	if obfs := q.Get("obfs"); obfs != "" {
		cfg.Extra = map[string]any{"obfs": obfs, "obfsPassword": q.Get("obfs-password")}
	}
	return cfg, nil
}

func parseVLESS(u *url.URL) (Config, error) {
	host, port, err := splitHostPort(u)
	if err != nil {
		return Config{}, err
	}
	uuid := ""
	if u.User != nil {
		uuid = u.User.Username()
	}
	if !validUUID(uuid) {
		return Config{}, fmt.Errorf("UUID 格式无效")
	}
	q := u.Query()
	security := strings.ToLower(q.Get("security"))
	if security == "" {
		security = "none"
	}
	network := strings.ToLower(q.Get("type"))
	if network == "" {
		network = "tcp"
	}
	transport := map[string]any{
		"sni":         q.Get("sni"),
		"security":    security,
		"fingerprint": q.Get("fp"),
		"network":     network,
		"insecure":    qBool(q, "allowInsecure") || qBool(q, "insecure"),
	}
	if security == "reality" {
		pbk, sid := q.Get("pbk"), q.Get("sid")
		if pbk == "" || sid == "" {
			return Config{}, fmt.Errorf("Reality 缺少 pbk/sid")
		}
		transport["publicKey"] = pbk
		transport["shortId"] = sid
	}
	switch network {
	case "ws":
		if q.Get("path") == "" {
			return Config{}, fmt.Errorf("ws 传输缺少 path")
		}
		transport["wsPath"] = q.Get("path")
		transport["wsHost"] = q.Get("host")
	case "grpc":
		transport["grpcService"] = q.Get("serviceName")
	}
	auth := map[string]any{"uuid": uuid}
	if flow := q.Get("flow"); flow != "" {
		auth["flow"] = flow
	}
	return Config{Protocol: "vless", Host: host, Port: port, Auth: auth, Transport: transport}, nil
}

func parseTUIC(u *url.URL) (Config, error) {
	host, port, err := splitHostPort(u)
	if err != nil {
		return Config{}, err
	}
	uuid, password := "", ""
	if u.User != nil {
		uuid = u.User.Username()
		password, _ = u.User.Password()
	}
	if !validUUID(uuid) || password == "" {
		return Config{}, fmt.Errorf("TUIC 需要 uuid 与 password")
	}
	q := u.Query()
	alpn := q.Get("alpn")
	if alpn == "" {
		alpn = "h3"
	}
	congestion := q.Get("congestion_control")
	if congestion == "" {
		congestion = "bbr"
	}
	return Config{
		Protocol: "tuic", Host: host, Port: port,
		Auth:      map[string]any{"uuid": uuid, "password": password},
		Transport: map[string]any{"sni": q.Get("sni"), "alpn": alpn, "insecure": qBool(q, "allow_insecure") || qBool(q, "insecure")},
		Extra:     map[string]any{"congestionControl": congestion},
	}, nil
}

func parseAnyTLS(u *url.URL) (Config, error) {
	host, port, err := splitHostPort(u)
	if err != nil {
		return Config{}, err
	}
	password := ""
	if u.User != nil {
		password = u.User.Username()
		if p, ok := u.User.Password(); ok && p != "" {
			password = p
		}
	}
	if password == "" {
		return Config{}, fmt.Errorf("AnyTLS 缺少密码")
	}
	q := u.Query()
	return Config{
		Protocol: "anytls", Host: host, Port: port,
		Auth:      map[string]any{"password": password},
		Transport: map[string]any{"sni": q.Get("sni"), "insecure": qBool(q, "insecure"), "fingerprint": q.Get("client-fingerprint")},
	}, nil
}

func parseMieru(u *url.URL) (Config, error) {
	host, port, err := splitHostPort(u)
	if err != nil {
		return Config{}, err
	}
	username, password := "", ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	if username == "" || password == "" {
		return Config{}, fmt.Errorf("Mieru 需要用户名与密码")
	}
	q := u.Query()
	return Config{
		Protocol: "mieru", Host: host, Port: port,
		Auth:      map[string]any{"username": username, "password": password},
		Transport: map[string]any{"sni": q.Get("sni"), "insecure": qBool(q, "insecure")},
	}, nil
}

func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// ParseSubscription decodes an airport payload: base64 URI list, plain URI list,
// or a Clash YAML proxy list. Parse failures are collected with line numbers and
// never abort the whole import.
type ParseError struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type Parsed struct {
	Name   string `json:"name"`
	Config Config `json:"config"`
}

func ParseSubscription(body []byte) ([]Parsed, []ParseError, int) {
	text := string(body)
	if decoded, ok := decodeBase64List(text); ok {
		text = decoded
	}
	if looksLikeYAML(text) {
		return parseClashYAML(text)
	}
	var out []Parsed
	var errs []ParseError
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		res, err := ParseLine(line)
		if err != nil {
			errs = append(errs, ParseError{Line: i + 1, Reason: err.Error()})
			continue
		}
		out = append(out, Parsed{Name: res.Name, Config: res.Config})
	}
	return out, errs, 0
}

func decodeBase64List(text string) (string, bool) {
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, text)
	if compact == "" {
		return "", false
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(compact); err == nil && strings.Contains(string(b), "://") {
			return string(b), true
		}
	}
	return "", false
}

func looksLikeYAML(text string) bool {
	return strings.Contains(text, "proxies:") || strings.HasPrefix(strings.TrimSpace(text), "- name:")
}

// Fingerprint identifies a node across airport refreshes.
func Fingerprint(c Config) string {
	auth, _ := json.Marshal(c.Auth)
	parts := []string{strings.ToLower(c.Protocol), strings.ToLower(strings.TrimSpace(c.Host)), strconv.Itoa(c.Port), string(auth)}
	return hashParts(parts)
}

func hashParts(parts []string) string {
	b, _ := json.Marshal(parts)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
