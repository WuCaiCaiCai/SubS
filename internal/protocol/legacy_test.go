package protocol

import (
	"encoding/base64"
	"testing"
)

func TestParseLegacyShareLinks(t *testing.T) {
	cases := []struct{ line, proto string }{
		{"ss://YWVzLTI1Ni1nY206cGFzcw@ss.example.com:8388#ss", "ss"},
		{"vmess://eyJhZGQiOiJ2bS5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6IjExMTExMTExLTIyMjItMzMzMy00NDQ0LTU1NTU1NTU1NTU1NSIsIm5ldCI6IndzIiwidGxzIjoidGxzIiwicHMiOiJWTSJ9#vm", "vmess"},
		{"trojan://secret@tr.example.com:443?sni=tr.example.com#tr", "trojan"},
	}
	for _, c := range cases {
		r, err := ParseLine(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.proto, err)
			continue
		}
		if r.Config.Protocol != c.proto {
			t.Errorf("got %s", r.Config.Protocol)
		}
	}
}

func TestParseVMessUsesPsFingerprintAndAllowInsecure(t *testing.T) {
	raw := `{"v":"2","ps":"HKG 01","add":"vm.example.com","port":"30001","id":"11111111-2222-3333-4444-555555555555","aid":"0","scy":"auto","net":"tcp","tls":"tls","fp":"chrome","allowInsecure":1,"sni":"cdn.example.com"}`
	line := "vmess://" + base64.StdEncoding.EncodeToString([]byte(raw))
	r, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Name != "HKG 01" {
		t.Fatalf("expected name HKG 01, got %q", r.Name)
	}
	if fp, _ := r.Config.Transport["fingerprint"].(string); fp != "chrome" {
		t.Fatalf("expected fingerprint chrome, got %q", fp)
	}
	if ins, _ := r.Config.Transport["insecure"].(bool); !ins {
		t.Fatalf("expected insecure=true")
	}
}
