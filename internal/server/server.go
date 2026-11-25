package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/wucai/subs"
	"github.com/wucai/subs/internal/secrets"
	"github.com/wucai/subs/internal/store"
)

var errAlreadyInitialized = errors.New("already initialized")

type ctxKey string

const sessionKey ctxKey = "session"

type Server struct {
	Store     *store.Store
	InitToken string
	Box       *secrets.Box
	DataDir   string
	DBPath    string
	// Version is the build-stamped version (ldflags -X main.version). The
	// instance table keeps its own column for compatibility with older rows.
	Version       string
	Backups       backupStore
	loginMu       sync.Mutex
	loginAttempts map[string][]time.Time
}
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Server) box() (*secrets.Box, error) {
	if s.Box == nil {
		return nil, errors.New("加密密钥未初始化")
	}
	return s.Box, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, msg string) {
	var e apiError
	e.Error.Code = code
	e.Error.Message = msg
	writeJSON(w, status, e)
}
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(recoverMiddleware)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/init", s.init)
		r.Post("/login", s.login)
		r.Post("/logout", s.logout)
		r.With(s.requireAuth).Get("/me", s.me)
	})
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/api/system", s.system)
		r.Put("/api/system/listen", s.csrf(s.listen))
		r.Put("/api/system", s.csrf(s.updateSystem))
		r.Get("/api/airports", s.listAirports)
		r.Post("/api/airports", s.csrf(s.createAirport))
		r.Get("/api/airports/{id}", s.getAirport)
		r.Put("/api/airports/{id}", s.csrf(s.updateAirport))
		r.Delete("/api/airports/{id}", s.csrf(s.deleteAirport))
		r.Post("/api/airports/{id}/refresh", s.csrf(s.refreshAirport))
		r.Post("/api/airports/{id}/confirm-new", s.csrf(s.confirmNew))
		r.Get("/api/servers", s.listServers)
		r.Post("/api/servers", s.csrf(s.createServer))
		r.Get("/api/servers/{id}", s.getServer)
		r.Put("/api/servers/{id}", s.csrf(s.updateServer))
		r.Delete("/api/servers/{id}", s.csrf(s.deleteServer))
		r.Get("/api/nodes", s.listNodes)
		r.Post("/api/nodes", s.csrf(s.createNode))
		r.Post("/api/nodes/import", s.csrf(s.importNodes))
		r.Post("/api/nodes/confirm-new", s.csrf(s.confirmAllNew))
		r.Get("/api/nodes/{id}", s.getNode)
		r.Put("/api/nodes/{id}", s.csrf(s.updateNode))
		r.Delete("/api/nodes/{id}", s.csrf(s.deleteNode))
		r.Get("/api/templates", s.listTemplates)
		r.Post("/api/templates", s.csrf(s.createTemplate))
		r.Get("/api/templates/{id}", s.getTemplate)
		r.Put("/api/templates/{id}", s.csrf(s.updateTemplate))
		r.Delete("/api/templates/{id}", s.csrf(s.deleteTemplate))
		r.Post("/api/templates/{id}/validate", s.csrf(s.validateTemplate))
		r.Get("/api/profiles", s.listProfiles)
		r.Post("/api/profiles", s.csrf(s.createProfile))
		r.Get("/api/profiles/{id}", s.getProfile)
		r.Put("/api/profiles/{id}", s.csrf(s.updateProfile))
		r.Delete("/api/profiles/{id}", s.csrf(s.deleteProfile))
		r.Put("/api/profiles/{id}/nodes", s.csrf(s.setProfileNodes))
		r.Post("/api/profiles/{id}/preview", s.csrf(s.previewProfile))
		r.Post("/api/profiles/{id}/rotate-token", s.csrf(s.rotateToken))
		r.Get("/api/profiles/{id}/naming-preview", s.namingPreview)
		r.Post("/api/backup", s.csrf(s.createBackup))
		r.Get("/api/backup/{id}", s.downloadBackup)
		r.Post("/api/restore", s.csrf(s.restoreBackup))
		r.Post("/api/destroy", s.csrf(s.destroy))
	})
	r.Get("/s/{token}", s.subscription)
	r.Get("/admin/*", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/", http.StatusTemporaryRedirect) })
	r.Get("/*", s.serveFrontend)
	return r
}
func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if strings.Contains(path, "..") || strings.Contains(path, "\\") {
		http.NotFound(w, r)
		return
	}
	name := "web/dist/" + path
	data, err := subs.WebAssets.ReadFile(name)
	if err != nil {
		data, err = subs.WebAssets.ReadFile("web/dist/index.html")
		name = "index.html"
	}
	if err != nil {
		http.Error(w, "前端资源缺失：请先执行 make build（或 cd web && npm run build）再编译二进制", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", contentTypeFor(name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Hashed asset names are immutable; index.html must always revalidate so
	// upgrades are picked up without a hard refresh.
	if strings.HasPrefix(path, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	_, _ = w.Write(data)
}

func contentTypeFor(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	}
	if ct := mime.TypeByExtension(filepath.Ext(name)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				fail(w, 500, "internal_error", "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (s *Server) initialized(ctx context.Context) (bool, error) {
	var n int
	err := s.Store.DB().QueryRowContext(ctx, "SELECT count(*) FROM admin").Scan(&n)
	return n > 0, err
}
func (s *Server) init(w http.ResponseWriter, r *http.Request) {
	if s.InitToken == "" {
		fail(w, 503, "init_disabled", "未配置初始化密钥")
		return
	}
	var in struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		InitToken string `json:"initToken"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in) != nil {
		fail(w, 400, "invalid_json", "请求格式无效")
		return
	}
	if len(in.Username) < 1 || len(in.Username) > 64 || len(in.Password) < 12 {
		fail(w, 400, "invalid_input", "用户名不能为空且密码至少 12 字符")
		return
	}
	if subtle.ConstantTimeCompare([]byte(in.InitToken), []byte(s.InitToken)) != 1 {
		fail(w, 403, "invalid_token", "初始化密钥无效")
		return
	}
	hash, err := secrets.HashPassword(in.Password)
	if err != nil {
		fail(w, 500, "hash_error", "创建凭据失败")
		return
	}
	err = s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRowContext(r.Context(), "SELECT count(*) FROM admin").Scan(&n); e != nil {
			return e
		}
		if n > 0 {
			return errAlreadyInitialized
		}
		if _, e := tx.ExecContext(r.Context(), "INSERT INTO admin(id,username,secret_password_hash) VALUES(1,?,?)", in.Username, hash); e != nil {
			return e
		}
		_, e := tx.ExecContext(r.Context(), "INSERT OR IGNORE INTO instance(id,name,listen_host,listen_port,version,key_salt) VALUES(1,'my-subs','127.0.0.1',8787,'dev',randomblob(16))")
		return e
	})
	if errors.Is(err, errAlreadyInitialized) {
		fail(w, 409, "already_initialized", "实例已初始化")
		return
	}
	if err != nil {
		fail(w, 500, "database_error", "保存初始化数据失败")
		return
	}
	s.createSession(w, r, in.Username)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.allowLogin(ip) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429, "rate_limited", "登录尝试过于频繁，请稍后再试")
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in) != nil {
		fail(w, 400, "invalid_json", "请求格式无效")
		return
	}
	var username, hash string
	err := s.Store.DB().QueryRowContext(r.Context(), "SELECT username,secret_password_hash FROM admin WHERE id=1").Scan(&username, &hash)
	if err != nil || in.Username != username || !secrets.VerifyPassword(hash, in.Password) {
		s.recordLoginFailure(ip)
		fail(w, 401, "invalid_credentials", "用户名或密码错误")
		return
	}
	s.clearLoginFailures(ip)
	s.createSession(w, r, username)
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func (s *Server) allowLogin(ip string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginAttempts == nil {
		s.loginAttempts = make(map[string][]time.Time)
	}
	cutoff := time.Now().Add(-time.Minute)
	all := s.loginAttempts[ip]
	kept := all[:0]
	for _, t := range all {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.loginAttempts[ip] = kept
	return len(kept) < 5
}
func (s *Server) recordLoginFailure(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if s.loginAttempts == nil {
		s.loginAttempts = make(map[string][]time.Time)
	}
	s.loginAttempts[ip] = append(s.loginAttempts[ip], time.Now())
}
func (s *Server) clearLoginFailures(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	delete(s.loginAttempts, ip)
}
func (s *Server) createSession(w http.ResponseWriter, r *http.Request, username string) {
	id, err := randomToken(32)
	if err != nil {
		fail(w, 500, "crypto_error", "创建会话失败")
		return
	}
	now := time.Now().UTC()
	exp := now.Add(24 * time.Hour)
	_, err = s.Store.DB().ExecContext(r.Context(), "INSERT INTO admin_session(id,created_at,expires_at,ip) VALUES(?,?,?,?)", id, now.Format(time.RFC3339), exp.Format(time.RFC3339), clientIP(r))
	if err != nil {
		fail(w, 500, "database_error", "创建会话失败")
		return
	}
	csrf, _ := randomToken(32)
	secure := r.TLS != nil
	http.SetCookie(w, &http.Cookie{Name: "subs_session", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: exp, MaxAge: 86400, Secure: secure})
	http.SetCookie(w, &http.Cookie{Name: "subs_csrf", Value: csrf, Path: "/", SameSite: http.SameSiteLaxMode, Expires: exp, MaxAge: 86400, Secure: secure})
	writeJSON(w, 200, map[string]string{"username": username, "csrfToken": csrf})
}
func (s *Server) session(r *http.Request) (string, bool) {
	c, e := r.Cookie("subs_session")
	if e != nil {
		return "", false
	}
	var exp string
	err := s.Store.DB().QueryRowContext(r.Context(), "SELECT expires_at FROM admin_session WHERE id=?", c.Value).Scan(&exp)
	if err != nil {
		return "", false
	}
	t, e := time.Parse(time.RFC3339, exp)
	return c.Value, e == nil && time.Now().Before(t)
}
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.session(r)
		if !ok {
			fail(w, 401, "unauthorized", "请先登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, id)))
	})
}
func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, e := r.Cookie("subs_csrf")
		header := r.Header.Get("X-CSRF-Token")
		if e != nil || header == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			fail(w, 403, "csrf", "CSRF 校验失败")
			return
		}
		next(w, r)
	}
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	var username string
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT username FROM admin WHERE id=1").Scan(&username); err != nil {
		fail(w, 500, "database_error", "读取用户失败")
		return
	}
	csrf, _ := randomToken(32)
	http.SetCookie(w, &http.Cookie{Name: "subs_csrf", Value: csrf, Path: "/", SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: 86400})
	writeJSON(w, 200, map[string]string{"username": username, "csrfToken": csrf})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if id, ok := s.session(r); ok {
		_, _ = s.Store.DB().ExecContext(r.Context(), "DELETE FROM admin_session WHERE id=?", id)
	}
	http.SetCookie(w, &http.Cookie{Name: "subs_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "subs_csrf", Value: "", Path: "/", SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	var name, host, dbVersion string
	var port int
	var external sql.NullString
	err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT name,listen_host,listen_port,version,external_url FROM instance WHERE id=1").
		Scan(&name, &host, &port, &dbVersion, &external)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		fail(w, 500, "database_error", "读取系统状态失败")
		return
	}
	count := func(query string) int64 {
		var n int64
		_ = s.Store.DB().QueryRowContext(r.Context(), query).Scan(&n)
		return n
	}
	lastBackup := ""
	_ = s.Store.DB().QueryRowContext(r.Context(),
		"SELECT COALESCE(max(created_at),'') FROM backup_record").Scan(&lastBackup)
	var diskUsed int64
	if info, e := os.Stat(s.DBPath); e == nil {
		diskUsed = info.Size()
	}
	appVersion := s.Version
	if appVersion == "" {
		appVersion = dbVersion
	}
	writeJSON(w, 200, map[string]any{
		"instanceName":  name,
		"version":       appVersion,
		"listenHost":    host,
		"listenPort":    port,
		"externalUrl":   external.String,
		"diskUsedBytes": diskUsed,
		"airports":      count("SELECT count(*) FROM airport"),
		"servers":       count("SELECT count(*) FROM server"),
		"nodes":         count("SELECT count(*) FROM node"),
		"newNodes":      count("SELECT count(*) FROM node WHERE is_new=1"),
		"missingNodes":  count("SELECT count(*) FROM node WHERE missing=1"),
		"profiles":      count("SELECT count(*) FROM profile"),
		"lastBackupAt":  lastBackup,
	})
}
func (s *Server) updateSystem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		InstanceName string `json:"instanceName"`
		ExternalURL  string `json:"externalUrl"`
	}
	if decodeJSON(w, r, &in, 8192) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	name := strings.TrimSpace(in.InstanceName)
	if name == "" || len(name) > 64 {
		fail(w, 400, "invalid_input", "实例名需为 1-64 个字符")
		return
	}
	ext := strings.TrimSpace(in.ExternalURL)
	if ext != "" {
		u, err := url.Parse(ext)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail(w, 400, "invalid_url", "外部地址需为完整的 http(s) URL")
			return
		}
		if strings.ContainsAny(ext, " \t\n") {
			fail(w, 400, "invalid_url", "外部地址不能包含空白字符")
			return
		}
		ext = strings.TrimSuffix(ext, "/")
	}
	if _, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE instance SET name=?,external_url=? WHERE id=1", name, nullString(ext)); err != nil {
		fail(w, 500, "database_error", "保存设置失败")
		return
	}
	writeJSON(w, 200, map[string]any{"instanceName": name, "externalUrl": ext})
}

func (s *Server) listen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in) != nil || strings.TrimSpace(in.Host) == "" || in.Port < 1024 || in.Port > 65535 {
		fail(w, 400, "invalid_listen", "监听地址或端口无效")
		return
	}
	err := s.Store.Write(r.Context(), func(tx *sql.Tx) error {
		_, e := tx.ExecContext(r.Context(), "UPDATE instance SET listen_host=?,listen_port=? WHERE id=1", in.Host, in.Port)
		return e
	})
	if err != nil {
		fail(w, 500, "database_error", "保存监听设置失败")
		return
	}
	writeJSON(w, 200, map[string]string{"host": in.Host, "port": strconv.Itoa(in.Port)})
}
