package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WuCaiCaiCai/SubS/internal/backup"
	"github.com/WuCaiCaiCai/SubS/internal/store"
	"github.com/go-chi/chi/v5"
)

// backupTTL bounds how long a generated backup stays downloadable.
const backupTTL = 30 * time.Minute

type backupTicket struct {
	path      string
	sha256    string
	createdAt time.Time
}

// Server keeps pending backups in memory; they are one-shot and expire.
type backupStore struct {
	mu      sync.Mutex
	tickets map[string]backupTicket
}

func (b *backupStore) put(id string, t backupTicket) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tickets == nil {
		b.tickets = map[string]backupTicket{}
	}
	b.tickets[id] = t
}

func (b *backupStore) get(id string) (backupTicket, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.tickets[id]
	if !ok || time.Since(t.createdAt) > backupTTL {
		return backupTicket{}, false
	}
	return t, true
}

func (b *backupStore) drop(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.tickets, id)
}

// createBackup snapshots the database into an age-encrypted .sbox archive.
// The archive layout follows docs/技术架构与选型.md: metadata.json + db.sqlite.
func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if decodeJSON(w, r, &in, 8<<10) != nil || len(in.Password) < 12 {
		fail(w, 400, "weak_password", "备份密码至少 12 位")
		return
	}
	// Snapshot under the store backup lock so writes are paused briefly.
	var snapshot []byte
	var caddySnapshot []byte
	err := s.Store.WithBackup(r.Context(), func(db *sql.DB) error {
		path, err := s.snapshotDB(r)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		snapshot, err = os.ReadFile(path)
		if err != nil {
			return err
		}
		var c CaddyConfig
		if err := db.QueryRowContext(r.Context(), "SELECT caddy_domain,caddy_port FROM instance WHERE id=1").Scan(&c.Domain, &c.Port); err != nil {
			return err
		}
		if c.Validate() == nil {
			caddySnapshot = []byte(c.Render())
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, store.ErrBackupInProgress) {
			fail(w, 409, "backup_in_progress", "已有备份任务在进行")
			return
		}
		fail(w, 500, "backup_failed", "生成备份快照失败")
		return
	}

	var meta struct {
		InstanceName string `json:"instanceName"`
		Version      string `json:"version"`
		CreatedAt    string `json:"createdAt"`
	}
	_ = s.Store.DB().QueryRowContext(r.Context(),
		"SELECT name, version FROM instance WHERE id=1").Scan(&meta.InstanceName, &meta.Version)
	meta.CreatedAt = nowRFC3339()
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")

	archiveFiles := map[string][]byte{"metadata.json": metaJSON, "db.sqlite": snapshot}
	if caddySnapshot != nil {
		archiveFiles["caddy/Caddyfile"] = caddySnapshot
	}
	archive, err := buildArchive(archiveFiles, in.Password)
	if err != nil {
		fail(w, 500, "backup_failed", "加密备份失败")
		return
	}
	sum := sha256.Sum256(archive)
	sha := hex.EncodeToString(sum[:])

	id, err := randomToken(18)
	if err != nil {
		fail(w, 500, "backup_failed", "生成下载标识失败")
		return
	}
	dir := filepath.Join(s.DataDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fail(w, 500, "backup_failed", "写入备份失败")
		return
	}
	path := filepath.Join(dir, "backup-"+id+".sbox")
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		fail(w, 500, "backup_failed", "写入备份失败")
		return
	}
	s.Backups.put(id, backupTicket{path: path, sha256: sha, createdAt: time.Now()})
	if _, err := s.Store.DB().ExecContext(r.Context(),
		"INSERT INTO backup_record(sha256,size_bytes,created_at) VALUES(?,?,?)",
		sha, len(archive), nowRFC3339()); err != nil {
		// Recording is best-effort; the archive itself is valid.
		_ = err
	}
	writeJSON(w, 201, map[string]any{"downloadId": id, "sha256": sha, "sizeBytes": len(archive)})
}

// snapshotDB uses VACUUM INTO for a consistent copy while WAL is active.
func (s *Server) snapshotDB(r *http.Request) (string, error) {
	tmp, err := os.CreateTemp(filepath.Join(s.DataDir, "backups"), "snapshot-*.db")
	if err != nil {
		if mkErr := os.MkdirAll(filepath.Join(s.DataDir, "backups"), 0o700); mkErr != nil {
			return "", mkErr
		}
		tmp, err = os.CreateTemp(filepath.Join(s.DataDir, "backups"), "snapshot-*.db")
		if err != nil {
			return "", err
		}
	}
	path := tmp.Name()
	tmp.Close()
	os.Remove(path)
	if _, err := s.Store.DB().ExecContext(r.Context(), "VACUUM INTO ?", path); err != nil {
		return "", err
	}
	return path, nil
}

func buildArchive(files map[string][]byte, password string) ([]byte, error) {
	return backup.Build(files, password)
}

func readArchive(data []byte, password string) (map[string][]byte, error) {
	return backup.Read(data, password)
}

