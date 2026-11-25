package protocol

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseClashYAML accepts a Clash/Mihomo proxy list and converts the protocols we
// support. Unsupported types are counted as ignored rather than failing.
func parseClashYAML(text string) ([]Parsed, []ParseError, int) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, []ParseError{{Line: 1, Reason: "YAML 解析失败"}}, 0
	}
	var out []Parsed
	var errs []ParseError
	ignored := 0
	for i, p := range doc.Proxies {
		cfg, err := clashProxyToConfig(p)
		if err != nil {
			if err == errIgnoredProxy {
				ignored++
				continue
			}
			errs = append(errs, ParseError{Line: i + 1, Reason: err.Error()})
			continue
		}
		name, _ := p["name"].(string)
		if name == "" {
			name = cfg.Host
		}
		out = append(out, Parsed{Name: name, Config: cfg})
	}
	return out, errs, ignored
}

var errIgnoredProxy = fmt.Errorf("ignored")

func clashProxyToConfig(p map[string]any) (Config, error) {
	typ := strings.ToLower(str(p["type"]))
	host := str(p["server"])
	port := intOf(p["port"])
	if host == "" || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("缺少 server/port")
	}
	tls := map[string]any{
		"sni":         str(p["sni"]),
		"insecure":    boolOf(p["skip-cert-verify"]),
		"fingerprint": str(p["client-fingerprint"]),
	}
	switch typ {
	case "hysteria2", "hy2":
		if str(p["password"]) == "" {
			return Config{}, fmt.Errorf("缺少 password")
		}
		cfg := Config{Protocol: "hy2", Host: host, Port: port,
			Auth:      map[string]any{"password": str(p["password"])},
			Transport: tls}
		if obfs := str(p["obfs"]); obfs != "" {
			cfg.Extra = map[string]any{"obfs": obfs, "obfsPassword": str(p["obfs-password"])}
		}
		return cfg, nil
	case "vless":
		uuid := str(p["uuid"])
		if !validUUID(uuid) {
			return Config{}, fmt.Errorf("UUID 格式无效")
		}
		security := "none"
		if boolOf(p["tls"]) {
			security = "tls"
		}
		if ro, ok := p["reality-opts"].(map[string]any); ok {
			security = "reality"
			tls["publicKey"] = str(ro["public-key"])
			tls["shortId"] = str(ro["short-id"])
		}
		network := str(p["network"])
		if network == "" {
			network = "tcp"
		}
		tls["security"] = security
		tls["network"] = network
		if ws, ok := p["ws-opts"].(map[string]any); ok {
			tls["wsPath"] = str(ws["path"])
			if h, ok := ws["headers"].(map[string]any); ok {
				tls["wsHost"] = str(h["Host"])
			}
		}
		if g, ok := p["grpc-opts"].(map[string]any); ok {
			tls["grpcService"] = str(g["grpc-service-name"])
		}
		auth := map[string]any{"uuid": uuid}
		if flow := str(p["flow"]); flow != "" {
			auth["flow"] = flow
		}
		return Config{Protocol: "vless", Host: host, Port: port, Auth: auth, Transport: tls}, nil
	case "tuic":
		uuid, password := str(p["uuid"]), str(p["password"])
		if !validUUID(uuid) || password == "" {
			return Config{}, fmt.Errorf("TUIC 需要 uuid 与 password")
		}
		alpn := "h3"
		if v := str(p["alpn"]); v != "" {
			alpn = v
		}
		congestion := str(p["congestion-controller"])
		if congestion == "" {
			congestion = "bbr"
		}
		return Config{Protocol: "tuic", Host: host, Port: port,
			Auth:      map[string]any{"uuid": uuid, "password": password},
			Transport: map[string]any{"sni": str(p["sni"]), "alpn": alpn, "insecure": boolOf(p["skip-cert-verify"])},
			Extra:     map[string]any{"congestionControl": congestion}}, nil
	case "anytls":
		if str(p["password"]) == "" {
			return Config{}, fmt.Errorf("AnyTLS 缺少密码")
		}
		return Config{Protocol: "anytls", Host: host, Port: port,
			Auth:      map[string]any{"password": str(p["password"])},
			Transport: tls}, nil
	case "mieru":
		user, pass := str(p["username"]), str(p["password"])
		if user == "" || pass == "" {
			return Config{}, fmt.Errorf("Mieru 需要用户名与密码")
		}
		return Config{Protocol: "mieru", Host: host, Port: port,
			Auth:      map[string]any{"username": user, "password": pass},
			Transport: tls}, nil
	default:
		return Config{}, errIgnoredProxy
	}
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func intOf(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	}
	return 0
}

func boolOf(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case int:
		return t == 1
	}
	return false
}
