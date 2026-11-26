package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WuCaiCaiCai/SubS/internal/store"
)

func TestCaddyConfigRejectsInjectionAndRemoteUpstream(t *testing.T) {
	for _, c := range []CaddyConfig{
		{Domain: "example.com { respond 200", Port: 8787},
		{Domain: "127.0.0.1", Port: 8787},
		{Domain: "example.com", Port: 0},
		{Domain: "example.com", Port: 65536},
	} {
		if c.Validate() == nil {
			t.Fatalf("accepted invalid config: %+v", c)
		}
	}
	c := CaddyConfig{Domain: "Subs.Example.com", Port: 8787}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	want := "subs.example.com {\n\tencode zstd gzip\n\treverse_proxy 127.0.0.1:8787\n}\n"
	if got := c.Render(); got != want {
		t.Fatalf("rendered config:\n%s", got)
	}
}

func TestCaddyConfigAPIAndDownloadRequireAuth(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := &Server{Store: st, DataDir: t.TempDir()}
	if _, err := st.DB().Exec("INSERT INTO instance(id,name,version,key_salt) VALUES(1,'test','test',randomblob(16))"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/caddyfile", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
}

func TestWritePrivateFileUsesPrivateModeAndReplacesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "Caddyfile")
	if err := writePrivateFile(path, []byte("safe")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "safe" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestCaddyConfigRenderHasNoUserDirectives(t *testing.T) {
	c := CaddyConfig{Domain: "subs.example.com", Port: 8787}
	out := c.Render()
	if strings.Contains(out, "tls internal") || strings.Contains(out, "127.0.0.1:80") {
		t.Fatal("unexpected directives")
	}
}
