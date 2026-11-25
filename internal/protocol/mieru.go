package protocol

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Mieru nodes have no TLS layer and may bind a port *range* instead of a single
// port. Extra["portRange"] carries "start-end" when Port is 0.
//
// Supported URI form (本项目定义，同时兼容常见第三方变体):
//
//	mieru://user:pass@host:2090-2099?transport=TCP&multiplexing=MULTIPLEXING_LOW&handshake-mode=HANDSHAKE_STANDARD&traffic-pattern=...&port-range=2090-2099&sni=...#name
//
// Credentials may also be supplied as ?username=&password= for clients that
// cannot put them in the userinfo section.
func parseMieruShareLink(raw string) (Config, error) {
	body := raw
	if i := strings.Index(body, "://"); i >= 0 {
		body = body[i+3:]
	}
	fragment := ""
	if i := strings.Index(body, "#"); i >= 0 {
		fragment = body[i+1:]
		body = body[:i]
	}
	_ = fragment
	query := ""
	if i := strings.Index(body, "?"); i >= 0 {
		query = body[i+1:]
		body = body[:i]
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return Config{}, fmt.Errorf("Mieru 链接参数格式错误")
	}

	// userinfo may itself contain '@' inside an encoded password; split at the
	// last '@' that appears before the first '/'.
	userinfo := ""
	if i := strings.LastIndex(body, "@"); i >= 0 {
		userinfo = body[:i]
		body = body[i+1:]
	}
	username, password := "", ""
	if userinfo != "" {
		username, password, _ = strings.Cut(userinfo, ":")
		if decoded, e := url.PathUnescape(username); e == nil {
			username = decoded
		}
		if decoded, e := url.PathUnescape(password); e == nil {
			password = decoded
		}
	}
	if u := strings.TrimSpace(q.Get("username")); u != "" {
		username = u
	}
	if p := q.Get("password"); p != "" {
		password = p
	}
	if username == "" || password == "" {
		return Config{}, fmt.Errorf("Mieru 需要用户名与密码")
	}

	host, portSpec := splitMieruHostPort(body)
	if host == "" {
		return Config{}, fmt.Errorf("缺少服务器地址")
	}
	if portSpec == "" {
		portSpec = firstNonEmpty(q.Get("port-range"), q.Get("portRange"), q.Get("port"))
	}
	portRange := ""
	port := 0
	if portSpec != "" {
		if strings.Contains(portSpec, "-") {
			lo, hi, err := parsePortRange(portSpec)
			if err != nil {
				return Config{}, err
			}
			portRange = fmt.Sprintf("%d-%d", lo, hi)
		} else {
			p, err := strconv.Atoi(portSpec)
			if err != nil || p < 1 || p > 65535 {
				return Config{}, fmt.Errorf("端口无效")
			}
			port = p
		}
	}
	if pr := strings.TrimSpace(firstNonEmpty(q.Get("port-range"), q.Get("portRange"))); pr != "" {
		// An explicit port-range wins over the authority section.
		lo, hi, err := parsePortRange(pr)
		if err != nil {
			return Config{}, err
		}
		portRange = fmt.Sprintf("%d-%d", lo, hi)
		port = 0
	}
	if port == 0 && portRange == "" {
		return Config{}, fmt.Errorf("缺少端口")
	}

	transport := strings.ToUpper(strings.TrimSpace(firstNonEmpty(
		q.Get("transport"), q.Get("protocol"), "TCP")))
	if transport != "TCP" && transport != "UDP" {
		return Config{}, fmt.Errorf("Mieru transport 仅支持 TCP/UDP")
	}

	extra := map[string]any{"transport": transport}
	if portRange != "" {
		extra["portRange"] = portRange
	}
	if v := strings.ToUpper(strings.TrimSpace(firstNonEmpty(q.Get("multiplexing"), q.Get("multiplexingLevel")))); v != "" {
		extra["multiplexing"] = normaliseMultiplexing(v)
	}
	if v := strings.ToUpper(strings.TrimSpace(firstNonEmpty(q.Get("handshake-mode"), q.Get("handshakeMode")))); v != "" {
		extra["handshakeMode"] = normaliseHandshake(v)
	}
	if v := firstNonEmpty(q.Get("traffic-pattern"), q.Get("trafficPattern")); v != "" {
		extra["trafficPattern"] = v
	}
	// mieru 还有 mtu；Mihomo 不暴露该字段，保留以便未来导出客户端配置。
	if v := q.Get("mtu"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			extra["mtu"] = n
		}
	}

	return Config{
		Protocol: "mieru",
		Host:     host,
		Port:     port,
		Auth:     map[string]any{"username": username, "password": password},
		Transport: map[string]any{
			"sni":      q.Get("sni"),
			"insecure": qBool(q, "insecure") || qBool(q, "allowInsecure"),
		},
		Extra: extra,
	}, nil
}

