package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var caddyDomainRE = regexp.MustCompile(`^(?i)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// CaddyConfig is intentionally limited to a domain and a local SubS upstream.
type CaddyConfig struct {
	Domain string `json:"domain"`
	Port   int    `json:"port"`
}

func (c CaddyConfig) Validate() error {
	c.Domain = strings.TrimSpace(c.Domain)
	if !caddyDomainRE.MatchString(c.Domain) || net.ParseIP(c.Domain) != nil {
		return fmt.Errorf("请输入有效的公网域名（不支持 IP 地址）")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("上游端口必须在 1-65535 之间")
	}
	return nil
}

func (c CaddyConfig) Render() string {
	return fmt.Sprintf("%s {\n\tencode zstd gzip\n\treverse_proxy 127.0.0.1:%d\n}\n", strings.ToLower(strings.TrimSpace(c.Domain)), c.Port)
}

func (s *Server) getCaddyConfig(w http.ResponseWriter, r *http.Request) {
	var c CaddyConfig
	err := s.Store.DB().QueryRowContext(r.Context(), "SELECT caddy_domain,caddy_port FROM instance WHERE id=1").Scan(&c.Domain, &c.Port)
	if err != nil {
		fail(w, 500, "database_error", "读取反向代理配置失败")
		return
	}
	writeJSON(w, 200, c)
}

func (s *Server) putCaddyConfig(w http.ResponseWriter, r *http.Request) {
	var c CaddyConfig
	if decodeJSON(w, r, &c, 8192) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	c.Domain = strings.TrimSpace(c.Domain)
	if err := c.Validate(); err != nil {
		fail(w, 400, "invalid_caddy_config", err.Error())
		return
	}
	if _, err := s.Store.DB().ExecContext(r.Context(), "UPDATE instance SET caddy_domain=?,caddy_port=? WHERE id=1", c.Domain, c.Port); err != nil {
		fail(w, 500, "database_error", "保存反向代理配置失败")
		return
	}
	if err := s.writeCaddyFile(c); err != nil {
		fail(w, 500, "caddy_config_failed", "配置已保存，但写入生成文件失败")
		return
	}
	writeJSON(w, 200, map[string]any{"config": c, "caddyfile": c.Render()})
}

func (s *Server) downloadCaddyFile(w http.ResponseWriter, r *http.Request) {
	var c CaddyConfig
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT caddy_domain,caddy_port FROM instance WHERE id=1").Scan(&c.Domain, &c.Port); err != nil || c.Validate() != nil {
		fail(w, 404, "caddy_config_missing", "尚未配置有效的反向代理域名")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Caddyfile"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(c.Render()))
}

func (s *Server) writeCaddyFile(c CaddyConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return writePrivateFile(filepath.Join(s.DataDir, "Caddyfile"), []byte(c.Render()))
}

func writePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".caddyfile-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
