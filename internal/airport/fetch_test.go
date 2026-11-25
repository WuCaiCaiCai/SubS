package airport

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestGuardHostBlocksInternalTargets(t *testing.T) {
	blocked := []string{"localhost", "127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "::1"}
	for _, host := range blocked {
		if err := guardHost(context.Background(), host); err == nil {
			t.Errorf("%s should be blocked", host)
		}
	}
}

func TestIsBlockedIP(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.169.254", "::1", "fc00::1", "0.0.0.0"} {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "2001:4860:4860::8888"} {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestFetchRejectsNonHTTP(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/x", "gopher://x"} {
		if _, err := Fetch(context.Background(), u); err == nil {
			t.Errorf("%s should be rejected", u)
		}
	}
}

func TestFetchBlocksLoopback(t *testing.T) {
	_, err := Fetch(context.Background(), "http://127.0.0.1:8080/sub")
	if !errors.Is(err, ErrBlockedURL) {
		t.Fatalf("expected ErrBlockedURL, got %v", err)
	}
}
