package render

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/WuCaiCaiCai/SubS/internal/protocol"
)

func testNodes() []Node {
	return []Node{
		{Name: "01-HK-HY2", Config: protocol.Config{Protocol: "hy2", Host: "hk.example.com", Port: 443,
			Auth: map[string]any{"password": "pw"}, Transport: map[string]any{"sni": "hk.example.com", "insecure": true},
			Extra: map[string]any{"obfs": "salamander", "obfsPassword": "op"}}},
		{Name: "02-JP-VLESS", Config: protocol.Config{Protocol: "vless", Host: "jp.example.com", Port: 443,
			Auth:      map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "flow": "xtls-rprx-vision"},
			Transport: map[string]any{"security": "reality", "sni": "www.microsoft.com", "publicKey": "PBK", "shortId": "ab", "fingerprint": "chrome", "network": "tcp"}}},
		{Name: "03-SG-TUIC", Config: protocol.Config{Protocol: "tuic", Host: "sg.example.com", Port: 443,
			Auth:      map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "password": "pw"},
			Transport: map[string]any{"sni": "sg.example.com", "alpn": "h3"},
			Extra:     map[string]any{"congestionControl": "bbr"}}},
		{Name: "04-US-ANYTLS", Config: protocol.Config{Protocol: "anytls", Host: "us.example.com", Port: 443,
			Auth: map[string]any{"password": "pw"}, Transport: map[string]any{"sni": "us.example.com"}}},
		{Name: "05-DE-MIERU", Config: protocol.Config{Protocol: "mieru", Host: "de.example.com", Port: 9000,
			Auth: map[string]any{"username": "u", "password": "pw"}, Transport: map[string]any{}}},
	}
}

