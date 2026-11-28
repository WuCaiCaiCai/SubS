package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

var ErrBackupInProgress = errors.New("backup in progress")

type Store struct {
	db       *sql.DB
	writeMu  sync.Mutex
	backupMu sync.RWMutex
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}
	if err = migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) DB() *sql.DB  { return s.db }
func (s *Store) Close() error { return s.db.Close() }

// Write serializes mutations; an active consistent backup rejects new writes.
func (s *Store) Write(ctx context.Context, fn func(*sql.Tx) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if !s.backupMu.TryRLock() {
		return ErrBackupInProgress
	}
	defer s.backupMu.RUnlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) WithBackup(ctx context.Context, fn func(*sql.DB) error) error {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	return fn(s.db)
}

// ReplaceWith closes the current database, moves the replacement file into
// place and reopens. Callers must ensure no other request is mid-flight.
func (s *Store) ReplaceWith(ctx context.Context, newPath, dbPath string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	if err := s.db.Close(); err != nil {
		return err
	}
	// WAL and shared-memory files belong to the old database.
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}
	if err := os.Rename(newPath, dbPath); err != nil {
		return err
	}
	reopened, err := Open(dbPath)
	if err != nil {
		return err
	}
	s.db = reopened.db
	return nil
}

// Reset closes the current database, removes its files, and opens a fresh empty
// database at dbPath so a running server can cleanly return to uninitialized state.
func (s *Store) Reset(dbPath string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	if err := s.db.Close(); err != nil {
		return err
	}
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		_ = os.Remove(path)
	}
	reopened, err := Open(dbPath)
	if err != nil {
		return err
	}
	s.db = reopened.db
	return nil
}

// migrate moves data written by older versions into the current shape. Every
// step must be idempotent: it runs on each start.
func migrateListenColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(instance)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			_ = rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !columns["listen_host"] && !columns["listen_port"] {
		return nil
	}
	_, err = db.Exec(`BEGIN; ALTER TABLE instance DROP COLUMN listen_host; ALTER TABLE instance DROP COLUMN listen_port; COMMIT;`)
	if err != nil {
		_, _ = db.Exec(`ROLLBACK`)
	}
	return err
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			_ = rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func migrateProtocolConstraint(db *sql.DB) error {
	var ddl string
	// 先完全读完并关闭游标，再在同一连接上开启写事务；否则 SQLite 的读写
	// 连接争抢文件锁会导致 deadlock（旧库升级时进程直接崩溃）。
	if err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='node'").Scan(&ddl); err != nil {
		return err
	}
	if strings.Contains(ddl, "'trojan'") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		"ALTER TABLE node RENAME TO node_legacy_protocol",
		"CREATE TABLE node (" +
			"id INTEGER PRIMARY KEY, source_type TEXT NOT NULL CHECK(source_type IN ('airport','server'))," +
			"source_id INTEGER NOT NULL, protocol TEXT NOT NULL CHECK(protocol IN ('hy2','vless','tuic','anytls','mieru','ss','vmess','trojan'))," +
			"raw_name TEXT NOT NULL, fingerprint TEXT NOT NULL, config TEXT NOT NULL," +
			"enabled INTEGER NOT NULL DEFAULT 1, missing INTEGER NOT NULL DEFAULT 0,is_new INTEGER NOT NULL DEFAULT 0,tags TEXT)",
		"INSERT INTO node SELECT * FROM node_legacy_protocol",
		"DROP TABLE node_legacy_protocol",
		"CREATE INDEX IF NOT EXISTS idx_node_source ON node(source_type,source_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_node_fp ON node(source_type,source_id,fingerprint)",
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q); err != nil {
			return fmt.Errorf("migrate protocol %q: %w", q, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate protocol commit: %w", err)
	}
	return nil
}

func migrateProfileItem(db *sql.DB) error {
	// profile_node 是 v1 的遗留表。它带有 REFERENCES node(id) 外键，必须
	// 在 node 表重建（ALTER TABLE ... RENAME）之前完成迁移并删除：否则
	// 外键会跟随 RENAME 指向 node_legacy_protocol，随后 DROP 造成孤儿引用，
	// 下一次查询就报 no such table（旧库升级时曾因此崩溃）。
	var exists int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='profile_node'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO profile_item(profile_id,kind,ref_id,source_type,sort_order,name_override)
		SELECT profile_id,'node',node_id,'',sort_order,name_override FROM profile_node`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE profile_node`); err != nil {
		return err
	}
	return tx.Commit()
}

func migrate(db *sql.DB) error {
	if err := migrateListenColumns(db); err != nil {
		return fmt.Errorf("listen columns: %w", err)
	}
	// v1 → v2 遗留表迁移必须先行：它会 DROP 引用 node 的表，避免重建 node
	// 时外键被 ALTER RENAME 拖到临时表名下（见 migrateProfileItem 注释）。
	if err := migrateProfileItem(db); err != nil {
		return fmt.Errorf("profile item: %w", err)
	}
	// Rebuild node because SQLite CHECK constraints cannot be widened in place.
	if err := migrateProtocolConstraint(db); err != nil {
		return fmt.Errorf("protocol constraint: %w", err)
	}
	// https_only 替代旧版 Caddy 配置列（caddy_domain/caddy_port 已不再使用，
	// 旧库中遗留的列保留不读，避免无法回退）。
	if err := ensureColumn(db, "instance", "https_only", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("https only: %w", err)
	}
	// slug：订阅名的 URL 路径段，允许为空（空 = 兼容旧链接 /s/<token>）。
	if err := ensureColumn(db, "profile", "slug", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("profile slug: %w", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_profile_slug ON profile(slug) WHERE slug<>''`); err != nil {
		return fmt.Errorf("profile slug index: %w", err)
	}
	// 自动丢弃旧版本遗留的缺失节点与待确认标记，无需人工确认。
	if _, err := db.Exec(`DELETE FROM profile_item WHERE kind='node' AND ref_id IN (SELECT id FROM node WHERE missing=1)`); err != nil {
		return fmt.Errorf("cleanup missing profile items: %w", err)
	}
	if _, err := db.Exec(`DELETE FROM node WHERE missing=1`); err != nil {
		return fmt.Errorf("cleanup missing nodes: %w", err)
	}
	if _, err := db.Exec(`UPDATE node SET is_new=0 WHERE is_new=1`); err != nil {
		return fmt.Errorf("clear is_new flags: %w", err)
	}
	return nil
}
