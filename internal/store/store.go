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
	rows, err := db.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name='node'")
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return rows.Err()
	}
	var ddl string
	if err := rows.Scan(&ddl); err != nil {
		return err
	}
	if strings.Contains(ddl, "'trojan'") {
		return nil
	}
	_, err = db.Exec(`BEGIN;
ALTER TABLE node RENAME TO node_legacy_protocol;
CREATE TABLE node (
 id INTEGER PRIMARY KEY, source_type TEXT NOT NULL CHECK(source_type IN ('airport','server')),
 source_id INTEGER NOT NULL, protocol TEXT NOT NULL CHECK(protocol IN ('hy2','vless','tuic','anytls','mieru','ss','vmess','trojan')),
 raw_name TEXT NOT NULL, fingerprint TEXT NOT NULL, config TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 1, missing INTEGER NOT NULL DEFAULT 0,is_new INTEGER NOT NULL DEFAULT 0,tags TEXT);
INSERT INTO node SELECT * FROM node_legacy_protocol;
DROP TABLE node_legacy_protocol;
CREATE INDEX IF NOT EXISTS idx_node_source ON node(source_type,source_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_fp ON node(source_type,source_id,fingerprint);
COMMIT;`)
	return err
}

func migrate(db *sql.DB) error {
	if err := migrateListenColumns(db); err != nil {
		return err
	}
	// Rebuild node because SQLite CHECK constraints cannot be widened in place.
	if err := migrateProtocolConstraint(db); err != nil {
		return err
	}
	if err := ensureColumn(db, "instance", "caddy_domain", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "instance", "caddy_port", "INTEGER NOT NULL DEFAULT 8787"); err != nil {
		return err
	}
	// v1 → v2: per-node selection becomes profile_item(kind='node').
	// profile_node is emptied after copying so the step is a no-op next time.
	_, err := db.Exec(`
BEGIN;
INSERT OR IGNORE INTO profile_item(profile_id,kind,ref_id,source_type,sort_order,name_override)
 SELECT profile_id,'node',node_id,'',sort_order,name_override FROM profile_node;
DELETE FROM profile_node;
COMMIT;`)
	return err
}
