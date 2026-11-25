package render

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// kv is one ordered mapping entry, so generated proxies keep name/type/server
// first instead of YAML's alphabetical map ordering.
type kv struct {
	Key   string
	Value any
}

// proxyEntries converts a node into an ordered Clash/Mihomo proxy mapping.
func proxyEntries(n Node) ([]kv, error) {
	c := n.Config
	m := map[string]any{}
	var order []string
	set := func(k string, v any) {
		if _, exists := m[k]; !exists {
			order = append(order, k)
		}
		m[k] = v
	}
	set("name", n.Name)
	set("server", c.Host)
	// mieru 可以用端口范围代替单端口。
	if pr := extraString(c, "portRange"); pr != "" && c.Port == 0 {
		set("port-range", pr)
	} else {
		set("port", c.Port)
	}
	switch c.Protocol {
	case "hy2":
		set("type", "hysteria2")
		set("password", authString(c, "password"))
		if sni := transportString(c, "sni"); sni != "" {
			set("sni", sni)
		}
		if transportBool(c, "insecure") {
			set("skip-cert-verify", true)
		}
		if obfs := extraString(c, "obfs"); obfs != "" {
			set("obfs", obfs)
			if p := extraString(c, "obfsPassword"); p != "" {
				set("obfs-password", p)
			}
		}
	case "vless":
		set("type", "vless")
		set("uuid", authString(c, "uuid"))
		security := transportString(c, "security")
		switch security {
		case "tls":
			set("tls", true)
			if sni := transportString(c, "sni"); sni != "" {
				set("servername", sni)
			}
		case "reality":
			set("tls", true)
			if sni := transportString(c, "sni"); sni != "" {
				set("servername", sni)
			}
			set("reality-opts", map[string]any{
				"public-key": transportString(c, "publicKey"),
				"short-id":   transportString(c, "shortId"),
			})
		}
		if fp := transportString(c, "fingerprint"); fp != "" {
			set("client-fingerprint", fp)
		}
		if flow := authString(c, "flow"); flow != "" {
			set("flow", flow)
		}
		network := transportString(c, "network")
		if network == "" {
			network = "tcp"
		}
		if network != "tcp" {
			set("network", network)
		}
		switch network {
		case "ws":
			opts := map[string]any{"path": transportString(c, "wsPath")}
			if host := transportString(c, "wsHost"); host != "" {
				opts["headers"] = map[string]any{"Host": host}
			}
			set("ws-opts", opts)
		case "grpc":
			set("grpc-opts", map[string]any{"grpc-service-name": transportString(c, "grpcService")})
		}
		if transportBool(c, "insecure") {
			set("skip-cert-verify", true)
		}
	case "tuic":
		set("type", "tuic")
		set("uuid", authString(c, "uuid"))
		set("password", authString(c, "password"))
		if sni := transportString(c, "sni"); sni != "" {
			set("sni", sni)
		}
		if alpn := transportString(c, "alpn"); alpn != "" {
			set("alpn", []string{alpn})
		}
		if cc := extraString(c, "congestionControl"); cc != "" {
			set("congestion-controller", cc)
		}
		if transportBool(c, "insecure") {
			set("skip-cert-verify", true)
		}
	case "anytls":
		set("type", "anytls")
		set("password", authString(c, "password"))
		if sni := transportString(c, "sni"); sni != "" {
			set("sni", sni)
		}
		if fp := transportString(c, "fingerprint"); fp != "" {
			set("client-fingerprint", fp)
		}
		if transportBool(c, "insecure") {
			set("skip-cert-verify", true)
		}
	case "mieru":
		set("type", "mieru")
		set("username", authString(c, "username"))
		set("password", authString(c, "password"))
		if t := strings.ToUpper(extraString(c, "transport")); t != "" {
			set("transport", t)
		}
		if v := extraString(c, "multiplexing"); v != "" {
			set("multiplexing", v)
		}
		if v := extraString(c, "handshakeMode"); v != "" {
			set("handshake-mode", v)
		}
		if v := extraString(c, "trafficPattern"); v != "" {
			set("traffic-pattern", v)
		}
	default:
		return nil, fmt.Errorf("不支持的协议: %s", c.Protocol)
	}
	out := make([]kv, 0, len(order))
	for _, k := range order {
		out = append(out, kv{Key: k, Value: m[k]})
	}
	return out, nil
}

