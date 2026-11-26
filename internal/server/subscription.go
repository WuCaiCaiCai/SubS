package server

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/WuCaiCaiCai/SubS/internal/profile"
	"github.com/WuCaiCaiCai/SubS/internal/render"
	"github.com/go-chi/chi/v5"
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
	// 客户端 UA 是默认协商方式；显式 ?target= 覆盖它，方便网页端生成
	// 「另存为文件」或让用户手动指定格式。
	target := targetFromUA(r.UserAgent())
	if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target"))); q != "" {
		if !isTarget(q) {
			fail(w, 400, "invalid_target", "未知输出目标")
			return
		}
		target = q
	}
	asFile := r.URL.Query().Get("download") == "1"
	res, err := s.renderProfile(r, profileID, target)
	if err != nil {
		notFound(w)
		return
	}
	_, _ = s.Store.DB().ExecContext(r.Context(),
		"UPDATE profile SET last_fetch_at=? WHERE id=?", nowRFC3339(), profileID)

	h := w.Header()
	h.Set("Cache-Control", "private, no-store")
	h.Set("Content-Disposition", contentDisposition(target, asFile))
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

// bundleFile 是打包下载里的一份产物。
type bundleFile struct {
	target string
	name   string
	note   string
}

var bundleFiles = []bundleFile{
	{render.TargetMihomo, "subs.yaml", "Mihomo / Clash Meta 配置"},
	{render.TargetSingBox, "subs.json", "sing-box 配置"},
	{render.TargetLoon, "subs.conf", "Loon 配置"},
}

// subscriptionBundle renders every supported client into a single zip so the
// config can be moved to a machine that cannot reach this server.
func (s *Server) subscriptionBundle(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		notFound(w)
		return
	}
	var profileID int64
	var profileName string
	err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT id, name FROM profile WHERE token_hash=? AND enabled=1", profile.HashToken(token)).
		Scan(&profileID, &profileName)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		notFound(w)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var summary strings.Builder
	summary.WriteString("SubS 订阅包\n")
	summary.WriteString("编排：" + profileName + "\n")
	summary.WriteString("生成时间：" + nowRFC3339() + "\n\n")
	var skipped []string
	for _, f := range bundleFiles {
		res, err := s.renderProfile(r, profileID, f.target)
		if err != nil {
			notFound(w)
			return
		}
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			fail(w, 500, "zip_error", "打包失败")
			return
		}
		if _, err := fw.Write([]byte(res.Body)); err != nil {
			fail(w, 500, "zip_error", "打包失败")
			return
		}
		fmt.Fprintf(&summary, "%s\t%d 字节\t%s\n", f.name, len(res.Body), f.note)
		if len(res.Skipped) > 0 {
			skipped = res.Skipped
		}
	}
	if len(skipped) > 0 {
		summary.WriteString("\n以下节点在部分客户端没有对应协议，已跳过：\n  " + strings.Join(skipped, "\n  ") + "\n")
	}
	if fw, err := zw.CreateHeader(&zip.FileHeader{Name: "README.txt", Method: zip.Deflate, Modified: time.Now()}); err == nil {
		_, _ = fw.Write([]byte(summary.String()))
	}
	if err := zw.Close(); err != nil {
		fail(w, 500, "zip_error", "打包失败")
		return
	}

	_, _ = s.Store.DB().ExecContext(r.Context(),
		"UPDATE profile SET last_fetch_at=? WHERE id=?", nowRFC3339(), profileID)

	h := w.Header()
	h.Set("Cache-Control", "private, no-store")
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", `attachment; filename="`+bundleName(profileName)+`"; filename*=UTF-8''`+url.PathEscape(profileName+"-subs.zip"))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// bundleName keeps the zip filename ASCII-safe.
func bundleName(profileName string) string {
	var b strings.Builder
	for _, r := range profileName {
		switch {
		case r < 128 && (r == '-' || r == '_' || r == '.' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')):
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" || name == "subs" {
		return "subs.zip"
	}
	return name + "-subs.zip"
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNotFound)
}

// isTarget reports whether name is a known render target.
func isTarget(name string) bool {
	return name == render.TargetMihomo || name == render.TargetSingBox || name == render.TargetLoon
}

// contentDisposition picks a filename that matches the negotiated format.
func contentDisposition(target string, asFile bool) string {
	name := map[string]string{
		render.TargetMihomo:  "subs.yaml",
		render.TargetSingBox: "subs.json",
		render.TargetLoon:    "subs.conf",
	}[target]
	if asFile {
		return `attachment; filename="` + name + `"`
	}
	return `inline; filename="` + name + `"`
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
