package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func parseLegacyShare(u *url.URL, scheme string) (Config, error) {
	if scheme == "vmess" {
		return parseVMess(u)
	}
	host, port, err := splitHostPort(u)
	if err != nil {
		return Config{}, err
	}
	q := u.Query()
	tls := map[string]any{"sni": firstNonEmpty(q.Get("sni"), q.Get("peer")), "insecure": qBool(q, "allowInsecure") || qBool(q, "insecure") || qBool(q, "allow_insecure"), "fingerprint": firstNonEmpty(q.Get("fp"), q.Get("client-fingerprint"))}
	cfg := Config{Host: host, Port: port, Transport: tls, Extra: map[string]any{}}
	switch scheme {
	case "ss":
		if u.User == nil {
			return Config{}, fmt.Errorf("Shadowsocks 缺少认证信息")
		}
		raw := u.User.String()
		decoded, e := base64.RawURLEncoding.DecodeString(strings.TrimRight(raw, "="))
		if e != nil {
			decoded, e = base64.RawStdEncoding.DecodeString(raw)
		}
		if e == nil {
			raw = string(decoded)
		} else if x, ue := url.PathUnescape(raw); ue == nil {
			raw = x
		}
		cipher, password, ok := strings.Cut(raw, ":")
		if !ok || cipher == "" || password == "" {
			return Config{}, fmt.Errorf("Shadowsocks 认证信息格式无效")
		}
		cfg.Protocol = "ss"
		cfg.Auth = map[string]any{"cipher": cipher, "password": password}
		if plugin := q.Get("plugin"); plugin != "" {
			cfg.Extra["plugin"] = plugin
		}
	case "trojan":
		if u.User == nil {
			return Config{}, fmt.Errorf("Trojan 缺少密码")
		}
		password, _ := u.User.Password()
		if password == "" {
			password = u.User.Username()
		}
		if password == "" {
			return Config{}, fmt.Errorf("Trojan 缺少密码")
		}
		cfg.Protocol = "trojan"
		cfg.Auth = map[string]any{"password": password}
		cfg.Transport["security"] = "tls"
		cfg.Transport["network"] = firstNonEmpty(q.Get("type"), q.Get("network"), "tcp")
		cfg.Transport["wsPath"] = q.Get("path")
		cfg.Transport["wsHost"] = q.Get("host")
		cfg.Transport["grpcService"] = firstNonEmpty(q.Get("serviceName"), q.Get("service_name"))
	default:
		return Config{}, fmt.Errorf("不支持的协议")
	}
	return cfg, nil
}

func parseVMess(u *url.URL) (Config, error) {
	raw := u.Opaque
	if raw == "" {
		raw = strings.TrimPrefix(u.Host+u.Path, "/")
	}
	payload, e := base64.RawURLEncoding.DecodeString(strings.TrimRight(raw, "="))
	if e != nil {
		payload, e = base64.RawStdEncoding.DecodeString(strings.TrimRight(raw, "="))
	}
	if e != nil {
		return Config{}, fmt.Errorf("VMess 链接编码无效")
	}
	var v struct {
		Add           string          `json:"add"`
		Port          json.RawMessage `json:"port"`
		ID            string          `json:"id"`
		Aid           json.RawMessage `json:"aid"`
		Net           string          `json:"net"`
		Type          string          `json:"type"`
		Host          string          `json:"host"`
		Path          string          `json:"path"`
		TLS           string          `json:"tls"`
		SNI           string          `json:"sni"`
		Fp            string          `json:"fp"`
		AllowInsecure json.RawMessage `json:"allowInsecure"`
		Scy           string          `json:"scy"`
		Ps            string          `json:"ps"`
	}
	if json.Unmarshal(payload, &v) != nil || v.Add == "" || v.ID == "" {
		return Config{}, fmt.Errorf("VMess 配置缺少必要字段")
	}
	var p int
	_ = json.Unmarshal(v.Port, &p)
	if p == 0 {
		var s string
		_ = json.Unmarshal(v.Port, &s)
		p, _ = strconv.Atoi(s)
	}
	if p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("端口无效")
	}
	aid := 0
	_ = json.Unmarshal(v.Aid, &aid)
	if aid == 0 {
		var s string
		_ = json.Unmarshal(v.Aid, &s)
		aid, _ = strconv.Atoi(s)
	}
	insecure := false
	if len(v.AllowInsecure) > 0 {
		var b bool
		var n int
		var s string
		if json.Unmarshal(v.AllowInsecure, &b) == nil {
			insecure = b
		} else if json.Unmarshal(v.AllowInsecure, &n) == nil {
			insecure = n != 0
		} else if json.Unmarshal(v.AllowInsecure, &s) == nil {
			insecure = s == "1" || strings.EqualFold(s, "true")
		}
	}
	return Config{
		Protocol: "vmess",
		Host:     v.Add,
		Port:     p,
		Auth:     map[string]any{"uuid": v.ID, "alterId": aid, "cipher": firstNonEmpty(v.Scy, "auto")},
		Transport: map[string]any{
			"security":    strings.ToLower(v.TLS),
			"sni":         firstNonEmpty(v.SNI, v.Host),
			"network":     firstNonEmpty(v.Net, "tcp"),
			"wsHost":      v.Host,
			"wsPath":      v.Path,
			"grpcService": v.Path,
			"wsType":      v.Type,
			"fingerprint": v.Fp,
			"insecure":    insecure,
		},
		Extra: map[string]any{"name": v.Ps},
	}, nil
}
