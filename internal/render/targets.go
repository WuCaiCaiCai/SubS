package render

import (
	"fmt"
	"strings"
)

// renderSingBox builds a minimal sing-box config with one selector group.
func renderSingBox(nodes []Node) (Result, error) {
	outbounds := make([]any, 0, len(nodes)+2)
	tags := make([]string, 0, len(nodes))
	var skipped []string
	for _, n := range nodes {
		if !Supports(TargetSingBox, n.Config.Protocol) {
			skipped = append(skipped, n.Name)
			continue
		}
		ob, err := singBoxOutbound(n)
		if err != nil {
			return Result{}, err
		}
		outbounds = append(outbounds, ob)
		tags = append(tags, n.Name)
	}
	group := []any{"selector"}
	for _, t := range tags {
		group = append(group, t)
	}
	head := map[string]any{
		"log":       map[string]any{"level": "info", "timestamp": true},
		"outbounds": append([]any{map[string]any{"type": "selector", "tag": "PROXY", "outbounds": group}}, outbounds...),
	}
	body := marshal(head)

	res := Result{Body: body, Skipped: skipped}
	if len(skipped) > 0 {
		res.Body = fmt.Sprintf("// 已跳过节点（sing-box 不支持）: %s\n%s", strings.Join(skipped, ", "), body)
	}
	return res, nil
}

func singBoxOutbound(n Node) (map[string]any, error) {
	c := n.Config
	ob := map[string]any{
		"type":        c.Protocol,
		"tag":         n.Name,
		"server":      c.Host,
		"server_port": c.Port,
	}
	tls := map[string]any{"enabled": true}
	if sni := transportString(c, "sni"); sni != "" {
		tls["server_name"] = sni
	}
	if transportBool(c, "insecure") {
		tls["insecure"] = true
	}
	switch c.Protocol {
	case "hy2":
		ob["type"] = "hysteria2"
		ob["password"] = authString(c, "password")
		if obfs := extraString(c, "obfs"); obfs != "" {
			ob["obfs"] = map[string]any{"type": obfs, "password": extraString(c, "obfsPassword")}
		}
		ob["tls"] = tls
	case "vless":
		ob["uuid"] = authString(c, "uuid")
		if flow := authString(c, "flow"); flow != "" {
			ob["flow"] = flow
		}
		switch transportString(c, "security") {
		case "reality":
			tls["reality"] = map[string]any{
				"enabled":    true,
				"public_key": transportString(c, "publicKey"),
				"short_id":   transportString(c, "shortId"),
			}
			if fp := transportString(c, "fingerprint"); fp != "" {
				tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
			}
		case "none":
			tls = nil
		}
		if tls != nil {
			ob["tls"] = tls
		}
		switch network := transportString(c, "network"); network {
		case "ws":
			ob["transport"] = map[string]any{
				"type":    "ws",
				"path":    transportString(c, "wsPath"),
				"headers": map[string]any{"Host": transportString(c, "wsHost")},
			}
		case "grpc":
			ob["transport"] = map[string]any{"type": "grpc", "service_name": transportString(c, "grpcService")}
		}
	case "tuic":
		ob["uuid"] = authString(c, "uuid")
		ob["password"] = authString(c, "password")
		if alpn := transportString(c, "alpn"); alpn != "" {
			tls["alpn"] = []string{alpn}
		}
		if cc := extraString(c, "congestionControl"); cc != "" {
			ob["congestion_control"] = cc
		}
		ob["tls"] = tls
	case "anytls":
		ob["password"] = authString(c, "password")
		ob["tls"] = tls
	case "mieru":
		return nil, fmt.Errorf("mieru 无 sing-box 出站")
	default:
		return nil, fmt.Errorf("不支持的协议: %s", c.Protocol)
	}
	return ob, nil
}

// renderLoon builds the Loon [Proxy] and [Proxy Group] sections.
func renderLoon(nodes []Node) (Result, error) {
	var b strings.Builder
	var names []string
	var skipped []string
	b.WriteString("[Proxy]\n")
	for _, n := range nodes {
		if !Supports(TargetLoon, n.Config.Protocol) {
			skipped = append(skipped, n.Name)
			continue
		}
		line, err := loonProxyLine(n)
		if err != nil {
			return Result{}, err
		}
		b.WriteString(line)
		b.WriteString("\n")
		names = append(names, n.Name)
	}
	b.WriteString("\n[Proxy Group]\n")
	fmt.Fprintf(&b, "Proxy = select, %s\n", strings.Join(append([]string{"DIRECT"}, names...), ", "))
	b.WriteString("\n[Rule]\nFINAL,Proxy\n")
	res := Result{Body: b.String(), Skipped: skipped}
	if len(skipped) > 0 {
		res.Body = fmt.Sprintf("# 已跳过节点（Loon 不支持）: %s\n%s", strings.Join(skipped, ", "), res.Body)
	}
	return res, nil
}

func loonProxyLine(n Node) (string, error) {
	c := n.Config
	base := fmt.Sprintf("%s, %s, %d", n.Name, c.Host, c.Port)
	switch c.Protocol {
	case "hy2":
		parts := []string{base, "password=" + authString(c, "password")}
		if sni := transportString(c, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if transportBool(c, "insecure") {
			parts = append(parts, "skip-cert-verify=true")
		}
		if obfs := extraString(c, "obfs"); obfs != "" {
			parts = append(parts, "obfs="+obfs)
			if p := extraString(c, "obfsPassword"); p != "" {
				parts = append(parts, "obfs-password="+p)
			}
		}
		return "hysteria2=" + strings.Join(parts, ", "), nil
	case "vless":
		parts := []string{base, "uuid=" + authString(c, "uuid")}
		security := transportString(c, "security")
		if security == "tls" || security == "reality" {
			parts = append(parts, "tls=true")
		}
		if sni := transportString(c, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if fp := transportString(c, "fingerprint"); fp != "" {
			parts = append(parts, "fp="+fp)
		}
		if security == "reality" {
			parts = append(parts, "pbk="+transportString(c, "publicKey"), "sid="+transportString(c, "shortId"))
		}
		if flow := authString(c, "flow"); flow != "" {
			parts = append(parts, "flow="+flow)
		}
		if network := transportString(c, "network"); network == "ws" {
			parts = append(parts, "transport=ws", "path="+transportString(c, "wsPath"))
		}
		return "vless=" + strings.Join(parts, ", "), nil
	case "tuic":
		parts := []string{base, "uuid=" + authString(c, "uuid"), "password=" + authString(c, "password")}
		if sni := transportString(c, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if alpn := transportString(c, "alpn"); alpn != "" {
			parts = append(parts, "alpn="+alpn)
		}
		if cc := extraString(c, "congestionControl"); cc != "" {
			parts = append(parts, "congestion-controller="+cc)
		}
		return "tuic=" + strings.Join(parts, ", "), nil
	case "anytls":
		parts := []string{base, "password=" + authString(c, "password")}
		if sni := transportString(c, "sni"); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		return "anytls=" + strings.Join(parts, ", "), nil
	}
	return "", fmt.Errorf("Loon 不支持协议: %s", c.Protocol)
}
