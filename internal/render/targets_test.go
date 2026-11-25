package render

import (
	"strings"
	"testing"

	"github.com/wucai/subs/internal/protocol"
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
	if !strings.Contains(res.Body, "已跳过节点") {
		t.Error("missing skip comment")
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