// mieruDisplayName builds a human readable fallback name when the link carries
// no fragment, e.g. "mieru de.example.com:2090-2099".
func mieruDisplayName(c Config) string {
	if pr, ok := c.Extra["portRange"].(string); ok && pr != "" {
		return fmt.Sprintf("mieru %s:%s", c.Host, pr)
	}
	return fmt.Sprintf("mieru %s:%d", c.Host, c.Port)
}

// splitMieruHostPort splits "host:port", "host:2090-2099", "[v6]:2090-2099"
// and bare "host". Go's net/url rejects dashed ports, hence the manual split.
func splitMieruHostPort(s string) (host, port string) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "/"))
	if s == "" {
		return "", ""
	}
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]"); end > 0 {
			host = s[1:end]
			rest := strings.TrimPrefix(s[end+1:], ":")
			return host, rest
		}
	}
	if i := strings.LastIndex(s, ":"); i >= 0 && !strings.Contains(s[i+1:], ":") {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func parsePortRange(spec string) (int, int, error) {
	a, b, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, fmt.Errorf("端口范围格式应为 start-end")
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(a))
	hi, err2 := strconv.Atoi(strings.TrimSpace(b))
	if err1 != nil || err2 != nil || lo < 1 || hi > 65535 || lo > hi {
		return 0, 0, fmt.Errorf("端口范围无效")
	}
	return lo, hi, nil
}

func normaliseMultiplexing(v string) string {
	switch v {
	case "OFF", "0", "MULTIPLEXING_OFF":
		return "MULTIPLEXING_OFF"
	case "LOW", "1", "MULTIPLEXING_LOW":
		return "MULTIPLEXING_LOW"
	case "MIDDLE", "MID", "2", "MULTIPLEXING_MIDDLE":
		return "MULTIPLEXING_MIDDLE"
	case "HIGH", "3", "MULTIPLEXING_HIGH":
		return "MULTIPLEXING_HIGH"
	}
	return v
}

func normaliseHandshake(v string) string {
	v = strings.ReplaceAll(strings.ReplaceAll(v, "-", "_"), " ", "_")
	switch v {
	case "NO_WAIT", "NOWAIT", "HANDSHAKE_NO_WAIT", "0":
		return "HANDSHAKE_NO_WAIT"
	}
	return "HANDSHAKE_STANDARD"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// mieruProfile is the JSON document written by `mieru client` (mieru export
// config). Importing it is far more reliable than hand-writing URIs, so the
// subscription parser accepts it verbatim.
type mieruProfile struct {
	ProfileName string `json:"profileName"`
	User        struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	} `json:"user"`
	Servers []struct {
		IPAddress    string `json:"ipAddress"`
		DomainName   string `json:"domainName"`
		PortBindings []struct {
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		} `json:"portBindings"`
		PortRange string `json:"portRange"`
	} `json:"servers"`
	MTU          int `json:"mtu"`
	Multiplexing struct {
		Level int `json:"level"`
	} `json:"multiplexing"`
	HandshakeMode  string `json:"handshakeMode"`
	TrafficPattern string `json:"trafficPattern"`
}

