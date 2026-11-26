package protocol

import (
	"strings"
	"testing"
)

func TestParseMieruShareLink(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		host     string
		port     int
		prange   string
		username string
		password string
		extra    map[string]string
		wantErr  bool
	}{
		{
			name: "basic userinfo",
			line: "mieru://alice:s3cret@de.example.com:9000#德国 01",
			host: "de.example.com", port: 9000, username: "alice", password: "s3cret",
			extra: map[string]string{"transport": "TCP"},
		},
		{
			name: "port range in authority",
			line: "mieru://alice:s3cret@de.example.com:2090-2099?transport=UDP#DE",
			host: "de.example.com", port: 0, prange: "2090-2099",
			username: "alice", password: "s3cret",
			extra: map[string]string{"transport": "UDP"},
		},
		{
			name: "credentials in query",
			line: "mieru://de.example.com:9000?username=bob&password=pw123#DE",
			host: "de.example.com", port: 9000, username: "bob", password: "pw123",
			extra: map[string]string{"transport": "TCP"},
		},
		{
			name: "full options",
			line: "mieru://u:p@de.example.com:9000?transport=tcp&multiplexing=high&handshake-mode=no-wait&traffic-pattern=QUJD#DE",
			host: "de.example.com", port: 9000, username: "u", password: "p",
			extra: map[string]string{
				"transport":      "TCP",
				"multiplexing":   "MULTIPLEXING_HIGH",
				"handshakeMode":  "HANDSHAKE_NO_WAIT",
				"trafficPattern": "QUJD",
			},
		},
		{
			name: "ipv6 with range",
			line: "mieru://u:p@[2001:db8::1]:2090-2099#v6",
			host: "2001:db8::1", port: 0, prange: "2090-2099",
			username: "u", password: "p",
			extra: map[string]string{"transport": "TCP"},
		},
		{
			name: "escaped password",
			line: "mieru://alice:p%40ss%3Aword@de.example.com:9000#DE",
			host: "de.example.com", port: 9000, username: "alice", password: "p@ss:word",
			extra: map[string]string{"transport": "TCP"},
		},
		{
			// mierus:// 是分享链接常见变体，scheme 多一个 s；protocol= 与 port= query
			// 与 authority 重复时按现有规则解析，profile= 等无关参数忽略。
			name: "mierus scheme variant with ipv6",
			line: "mierus://user:pw@[2a0a:d687:f007::9d]:30077?port=30077&protocol=TCP&profile=default#SG",
			host: "2a0a:d687:f007::9d", port: 30077, username: "user", password: "pw",
			extra: map[string]string{"transport": "TCP"},
		},
		{name: "missing password", line: "mieru://alice@de.example.com:9000", wantErr: true},
		{name: "missing host", line: "mieru://alice:p@:9000", wantErr: true},
		{name: "missing port", line: "mieru://alice:p@de.example.com", wantErr: true},
		{name: "bad range", line: "mieru://alice:p@de.example.com:2099-2090", wantErr: true},
		{name: "bad transport", line: "mieru://alice:p@de.example.com:9000?transport=ICMP", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseLine(tc.line)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", res.Config)
				}
				if strings.Contains(err.Error(), tc.password) && tc.password != "" {
					t.Fatalf("error message leaks password: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			c := res.Config
			if c.Protocol != "mieru" {
				t.Errorf("protocol = %q", c.Protocol)
			}
			if c.Host != tc.host || c.Port != tc.port {
				t.Errorf("endpoint = %s:%d, want %s:%d", c.Host, c.Port, tc.host, tc.port)
			}
			if got, _ := c.Extra["portRange"].(string); got != tc.prange {
				t.Errorf("portRange = %q, want %q", got, tc.prange)
			}
			if c.Auth["username"] != tc.username || c.Auth["password"] != tc.password {
				t.Errorf("auth = %v", c.Auth)
			}
			for k, want := range tc.extra {
				got, _ := c.Extra[k].(string)
				if got != want {
					t.Errorf("extra[%s] = %q, want %q", k, got, want)
				}
			}
			if res.Name == "" {
				t.Error("name must not be empty")
			}
		})
	}
}

func TestParseMieruShareLinkKeepsName(t *testing.T) {
	res, err := ParseLine("mieru://u:p@de.example.com:9000?sni=de.example.com# 东京 Mieru ")
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "东京 Mieru" {
		t.Errorf("name = %q", res.Name)
	}
	if sni, _ := res.Config.Transport["sni"].(string); sni != "de.example.com" {
		t.Errorf("sni = %q", sni)
	}
}