func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ticket, ok := s.Backups.get(id)
	if !ok {
		notFound(w)
		return
	}
	f, err := os.Open(ticket.path)
	if err != nil {
		notFound(w)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		notFound(w)
		return
	}
	if _, err := s.Store.DB().ExecContext(r.Context(),
		"UPDATE backup_record SET downloaded_at=? WHERE sha256=?", nowRFC3339(), ticket.sha256); err != nil {
		_ = err
	}
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="subs-backup-%s.sbox"`, ticket.sha256[:12]))
	h.Set("Content-Length", fmt.Sprint(info.Size()))
	h.Set("Cache-Control", "no-store")
	h.Set("X-Backup-SHA256", ticket.sha256)
	w.WriteHeader(200)
	_, _ = io.Copy(w, f)
}

// restoreBackup validates an uploaded archive and swaps the database in place.
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		fail(w, 400, "invalid_upload", "上传内容格式错误")
		return
	}
	password := r.FormValue("password")
	if len(password) < 12 {
		fail(w, 400, "weak_password", "备份密码至少 12 位")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "invalid_upload", "缺少备份文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 512<<20))
	if err != nil {
		fail(w, 400, "invalid_upload", "读取上传文件失败")
		return
	}
	files, err := readArchive(data, password)
	if err != nil {
		fail(w, 400, "restore_failed", err.Error())
		return
	}
	tmp := filepath.Join(s.DataDir, "restore-"+time.Now().Format("20060102150405")+".db")
	if err := os.WriteFile(tmp, files["db.sqlite"], 0o600); err != nil {
		fail(w, 500, "restore_failed", "写入恢复文件失败")
		return
	}
	// Validate the restored database opens and has the expected schema.
	if err := s.replaceDatabase(r, tmp); err != nil {
		_ = os.Remove(tmp)
		fail(w, 500, "restore_failed", "替换数据库失败: "+err.Error())
		return
	}
	var restored CaddyConfig
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT caddy_domain,caddy_port FROM instance WHERE id=1").Scan(&restored.Domain, &restored.Port); err != nil {
		fail(w, 500, "restore_failed", "数据库已恢复，但读取反向代理配置失败")
		return
	}
	if restored.Validate() == nil {
		if err := s.writeCaddyFile(restored); err != nil {
			fail(w, 500, "restore_failed", "数据库已恢复，但写入 Caddyfile 失败")
			return
		}
	} else {
		_ = os.Remove(filepath.Join(s.DataDir, "Caddyfile"))
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "恢复完成，请重新登录；如需启用 Caddy，请 SSH 运行配置脚本"})
}

// destroy wipes the instance after verifying the confirmation phrase and a
// downloaded backup checksum.
func (s *Server) destroy(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ConfirmPhrase string `json:"confirmPhrase"`
		BackupSHA256  string `json:"backupSha256"`
		DryRun        bool   `json:"dryRun"`
	}
	if decodeJSON(w, r, &in, 8<<10) != nil {
		fail(w, 400, "invalid_input", "请求体格式错误")
		return
	}
	var name string
	if err := s.Store.DB().QueryRowContext(r.Context(), "SELECT name FROM instance WHERE id=1").Scan(&name); err != nil {
		fail(w, 500, "database_error", "读取实例信息失败")
		return
	}
	if strings.TrimSpace(in.ConfirmPhrase) != name {
		fail(w, 403, "confirm_mismatch", "确认短语与实例名不一致")
		return
	}
	if in.DryRun {
		writeJSON(w, 200, map[string]any{"ok": true, "dryRun": true})
		return
	}
	sha := strings.TrimSpace(in.BackupSHA256)
	if sha == "" {
		fail(w, 409, "backup_required", "请先下载备份并提交其 sha256 校验值")
		return
	}
	var downloaded sql.NullString
	if err := s.Store.DB().QueryRowContext(r.Context(),
		"SELECT downloaded_at FROM backup_record WHERE sha256=?", sha).Scan(&downloaded); err != nil {
		fail(w, 409, "backup_not_found", "未找到匹配的备份下载记录")
		return
	}
	if !downloaded.Valid {
		fail(w, 409, "backup_not_downloaded", "该备份尚未下载，请先下载")
		return
	}
	if err := s.wipe(r); err != nil {
		fail(w, 500, "destroy_failed", "销毁失败: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "实例已销毁"})
}

// replaceDatabase swaps in a validated database file.
func (s *Server) replaceDatabase(r *http.Request, newPath string) error {
	probe, err := store.Open(newPath)
	if err != nil {
		return errors.New("备份数据库无法打开")
	}
	var instanceName string
	err = probe.DB().QueryRowContext(r.Context(), "SELECT name FROM instance WHERE id=1").Scan(&instanceName)
	probe.Close()
	if err != nil {
		return errors.New("备份数据库结构不完整")
	}
	return s.Store.ReplaceWith(r.Context(), newPath, s.DBPath)
}

// wipe removes all data for the instance, leaving a final backup behind.
func (s *Server) wipe(r *http.Request) error {
	if err := s.finalBackup(r); err != nil {
		return err
	}
	if err := s.Store.DB().Close(); err != nil {
		return err
	}
	for _, name := range []string{"subs.db", "subs.db-wal", "subs.db-shm", "master.key"} {
		_ = os.Remove(filepath.Join(s.DataDir, name))
	}
	return nil
}

// finalBackup writes an unencrypted-to-disk snapshot that the user can still
// download from /var/lib/subs/final-backup.sbox before uninstalling.
func (s *Server) finalBackup(r *http.Request) error {
	path, err := s.snapshotDB(r)
	if err != nil {
		return err
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.DataDir, "final-backup.db"), data, 0o600); err != nil {
		return err
	}
	return nil
}