func TestRenderSingBoxAllProtocols(t *testing.T) {
	res, err := Render(TargetSingBox, "", testNodes())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"type": "hysteria2"`, `"type": "vless"`, `"reality"`, `"type": "tuic"`, `"type": "anytls"`, `"selector"`} {
		if !strings.Contains(res.Body, want) {
			t.Errorf("sing-box missing %s", want)
		}
	}
	if strings.Contains(res.Body, `"type": "mieru"`) {
		t.Error("mieru must not be rendered for sing-box")
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "05-DE-MIERU" {
		t.Fatalf("skipped=%v", res.Skipped)
	}
	// sing-box 只接受纯 JSON，被跳过的节点必须通过 Result.Skipped 暴露，
	// 不能写进 body（否则客户端解析失败）。
	if !json.Valid([]byte(res.Body)) {
		t.Error("sing-box body must be valid JSON")
	}
	if strings.Contains(res.Body, "已跳过") {
		t.Error("sing-box body must not contain comments")
	}
}

func TestRenderLoonAllProtocols(t *testing.T) {
	res, err := Render(TargetLoon, "", testNodes())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"hysteria2=", "vless=", "tuic=", "anytls=", "[Proxy Group]", "FINAL,Proxy"} {
		if !strings.Contains(res.Body, want) {
			t.Errorf("loon missing %q", want)
		}
	}
	if strings.Contains(res.Body, "mieru=") {
		t.Error("mieru must not be rendered for loon")
	}
}

func TestRenderMihomoFieldMapping(t *testing.T) {
	res, err := Render(TargetMihomo, "", testNodes())
	if err != nil {
		t.Fatal(err)
	}
	body := res.Body
	for _, want := range []string{
		"type: hysteria2", "obfs: salamander", "obfs-password: op",
		"reality-opts:", "public-key: PBK", "short-id: ab", "client-fingerprint: chrome", "flow: xtls-rprx-vision",
		"congestion-controller: bbr", "type: anytls", "type: mieru",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("mihomo missing %q", want)
		}
	}
	// Name/type/server must lead each proxy mapping.
	if !strings.Contains(body, "- name: 01-HK-HY2") {
		t.Error("proxy entries not ordered by name first")
	}
}

func TestRenderMihomoKeepsFlagEmojiUnescaped(t *testing.T) {
	nodes := []Node{{
		Name: "🇭🇰-HKG 01",
		Config: protocol.Config{
			Protocol:  "vless",
			Host:      "hk.example.com",
			Port:      443,
			Auth:      map[string]any{"uuid": "11111111-1111-1111-1111-111111111111"},
			Transport: map[string]any{"security": "tls", "sni": "hk.example.com"},
		},
	}}
	res, err := Render(TargetMihomo, "", nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Body, "🇭🇰-HKG 01") {
		t.Fatalf("expected raw flag emoji in YAML, got:\n%s", res.Body)
	}
	if strings.Contains(res.Body, `\U0001F1`) {
		t.Fatalf("unexpected escaped unicode in YAML:\n%s", res.Body)
	}
}

func TestRenderMihomoMarkerInjection(t *testing.T) {
	tmpl := `port: 7890
# subs:proxies
proxies: []
proxy-groups:
  - name: PROXY
    type: select
    # subs:nodes
    proxies: []
rules:
  - MATCH,PROXY
`
	res, err := Render(TargetMihomo, tmpl, testNodes()[:2])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Body, "- 01-HK-HY2") || !strings.Contains(res.Body, "- 02-JP-VLESS") {
		t.Fatalf("names not injected:\n%s", res.Body)
	}
}

// include-all 组会由 Mihomo 自行展开全部节点，自动补名会造成重复；
// 有 # subs:nodes 标记的模板也不应触发回退逻辑。
func TestRenderMihomoAutoFixSkipsIncludeAllAndMarkers(t *testing.T) {
	tmpl := `proxies: ~
proxy-groups:
  - name: All
    type: select
    include-all: true
    proxies:
      - DIRECT
  - name: P2
    type: select
    # subs:nodes
    proxies: []
rules:
  - MATCH,All
`
	res, err := Render(TargetMihomo, tmpl, testNodes()[:2])
	if err != nil {
		t.Fatal(err)
	}
	// include-all 组不得被追加节点名（否则与自动展开重复）。
	if strings.Contains(res.Body, "    proxies:\n      - DIRECT\n      - 01-HK-HY2") {
		t.Fatalf("include-all group must stay untouched:\n%s", res.Body)
	}
	// 标记组照常注入。
	if !strings.Contains(res.Body, "- 01-HK-HY2") {
		t.Fatalf("marker group must be injected:\n%s", res.Body)
	}
}

func TestValidateRejectsUnknownMarker(t *testing.T) {
	issues, _ := ValidateTemplate("proxies: []\n# subs:bogus\nproxy-groups: []\n")
	found := false
	for _, i := range issues {
		if strings.Contains(i.Message, "未知 subs 标记") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown marker issue, got %+v", issues)
	}
}

func TestSupportsMatrix(t *testing.T) {
	if Supports(TargetSingBox, "mieru") || Supports(TargetLoon, "mieru") {
		t.Error("mieru should be unsupported on sing-box/loon")
	}
	for _, p := range []string{"hy2", "vless", "tuic", "anytls"} {
		for _, tg := range []string{TargetMihomo, TargetSingBox, TargetLoon} {
			if !Supports(tg, p) {
				t.Errorf("%s should support %s", tg, p)
			}
		}
	}
}

func TestRenderMihomoMieruPortRange(t *testing.T) {
	tpl := "proxies: ~\nproxy-groups:\n  - name: PROXY\n    type: select\n    include-all: true\nrules:\n  - MATCH,PROXY\n"
	res, err := Render(TargetMihomo, tpl, []Node{{Name: "01-DE-MIERU", Config: protocol.Config{
		Protocol: "mieru", Host: "de.example.com", Port: 0,
		Auth:      map[string]any{"username": "alice", "password": "s3cret"},
		Transport: map[string]any{"sni": "", "insecure": false},
		Extra: map[string]any{
			"transport":      "UDP",
			"portRange":      "2090-2099",
			"multiplexing":   "MULTIPLEXING_HIGH",
			"handshakeMode":  "HANDSHAKE_NO_WAIT",
			"trafficPattern": "QUJD",
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(res.Body), &doc); err != nil {
		t.Fatalf("rendered YAML is invalid: %v", err)
	}
	if len(doc.Proxies) != 1 {
		t.Fatalf("proxies = %d", len(doc.Proxies))
	}
	p := doc.Proxies[0]
	if p["type"] != "mieru" {
		t.Errorf("type = %v", p["type"])
	}
	if p["port-range"] != "2090-2099" {
		t.Errorf("port-range = %v", p["port-range"])
	}
	if _, hasPort := p["port"]; hasPort {
		t.Error("port must be omitted when port-range is used")
	}
	if p["transport"] != "UDP" {
		t.Errorf("transport = %v", p["transport"])
	}
	if p["multiplexing"] != "MULTIPLEXING_HIGH" {
		t.Errorf("multiplexing = %v", p["multiplexing"])
	}
	if p["handshake-mode"] != "HANDSHAKE_NO_WAIT" {
		t.Errorf("handshake-mode = %v", p["handshake-mode"])
	}
	if p["traffic-pattern"] != "QUJD" {
		t.Errorf("traffic-pattern = %v", p["traffic-pattern"])
	}
	if p["username"] != "alice" || p["password"] != "s3cret" {
		t.Errorf("credentials missing: %v", p)
	}
}
func TestRenderMihomoDefaultTemplateGroupsGetNodes(t *testing.T) {
	// WebUI 新建模板的默认内容：没有 # subs: 标记，组里只有 DIRECT。
	tmpl := `mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090

proxies:

proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - DIRECT

rules:
  - MATCH,PROXY
`
	res, err := Render(TargetMihomo, tmpl, testNodes()[:2])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Body, "- 01-HK-HY2") {
		t.Fatalf("BUG: nodes never appear in any group:\n%s", res.Body)
	}
}
