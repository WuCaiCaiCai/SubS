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
