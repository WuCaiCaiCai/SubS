package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestOpenMigratesLegacyProtocolConstraint 复现历史库升级场景：旧的 node 表
// 协议 CHECK 只允许五种，升级后要保留现有行并放宽到八种。这也覆盖了曾经
// 导致启动 deadlock 的迁移路径。
func TestOpenMigratesLegacyProtocolConstraint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-node.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE node (
		id INTEGER PRIMARY KEY, source_type TEXT NOT NULL,
		source_id INTEGER NOT NULL, protocol TEXT NOT NULL CHECK(protocol IN ('hy2','vless','tuic','anytls','mieru')),
		raw_name TEXT NOT NULL, fingerprint TEXT NOT NULL, config TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1, missing INTEGER NOT NULL DEFAULT 0,
		is_new INTEGER NOT NULL DEFAULT 0, tags TEXT);
	INSERT INTO node(id,source_type,source_id,protocol,raw_name,fingerprint,config)
	VALUES(1,'server',1,'hy2','legacy-node','fp','{}');
	-- v1 遗留表：带 REFERENCES node(id) 外键，正是历史上导致迁移崩溃的表。
	CREATE TABLE profile (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL, note TEXT,
		template_id INTEGER NOT NULL, enabled INTEGER NOT NULL DEFAULT 1,
		naming_rule TEXT NOT NULL, secret_token TEXT NOT NULL UNIQUE,
		token_hash TEXT NOT NULL UNIQUE, last_fetch_at TEXT);
	CREATE TABLE profile_node (
		profile_id INTEGER NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
		node_id INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
		sort_order INTEGER NOT NULL, name_override TEXT,
		PRIMARY KEY(profile_id,node_id));
	INSERT INTO profile(id,name,template_id,naming_rule,secret_token,token_hash)
	VALUES(1,'p',0,'{}','t','h');
	INSERT INTO profile_node(profile_id,node_id,sort_order) VALUES(1,1,0);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var proto string
	if err := st.DB().QueryRow("SELECT protocol FROM node WHERE id=1").Scan(&proto); err != nil {
		t.Fatal(err)
	}
	if proto != "hy2" {
		t.Fatalf("legacy node lost during migration: %q", proto)
	}
	// v1 → v2：profile_node 的引用应迁入 profile_item，遗留表被删除。
	var items int
	if err := st.DB().QueryRow("SELECT count(*) FROM profile_item WHERE profile_id=1 AND kind='node' AND ref_id=1").Scan(&items); err != nil {
		t.Fatal(err)
	}
	if items != 1 {
		t.Fatalf("profile_node not migrated to profile_item: %d", items)
	}
	var legacy int
	if err := st.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='profile_node'").Scan(&legacy); err != nil {
		t.Fatal(err)
	}
	if legacy != 0 {
		t.Fatal("legacy profile_node table remains after migration")
	}
	// 新的 CHECK 应允许之前被拒绝的协议写入。
	if _, err := st.DB().Exec(`INSERT INTO node(source_type,source_id,protocol,raw_name,fingerprint,config)
		VALUES('server',1,'trojan','t','fp2','{}')`); err != nil {
		t.Fatalf("widened protocol constraint rejected trojan: %v", err)
	}
}

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
