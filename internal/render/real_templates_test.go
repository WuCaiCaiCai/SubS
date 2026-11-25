package render

import (
	"os"
	"strings"
	"testing"

	"github.com/wucai/subs/internal/protocol"
)

func TestRealTemplates(t *testing.T) {
	nodes := []Node{
		{Name: "01-HK-HY2-香港01", Config: protocol.Config{Protocol: "hy2", Host: "hk1.example.com", Port: 443,
			Auth: map[string]any{"password": "secret"}, Transport: map[string]any{"sni": "hk1.example.com", "insecure": false}}},
		{Name: "02-JP-VLESS-东京", Config: protocol.Config{Protocol: "vless", Host: "jp1.example.com", Port: 443,
			Auth:      map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"},
			Transport: map[string]any{"security": "reality", "sni": "www.microsoft.com", "publicKey": "PBK", "shortId": "ab", "fingerprint": "chrome", "network": "tcp"}}},
		{Name: "03-US-TUIC-美国", Config: protocol.Config{Protocol: "tuic", Host: "us1.example.com", Port: 443,
			Auth:      map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "password": "pw"},
			Transport: map[string]any{"sni": "us1.example.com", "alpn": "h3", "insecure": true},
			Extra:     map[string]any{"congestionControl": "bbr"}}},
	}
	for _, f := range []string{"../../模板示例/Client.yaml", "../../模板示例/Router.yaml"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		res, err := Render(TargetMihomo, string(raw), nodes)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, want := range []string{"hk1.example.com", "jp1.example.com", "us1.example.com", "reality-opts", "congestion-controller"} {
			if !strings.Contains(res.Body, want) {
				t.Errorf("%s: missing %q", f, want)
			}
		}
		if strings.Contains(res.Body, "proxies: ~") || strings.Contains(res.Body, "proxies: []") {
			t.Errorf("%s: proxies not injected", f)
		}
		if !strings.Contains(res.Body, "include-all: true") {
			t.Errorf("%s: template body altered", f)
		}
	}
}
