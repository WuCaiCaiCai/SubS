package protocol

import "testing"

const uuid = "11111111-2222-3333-4444-555555555555"

func TestParseShareLinks(t *testing.T) {
	cases := []struct {
		line     string
		protocol string
		host     string
		port     int
		authKey  string
	}{
		{"hy2://pw@hk.example.com:443?sni=hk.example.com&insecure=1#香港", "hy2", "hk.example.com", 443, "password"},
		{"hysteria2://pw@hk.example.com:8443?obfs=salamander&obfs-password=x#HK", "hy2", "hk.example.com", 8443, "password"},
		{"vless://" + uuid + "@jp.example.com:443?security=reality&sni=a.com&pbk=PBK&sid=ab&fp=chrome#JP", "vless", "jp.example.com", 443, "uuid"},
		{"tuic://" + uuid + ":pw@us.example.com:443?sni=us.example.com&alpn=h3#US", "tuic", "us.example.com", 443, "uuid"},
		{"anytls://pw@sg.example.com:443?sni=sg.example.com#SG", "anytls", "sg.example.com", 443, "password"},
		{"mieru://user:pw@de.example.com:9000?sni=de.example.com#DE", "mieru", "de.example.com", 9000, "username"},
	}
	for _, c := range cases {
		res, err := ParseLine(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.protocol, err)
			continue
		}
		if res.Config.Protocol != c.protocol || res.Config.Host != c.host || res.Config.Port != c.port {
			t.Errorf("%s: got %+v", c.line, res.Config)
		}
		if res.Config.Auth[c.authKey] == nil {
			t.Errorf("%s: missing auth.%s", c.protocol, c.authKey)
		}
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"",
		"not a link",
		"ss://unsupported@host:443#x",
		"vless://not-a-uuid@host:443?security=tls#x",
		"hy2://pw@host:0#x",
		"vless://" + uuid + "@host:443?security=reality&sni=a.com#x",
	}
	for _, line := range bad {
		if _, err := ParseLine(line); err == nil {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Config{Protocol: "hy2", Host: "HK.example.com", Port: 443, Auth: map[string]any{"password": "pw"}}
	b := Config{Protocol: "hy2", Host: "hk.example.com", Port: 443, Auth: map[string]any{"password": "pw"}}
	if Fingerprint(a) != Fingerprint(b) {
		t.Fatal("fingerprint should be case-insensitive on host")
	}
	c := Config{Protocol: "hy2", Host: "hk.example.com", Port: 443, Auth: map[string]any{"password": "other"}}
	if Fingerprint(a) == Fingerprint(c) {
		t.Fatal("different credentials must not collide")
	}
}

func TestParseSubscriptionBase64(t *testing.T) {
	raw := "hy2://pw@a.example.com:443#A\nvless://" + uuid + "@b.example.com:443?security=tls#B\n"
	encoded := encodeBase64(raw)
	parsed, errs, ignored := ParseSubscription([]byte(encoded))
	if len(parsed) != 2 || len(errs) != 0 || ignored != 0 {
		t.Fatalf("parsed=%d errs=%d ignored=%d", len(parsed), len(errs), ignored)
	}
}

func TestParseSubscriptionClashYAML(t *testing.T) {
	body := `proxies:
  - name: HK
    type: hysteria2
    server: hk.example.com
    port: 443
    password: pw
    sni: hk.example.com
  - name: JP
    type: vless
    server: jp.example.com
    port: 443
    uuid: ` + uuid + `
    tls: true
    network: ws
    ws-opts:
      path: /ws
  - name: Ignored
    type: ss
    server: x.example.com
    port: 8388
    cipher: aes-128-gcm
    password: pw
`
	parsed, errs, ignored := ParseSubscription([]byte(body))
	if len(parsed) != 3 {
		t.Fatalf("parsed=%d errs=%+v", len(parsed), errs)
	}
	if ignored != 0 {
		t.Fatalf("ignored=%d want 0", ignored)
	}
	if parsed[1].Config.Transport["network"] != "ws" || parsed[1].Config.Transport["wsPath"] != "/ws" {
		t.Fatalf("ws transport lost: %+v", parsed[1].Config.Transport)
	}
}
