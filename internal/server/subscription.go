package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/wucai/subs/internal/profile"
	"github.com/wucai/subs/internal/render"
)

// subscription serves the public, token-authenticated profile endpoint.
// Missing, disabled and unknown tokens all return the same 404 to prevent
// enumeration.
func (s *Server) subscription(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		notFound(w)
		return
	}
	var profileID int64
	err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT id FROM profile WHERE token_hash=? AND enabled=1", profile.HashToken(token)).Scan(&profileID)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		notFound(w)
		return
	}
	target := targetFromUA(r.UserAgent())
	res, err := s.renderProfile(r, profileID, target)
	if err != nil {
		notFound(w)
		return
	}
	_, _ = s.Store.DB().ExecContext(r.Context(),
		"UPDATE profile SET last_fetch_at=? WHERE id=?", nowRFC3339(), profileID)

	h := w.Header()
	h.Set("Cache-Control", "private, no-store")
	h.Set("Content-Disposition", `inline; filename="subs"`)
	h.Set("Profile-Update-Interval", "24")
	// 节点名可能含非 ASCII 字符，HTTP 头要求 ASCII：先 URL 编码。
	h.Set("X-Skipped-Count", strconv.Itoa(len(res.Skipped)))
	if len(res.Skipped) > 0 {
		h.Set("X-Skipped-Nodes", url.QueryEscape(strings.Join(res.Skipped, ",")))
	}
	switch target {
	case render.TargetSingBox:
		h.Set("Content-Type", "application/json; charset=utf-8")
	case render.TargetLoon:
		h.Set("Content-Type", "text/plain; charset=utf-8")
	default:
		h.Set("Content-Type", "text/yaml; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(res.Body))
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNotFound)
}

// targetFromUA maps a client User-Agent to a render target.
func targetFromUA(ua string) string {
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "sing-box") || strings.Contains(l, "singbox"):
		return render.TargetSingBox
	case strings.Contains(l, "loon") || strings.Contains(l, "surge"):
		return render.TargetLoon
	default:
		return render.TargetMihomo
	}
}

var _ = fmt.Sprintf
