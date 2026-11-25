package render

import "github.com/wucai/subs/internal/protocol"

// sampleConfig builds a representative config per protocol for dry-run validation.
func sampleConfig(proto string) protocol.Config {
	switch proto {
	case "vless":
		return protocol.Config{Protocol: "vless", Host: "example.com", Port: 443,
			Auth:      map[string]any{"uuid": "11111111-2222-3333-4444-555555555555"},
			Transport: map[string]any{"security": "tls", "sni": "example.com", "network": "tcp"}}
	case "tuic":
		return protocol.Config{Protocol: "tuic", Host: "example.com", Port: 443,
			Auth:      map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "password": "pw"},
			Transport: map[string]any{"sni": "example.com", "alpn": "h3"},
			Extra:     map[string]any{"congestionControl": "bbr"}}
	case "anytls":
		return protocol.Config{Protocol: "anytls", Host: "example.com", Port: 443,
			Auth: map[string]any{"password": "pw"}, Transport: map[string]any{"sni": "example.com"}}
	case "mieru":
		return protocol.Config{Protocol: "mieru", Host: "example.com", Port: 443,
			Auth: map[string]any{"username": "u", "password": "pw"}, Transport: map[string]any{}}
	default:
		return protocol.Config{Protocol: "hy2", Host: "example.com", Port: 443,
			Auth: map[string]any{"password": "pw"}, Transport: map[string]any{"sni": "example.com"}}
	}
}
