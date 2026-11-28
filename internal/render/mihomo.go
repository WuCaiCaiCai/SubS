package render

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/WuCaiCaiCai/SubS/internal/protocol"
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
	case "ss":
		set("type", "ss")
		set("cipher", authString(c, "cipher"))
		set("password", authString(c, "password"))
		if plugin := extraString(c, "plugin"); plugin != "" {
			set("plugin", plugin)
		}
	case "vmess":
		set("type", "vmess")
		set("uuid", authString(c, "uuid"))
		set("alterId", authInt(c, "alterId"))
		set("cipher", authString(c, "cipher"))
		if transportBool(c, "tls") || transportString(c, "security") == "tls" {
			set("tls", true)
		}
		if sni := transportString(c, "sni"); sni != "" {
			set("servername", sni)
		}
		setVMessTransport(set, c)
	case "trojan":
		set("type", "trojan")
		set("password", authString(c, "password"))
		set("tls", true)
		if sni := transportString(c, "sni"); sni != "" {
			set("sni", sni)
		}
		setVMessTransport(set, c)
	case "hy2":
		set("type", "hysteria2")
		set("password", authString(c, "password"))
		if sni := transportString(c, "sni"); sni != "" {
			set("sni", sni)
		}
		if alpn := transportString(c, "alpn"); alpn != "" {
			set("alpn", []string{alpn})
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
		// TUIC 走 QUIC，mihomo wiki 示例显式写 tls: true。
		set("tls", true)
		if sni := transportString(c, "sni"); sni != "" {
			set("sni", sni)
		}
		if alpn := transportString(c, "alpn"); alpn != "" {
			set("alpn", []string{alpn})
		}
		// mihomo 仅支持 v4/v5，默认 5；显式写出与参考配置一致，也避免旧内核猜测。
		set("version", extraInt(c, "tuicVersion", 5))
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
	// udp 开 UDP relay：解析端已按协议归一（QUIC 系默认开），
	// 显式写出与主流订阅输出一致，方便客户端直接采用。
	if udp, ok := c.Transport["udp"].(bool); ok {
		set("udp", udp)
	}
	out := make([]kv, 0, len(order))
	for _, k := range order {
		out = append(out, kv{Key: k, Value: m[k]})
	}
	return out, nil
}

func setVMessTransport(set func(string, any), c protocol.Config) {
	switch transportString(c, "network") {
	case "ws":
		opts := map[string]any{"path": transportString(c, "wsPath")}
		if host := transportString(c, "wsHost"); host != "" {
			opts["headers"] = map[string]any{"Host": host}
		}
		set("network", "ws")
		set("ws-opts", opts)
	case "grpc":
		set("network", "grpc")
		set("grpc-opts", map[string]any{"grpc-service-name": transportString(c, "grpcService")})
	}
}