// looksLikeMieruProfile reports whether the payload is a mieru client config.
func looksLikeMieruProfile(text string) bool {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "{") {
		return false
	}
	if !strings.Contains(t, "\"servers\"") {
		return false
	}
	return strings.Contains(t, "\"user\"") || strings.Contains(t, "portBindings")
}

// parseMieruProfile converts a mieru client config into one node per port
// binding. Names follow "profileName / host / port" so users can rename later.
func parseMieruProfile(text string) ([]Parsed, []ParseError, int) {
	var p mieruProfile
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return nil, []ParseError{{Line: 1, Reason: "Mieru 配置 JSON 解析失败"}}, 0
	}
	if p.User.Name == "" || p.User.Password == "" {
		return nil, []ParseError{{Line: 1, Reason: "Mieru 配置缺少 user.name / user.password"}}, 0
	}
	if len(p.Servers) == 0 {
		return nil, []ParseError{{Line: 1, Reason: "Mieru 配置缺少 servers"}}, 0
	}
	// Deterministic output: port bindings in file order, servers in order.
	var out []Parsed
	var errs []ParseError
	for si, srv := range p.Servers {
		host := firstNonEmpty(srv.IPAddress, srv.DomainName)
		if host == "" {
			errs = append(errs, ParseError{Line: si + 1, Reason: "Mieru server 缺少 ipAddress/domainName"})
			continue
		}
		base := p.ProfileName
		if base == "" {
			base = "mieru"
		}
		made := 0
		if srv.PortRange != "" {
			lo, hi, err := parsePortRange(srv.PortRange)
			if err != nil {
				errs = append(errs, ParseError{Line: si + 1, Reason: err.Error()})
			} else {
				cfg := p.mieruConfig(host, 0, lo, hi)
				out = append(out, Parsed{Name: fmt.Sprintf("%s %s %s", base, host, srv.PortRange), Config: cfg})
				made++
			}
		}
		for _, bind := range srv.PortBindings {
			if bind.Port < 1 || bind.Port > 65535 {
				errs = append(errs, ParseError{Line: si + 1, Reason: "Mieru portBindings 端口无效"})
				continue
			}
			cfg := p.mieruConfig(host, bind.Port, 0, 0)
			cfg.Extra["transport"] = strings.ToUpper(firstNonEmpty(bind.Protocol, "TCP"))
			out = append(out, Parsed{Name: fmt.Sprintf("%s %s %d", base, host, bind.Port), Config: cfg})
			made++
		}
		if made == 0 && srv.PortRange == "" {
			errs = append(errs, ParseError{Line: si + 1, Reason: "Mieru server 缺少 portBindings"})
		}
	}
	return out, errs, 0
}

func (p mieruProfile) mieruConfig(host string, port, lo, hi int) Config {
	extra := map[string]any{"transport": "TCP", "multiplexing": mieruMultiplexing(p.Multiplexing.Level)}
	if hi > 0 {
		extra["portRange"] = fmt.Sprintf("%d-%d", lo, hi)
	}
	if p.HandshakeMode != "" {
		extra["handshakeMode"] = normaliseHandshake(strings.ToUpper(p.HandshakeMode))
	}
	if p.TrafficPattern != "" {
		extra["trafficPattern"] = p.TrafficPattern
	}
	if p.MTU > 0 {
		extra["mtu"] = p.MTU
	}
	return Config{
		Protocol:  "mieru",
		Host:      host,
		Port:      port,
		Auth:      map[string]any{"username": p.User.Name, "password": p.User.Password},
		Transport: map[string]any{"sni": "", "insecure": false},
		Extra:     extra,
	}
}

func mieruMultiplexing(level int) string {
	switch level {
	case 0:
		return "MULTIPLEXING_OFF"
	case 2:
		return "MULTIPLEXING_MIDDLE"
	case 3:
		return "MULTIPLEXING_HIGH"
	default:
		return "MULTIPLEXING_LOW"
	}
}

// MieruExtraKeys lists Extra keys in output order, for renderers that want a
// stable field sequence.
var MieruExtraKeys = []string{"transport", "portRange", "multiplexing", "handshakeMode", "trafficPattern", "mtu"}
