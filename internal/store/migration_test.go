package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenMigratesLegacyListenColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE instance (
		id INTEGER PRIMARY KEY CHECK(id=1), name TEXT NOT NULL,
		listen_host TEXT NOT NULL DEFAULT '127.0.0.1',
		listen_port INTEGER NOT NULL DEFAULT 8787,
		external_url TEXT, version TEXT NOT NULL, key_salt BLOB NOT NULL
	);
	INSERT INTO instance(id,name,listen_host,listen_port,version,key_salt)
	VALUES(1,'existing','0.0.0.0',9000,'test',randomblob(16));`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.DB().Query(`PRAGMA table_info(instance)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "listen_host" || name == "listen_port" {
			t.Fatalf("legacy listen column %q remains", name)
		}
	}
	var name string
	if err := store.DB().QueryRow("SELECT name FROM instance WHERE id=1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "existing" {
		t.Fatalf("instance name changed during migration: %q", name)
	}
}
