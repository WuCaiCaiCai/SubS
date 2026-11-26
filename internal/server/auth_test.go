package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/WuCaiCaiCai/SubS/internal/store"
)

func testServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := &Server{Store: db, InitToken: "init-secret"}
	return s, s.Router()
}
func TestInitLoginMeLogout(t *testing.T) {
	s, h := testServer(t)
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "correct horse battery staple", "initToken": "init-secret"})
	r := httptest.NewRequest(http.MethodPost, "/api/auth/init", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("init status=%d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["csrfToken"] == "" {
		t.Fatal("missing csrf token")
	}
	cookies := w.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected session and csrf cookies, got %d", len(cookies))
	}
	me := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	for _, c := range cookies {
		me.AddCookie(c)
	}
	mw := httptest.NewRecorder()
	h.ServeHTTP(mw, me)
	if mw.Code != 200 {
		t.Fatalf("me status=%d", mw.Code)
	}
	loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "correct horse battery staple"})
	lr := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	lw := httptest.NewRecorder()
	h.ServeHTTP(lw, lr)
	if lw.Code != 200 {
		t.Fatalf("login status=%d body=%s", lw.Code, lw.Body.String())
	}
	_ = s
}
func TestLoginRateLimit(t *testing.T) {
	s, h := testServer(t)
	body := []byte(`{"username":"admin","password":"wrong"}`)
	for i := 0; i < 5; i++ {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status=%d", i+1, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("sixth attempt status=%d", w.Code)
	}
	_ = s
}

func TestInitRequiresToken(t *testing.T) {
	_, h := testServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/auth/init", bytes.NewBufferString(`{"username":"admin","password":"correct horse battery staple","initToken":"wrong"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
}