func TestParseMieruClientProfile(t *testing.T) {
	profile := `{
	  "profileName": "default",
	  "user": {"name": "alice", "password": "s3cret"},
	  "servers": [{
	    "ipAddress": "203.0.113.9",
	    "domainName": "",
	    "portBindings": [
	      {"port": 9000, "protocol": "TCP"},
	      {"port": 9001, "protocol": "UDP"}
	    ]
	  }],
	  "mtu": 1400,
	  "multiplexing": {"level": 2}
	}`
	nodes, errs, ignored := ParseSubscription([]byte(profile))
	if ignored != 0 {
		t.Errorf("ignored = %d", ignored)
	}
	if len(errs) != 0 {
		t.Fatalf("errors = %+v", errs)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if nodes[0].Config.Host != "203.0.113.9" || nodes[0].Config.Port != 9000 {
		t.Errorf("node0 = %s:%d", nodes[0].Config.Host, nodes[0].Config.Port)
	}
	if t0, _ := nodes[0].Config.Extra["transport"].(string); t0 != "TCP" {
		t.Errorf("node0 transport = %q", t0)
	}
	if t1, _ := nodes[1].Config.Extra["transport"].(string); t1 != "UDP" {
		t.Errorf("node1 transport = %q", t1)
	}
	if m, _ := nodes[0].Config.Extra["multiplexing"].(string); m != "MULTIPLEXING_MIDDLE" {
		t.Errorf("multiplexing = %q", m)
	}
	if nodes[0].Config.Auth["username"] != "alice" {
		t.Errorf("username = %v", nodes[0].Config.Auth["username"])
	}
	// fingerprints must differ for different ports
	if Fingerprint(nodes[0].Config) == Fingerprint(nodes[1].Config) {
		t.Error("distinct port bindings must have distinct fingerprints")
	}
}

func TestParseMieruProfileWithPortRange(t *testing.T) {
	profile := `{"profileName":"r","user":{"name":"u","password":"p"},"servers":[{"ipAddress":"198.51.100.7","portRange":"2090-2099"}]}`
	nodes, errs, _ := ParseSubscription([]byte(profile))
	if len(errs) != 0 || len(nodes) != 1 {
		t.Fatalf("nodes=%d errs=%+v", len(nodes), errs)
	}
	if pr, _ := nodes[0].Config.Extra["portRange"].(string); pr != "2090-2099" {
		t.Errorf("portRange = %q", pr)
	}
	if nodes[0].Config.Port != 0 {
		t.Errorf("port = %d, want 0 for range node", nodes[0].Config.Port)
	}
}

func TestMieruFingerprintIncludesPortRange(t *testing.T) {
	a, err := ParseLine("mieru://u:p@de.example.com:2090-2099")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseLine("mieru://u:p@de.example.com:3000-3009")
	if err != nil {
		t.Fatal(err)
	}
	if Fingerprint(a.Config) == Fingerprint(b.Config) {
		t.Error("different port ranges must not share a fingerprint")
	}
}

func TestClashMieruWithPortRange(t *testing.T) {
	yamlDoc := `proxies:
  - name: 德国 01
    type: mieru
    server: de.example.com
    port-range: 2090-2099
    transport: UDP
    username: alice
    password: s3cret
    multiplexing: MULTIPLEXING_LOW
    handshake-mode: HANDSHAKE_NO_WAIT
`
	nodes, errs, ignored := ParseSubscription([]byte(yamlDoc))
	if ignored != 0 || len(errs) != 0 || len(nodes) != 1 {
		t.Fatalf("nodes=%d errs=%+v ignored=%d", len(nodes), errs, ignored)
	}
	c := nodes[0].Config
	if c.Port != 0 {
		t.Errorf("port = %d", c.Port)
	}
	if pr, _ := c.Extra["portRange"].(string); pr != "2090-2099" {
		t.Errorf("portRange = %q", pr)
	}
	if tr, _ := c.Extra["transport"].(string); tr != "UDP" {
		t.Errorf("transport = %q", tr)
	}
	if hm, _ := c.Extra["handshakeMode"].(string); hm != "HANDSHAKE_NO_WAIT" {
		t.Errorf("handshakeMode = %q", hm)
	}
}
