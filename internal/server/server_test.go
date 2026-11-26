package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddedFrontendServed(t *testing.T) {
	_, h := testServer(t)
	for _, path := range []string{"/", "/assets/index-D5mpLgCT.css", "/assets/index-CKzrGqUv.js", "/arbitrary-client-route"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("%s status=%d", path, w.Code)
		}
		if w.Body.Len() == 0 {
			t.Errorf("%s returned empty body", path)
		}
	}
}

// 仅 HTTPS 访问：开启后明文请求被拒，回环反代声明 X-Forwarded-Proto: https
// 与 /healthz 探活始终放行；开关关闭时不拦截。
func TestHTTPSOnlyMiddleware(t *testing.T) {
	s, h := testServer(t)
	db := s.Store.DB()
	if _, err := db.Exec("INSERT INTO instance(id,name,version,key_salt) VALUES(1,'test','test',randomblob(16))"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:5555"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disabled: status=%d", w.Code)
	}

	if _, err := db.Exec("UPDATE instance SET https_only=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}

	// 明文公网直连 → 426
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:5555"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUpgradeRequired {
		t.Fatalf("plain: status=%d", w.Code)
	}

	// /healthz 放行
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "203.0.113.10:5555"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("healthz: status=%d", w.Code)
	}

	// 回环反代声明 https → 放行
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("X-Forwarded-Proto", "https")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("proxy https: status=%d", w.Code)
	}
}

// 未经加密连接不能开启仅 HTTPS，否则开关一开自己就被锁在门外。
func TestUpdateSystemHTTPSOnlyRequiresSecure(t *testing.T) {
	c, s := newAPI(t)
	_ = s
	// httptest 请求 RemoteAddr 是回环但没有 X-Forwarded-Proto，视为明文。
	c.do(http.MethodPut, "/api/system", map[string]any{
		"instanceName": "my-subs", "externalUrl": "https://subs.example.com", "httpsOnly": true,
	}, nil, http.StatusBadRequest)

	var on bool
	c.do(http.MethodGet, "/api/system", nil, nil, http.StatusOK)
	_ = on
}
