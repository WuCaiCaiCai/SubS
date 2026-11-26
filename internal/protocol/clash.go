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
	portRange := str(p["port-range"])
	if host == "" {
		return Config{}, fmt.Errorf("缺少 server/port")
	}
	// mieru 允许只写 port-range，其余协议必须给出单端口。
	if typ != "mieru" && (port < 1 || port > 65535) {
		return Config{}, fmt.Errorf("缺少 server/port")
	}
	if typ == "mieru" && (port < 1 || port > 65535) && strings.TrimSpace(portRange) == "" {
		return Config{}, fmt.Errorf("缺少 server/port")
	}
	tls := map[string]any{
		"sni":         firstNonEmpty(str(p["servername"]), str(p["sni"])),
		"insecure":    boolOf(p["skip-cert-verify"]),
		"fingerprint": str(p["client-fingerprint"]),
		// mihomo 的 udp 开 UDP relay；订阅生态几乎都显式写 true，
		// 字段缺失时默认开启，与参考配置行为一致。
		"udp": udpOf(p),
	}
	switch typ {
	case "ss", "shadowsocks":
		method, password := str(p["cipher"]), str(p["password"])
		if method == "" || password == "" {
			return Config{}, fmt.Errorf("Shadowsocks 缺少 cipher/password")
		}
		return Config{Protocol: "ss", Host: host, Port: port, Auth: map[string]any{"cipher": method, "password": password}, Transport: tls, Extra: map[string]any{}}, nil
	case "vmess":
		uuid := str(p["uuid"])
		if !validUUID(uuid) {
			return Config{}, fmt.Errorf("UUID 格式无效")
		}
		network := firstNonEmpty(str(p["network"]), "tcp")
		ws, _ := p["ws-opts"].(map[string]any)
		grpc, _ := p["grpc-opts"].(map[string]any)
		security := "none"
		if boolOf(p["tls"]) {
			security = "tls"
		}
		tls["security"] = security
		tls["network"] = network
		if ws != nil {
			tls["wsPath"] = str(ws["path"])
			if h, ok := ws["headers"].(map[string]any); ok {
				tls["wsHost"] = str(h["Host"])
			}
		}
		if grpc != nil {
			tls["grpcService"] = str(grpc["grpc-service-name"])
		}
		return Config{Protocol: "vmess", Host: host, Port: port, Auth: map[string]any{"uuid": uuid, "alterId": intOf(p["alterId"]), "cipher": firstNonEmpty(str(p["cipher"]), "auto")}, Transport: tls, Extra: map[string]any{}}, nil
	case "trojan":
		password := str(p["password"])
		if password == "" {
			return Config{}, fmt.Errorf("Trojan 缺少 password")
		}
		tls["security"] = "tls"
		return Config{Protocol: "trojan", Host: host, Port: port, Auth: map[string]any{"password": password}, Transport: tls, Extra: map[string]any{}}, nil
	case "hysteria2", "hy2":
		if str(p["password"]) == "" {
			return Config{}, fmt.Errorf("缺少 password")
		}
		// hysteria2 走 QUIC，alpn 默认 h3；参考配置会显式写 alpn: [h3]。
		tls["alpn"] = firstNonEmpty(alpnOf(p), "h3")
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
		// TUIC v5 是 mihomo 唯一广泛支持的版本；旧 YAML 未写时默认 5。
		version := 5
		if v := intOf(p["version"]); v == 4 || v == 5 {
			version = v
		}
		return Config{Protocol: "tuic", Host: host, Port: port,
			Auth:      map[string]any{"uuid": uuid, "password": password},
			Transport: map[string]any{"sni": firstNonEmpty(str(p["servername"]), str(p["sni"])), "alpn": alpn, "insecure": boolOf(p["skip-cert-verify"]), "udp": udpOf(p)},
			Extra:     map[string]any{"congestionControl": congestion, "tuicVersion": version}}, nil
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
		cfg := Config{Protocol: "mieru", Host: host, Port: port,
			Auth:      map[string]any{"username": user, "password": pass},
			Transport: tls,
			Extra:     map[string]any{"transport": strings.ToUpper(firstNonEmpty(str(p["transport"]), "TCP"))}}
		if strings.TrimSpace(portRange) != "" {
			lo, hi, err := parsePortRange(portRange)
			if err != nil {
				return Config{}, err
			}
			cfg.Extra["portRange"] = fmt.Sprintf("%d-%d", lo, hi)
			cfg.Port = 0
		}
		if v := str(p["multiplexing"]); v != "" {
			cfg.Extra["multiplexing"] = normaliseMultiplexing(strings.ToUpper(v))
		}
		if v := str(p["handshake-mode"]); v != "" {
			cfg.Extra["handshakeMode"] = normaliseHandshake(strings.ToUpper(v))
		}
		if v := str(p["traffic-pattern"]); v != "" {
			cfg.Extra["trafficPattern"] = v
		}
		return cfg, nil
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

// udpOf 读 mihomo 的 udp 字段；QUIC 协议（hy2/tuic）天然 UDP，
// 字段缺失时与多数订阅一致地开启。
func udpOf(p map[string]any) bool {
	if v, ok := p["udp"]; ok {
		return boolOf(v)
	}
	switch strings.ToLower(str(p["type"])) {
	case "hysteria2", "hy2", "tuic":
		return true
	}
	return false
}

// alpnOf 把 mihomo 的 alpn 列表压回单个字符串（多值时取第一个）。
func alpnOf(p map[string]any) string {
	switch t := p["alpn"].(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			return str(t[0])
		}
	case []string:
		if len(t) > 0 {
			return t[0]
		}
	}
	return ""
}
