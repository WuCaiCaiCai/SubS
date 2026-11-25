package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchemaAndWAL(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var mode string
	if err := s.DB().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode=%q", mode)
	}
	var tables int
	if err := s.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='admin_session'").Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("session table count=%d err=%v", tables, err)
	}
}

func TestWriteTransactionRollsBack(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	wantErr := context.Canceled
	err = s.Write(context.Background(), func(tx *sql.Tx) error {
		_, e := tx.Exec("INSERT INTO instance(id,name,listen_host,listen_port,version,key_salt) VALUES(1,'x','127.0.0.1',8787,'test',randomblob(16))")
		if e != nil {
			return e
		}
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
	var n int
	if err := s.DB().QueryRow("SELECT count(*) FROM instance").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("transaction was not rolled back: rows=%d", n)
	}
}
