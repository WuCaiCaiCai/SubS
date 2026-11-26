package protocol

import "testing"

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