// renderMihomo injects nodes into a user template via YAML AST edits.
//
// Injection points, in order of preference:
//  1. a mapping key annotated with `# subs:proxies`
//  2. the top-level `proxies:` key (covers templates that ship `proxies: ~` / `[]`)
//
// Group name lists are only rewritten where a `# subs:nodes` / `# subs:names`
// marker is present, so templates relying on `include-all: true` keep working
// untouched. Nothing else in the template is modified.
func renderMihomo(template string, nodes []Node) (Result, error) {
	if strings.TrimSpace(template) == "" {
		template = defaultMihomoTemplate
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(template), &root); err != nil {
		return Result{}, fmt.Errorf("模板 YAML 解析失败: %v", err)
	}
	if len(root.Content) == 0 {
		return Result{}, fmt.Errorf("模板为空")
	}
	doc := root.Content[0]

	proxies := make([]*yaml.Node, 0, len(nodes))
	names := make([]string, 0, len(nodes))
	var skipped []string
	for _, n := range nodes {
		if !Supports(TargetMihomo, n.Config.Protocol) {
			skipped = append(skipped, n.Name)
			continue
		}
		node, err := proxyNode(n)
		if err != nil {
			return Result{}, err
		}
		proxies = append(proxies, node)
		names = append(names, n.Name)
	}

	proxySeq := func() *yaml.Node {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		seq.Content = append(seq.Content, proxies...)
		return seq
	}
	nameSeq := func() *yaml.Node {
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, name := range names {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name})
		}
		return seq
	}

	injected := false
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.SequenceNode {
			for _, item := range n.Content {
				walk(item)
			}
			return
		}
		if n.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			markers := markerSet(key)
			switch {
			case markers["proxies"]:
				n.Content[i+1] = proxySeq()
				injected = true
			case markers["nodes"] || markers["names"]:
				n.Content[i+1] = nameSeq()
			default:
				walk(val)
			}
		}
	}
	walk(doc)

	if !injected {
		// Fall back to the canonical top-level proxies key.
		if doc.Kind != yaml.MappingNode {
			return Result{}, fmt.Errorf("模板顶层必须是映射")
		}
		for i := 0; i+1 < len(doc.Content); i += 2 {
			if doc.Content[i].Value == "proxies" {
				doc.Content[i+1] = proxySeq()
				injected = true
				break
			}
		}
	}
	if !injected {
		return Result{}, fmt.Errorf("模板缺少 proxies 键或 # subs:proxies 注入点")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return Result{}, fmt.Errorf("生成 YAML 失败")
	}
	if err := enc.Close(); err != nil {
		return Result{}, fmt.Errorf("生成 YAML 失败")
	}
	out := buf.Bytes()
	var check yaml.Node
	if err := yaml.Unmarshal(out, &check); err != nil {
		return Result{}, fmt.Errorf("生成的 YAML 无法重新解析")
	}
	res := Result{Body: string(out), Skipped: skipped}
	if len(skipped) > 0 {
		res.Body = fmt.Sprintf("# 已跳过节点（当前客户端不支持）: %s\n%s", strings.Join(skipped, ", "), res.Body)
	}
	return res, nil
}

// markerSet reports which subs markers annotate a mapping key.
func markerSet(key *yaml.Node) map[string]bool {
	out := map[string]bool{}
	if key.Kind != yaml.ScalarNode || key.HeadComment == "" {
		return out
	}
	for _, line := range strings.Split(key.HeadComment, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		if strings.HasPrefix(line, "subs:") {
			out[strings.TrimSpace(strings.TrimPrefix(line, "subs:"))] = true
		}
	}
	return out
}

// proxyNode renders one proxy as an ordered YAML mapping node.
func proxyNode(n Node) (*yaml.Node, error) {
	entries, err := proxyEntries(n)
	if err != nil {
		return nil, err
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, e := range entries {
		val, err := toYAMLNode(e.Value)
		if err != nil {
			return nil, err
		}
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.Key}, val)
	}
	return m, nil
}

func toYAMLNode(v any) (*yaml.Node, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	if len(n.Content) == 0 {
		return nil, fmt.Errorf("空节点")
	}
	return n.Content[0], nil
}

const defaultMihomoTemplate = `port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
# subs:proxies
proxies: []
proxy-groups:
  - name: PROXY
    type: select
    # subs:nodes
    proxies:
      - DIRECT
rules:
  - MATCH,PROXY
`

var _ = strconv.Itoa