// renderMihomo injects nodes into a user template via YAML AST edits.
//
// Injection points, in order of preference:
//  1. a mapping key annotated with `# subs:proxies`
//  2. the top-level `proxies:` key (covers templates that ship `proxies: ~` / `[]`)
//
// Group name lists are only rewritten where a `# subs:nodes` / `# subs:names`
// marker is present, so templates relying on `include-all: true` keep working
// untouched. When NO group received node names (no markers, no include-all),
// usable groups are auto-fixed by appending node names — otherwise the config
// would import fine but MATCH rules fall through to DIRECT.
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
	// 先跑标记注入（# subs:proxies / # subs:nodes / # subs:names），统计有多少组
	// 被改写，供后续回退判断使用。
	groupsWithNames := 0
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
				groupsWithNames++
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

	// 没有任何组通过标记拿到节点名列表时，给「可用组」自动补节点引用。
	// 背景：WebUI 默认模板和多数用户粘贴的配置既无 # subs:nodes 标记也无
	// include-all: true，此前注入只有 proxies 定义，组里仅剩 DIRECT，
	// MATCH 规则全部直连 —— 订阅“导入成功但不可用”的重大 bug。
	// 规则：仅处理 proxies 列表存在且不含 include-all 的组；把节点名追加在
	// 原有成员之后，保留用户已有的 DIRECT / 其他组引用。若组列表里已有本组
	// 之外的组名（显式组间引用），视为用户特意构造的层级，同样追加（无害）。
	if groupsWithNames == 0 && len(names) > 0 {
		fixGroupsWithoutNodes(doc, names)
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
	out := unescapeYAMLUnicode(buf.Bytes())
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

// unescapeYAMLUnicode restores supplementary-plane characters (like flag emoji)
// that yaml.v3 escapes as \U0001Fxxx inside double-quoted strings.
func unescapeYAMLUnicode(b []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(b))
	for i := 0; i < len(b); {
		if b[i] == '\\' && i+10 <= len(b) && b[i+1] == 'U' {
			bs := 0
			for j := i - 1; j >= 0 && b[j] == '\\'; j-- {
				bs++
			}
			if bs%2 == 0 {
				if v, err := strconv.ParseUint(string(b[i+2:i+10]), 16, 32); err == nil {
					r := rune(v)
					if r >= 0x10000 && r <= 0x10FFFF && utf8.ValidRune(r) {
						out.WriteRune(r)
						i += 10
						continue
					}
				}
			}
		}
		out.WriteByte(b[i])
		i++
	}
	return out.Bytes()
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

// fixGroupsWithoutNodes appends node names to every proxy-group under a
// `proxy-groups` key whose `proxies` list exists (non-null) and that lacks
// `include-all: true`. Only runs when no group was filled via markers, so
// explicit marker/include-all templates stay untouched.
//
// 层级组例外：成员里引用了其他组名（如 [Stable, DIRECT]）的组是用户特意
// 构造的选择层级，追加节点名会污染语义，直接跳过。
func fixGroupsWithoutNodes(doc *yaml.Node, names []string) {
	// 先收集全部组名，用于识别组间引用。
	groupNames := map[string]bool{}
	var collectNames func(n *yaml.Node)
	collectNames = func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if key.Value == "proxy-groups" && val.Kind == yaml.SequenceNode {
				for _, item := range val.Content {
					if item.Kind != yaml.MappingNode {
						continue
					}
					for j := 0; j+1 < len(item.Content); j += 2 {
						if item.Content[j].Value == "name" {
							groupNames[item.Content[j+1].Value] = true
						}
					}
				}
			}
			collectNames(val)
		}
	}
	collectNames(doc)

	var fix func(n *yaml.Node)
	fix = func(n *yaml.Node) {
		if n.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if key.Value == "proxy-groups" && val.Kind == yaml.SequenceNode {
				for _, item := range val.Content {
					fixGroup(item, names, groupNames)
				}
			}
			fix(val)
		}
	}
	fix(doc)
}

// fixGroup rewrites one proxy-group entry: appends node names to its
// `proxies` sequence, keeping existing members (DIRECT, group refs…).
func fixGroup(item *yaml.Node, names []string, groupNames map[string]bool) {
	if item.Kind != yaml.MappingNode {
		return
	}
	var list *yaml.Node
	hasIncludeAll := false
	for i := 0; i+1 < len(item.Content); i += 2 {
		k, v := item.Content[i], item.Content[i+1]
		switch k.Value {
		case "proxies":
			// 只改写已存在的列表：proxies: ~（null）意图是“留空”，不动。
			if v.Kind == yaml.SequenceNode {
				list = v
			}
		case "include-all":
			if b, err := strconv.ParseBool(v.Value); err == nil {
				hasIncludeAll = b
			}
		}
	}
	if list == nil || hasIncludeAll {
		return
	}
	existing := map[string]bool{}
	for _, m := range list.Content {
		existing[m.Value] = true
		// 成员引用了其他组名 → 层级组，不追加节点。
		if m.Value != "DIRECT" && m.Value != "REJECT" && groupNames[m.Value] {
			return
		}
	}
	for _, name := range names {
		if existing[name] {
			continue
		}
		list.Content = append(list.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: name,
		})
	}
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
