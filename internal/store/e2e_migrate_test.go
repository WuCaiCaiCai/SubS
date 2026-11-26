package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// 用 v0.1.1 发布的完整 schema 建库，再用当前代码 Open，验证真实升级路径。
func TestE2EMigrateFromV011(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v011.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil { t.Fatal(err) }
	_, err = db.Exec(`PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS instance (
 id INTEGER PRIMARY KEY CHECK(id=1), name TEXT NOT NULL DEFAULT 'my-subs',
 external_url TEXT, version TEXT NOT NULL DEFAULT 'dev', key_salt BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS admin (
 id INTEGER PRIMARY KEY CHECK(id=1), username TEXT NOT NULL,
 secret_password_hash TEXT NOT NULL, totp_secret TEXT
);
CREATE TABLE IF NOT EXISTS admin_session (
 id TEXT PRIMARY KEY, created_at TEXT NOT NULL, expires_at TEXT NOT NULL, ip TEXT
);
CREATE INDEX IF NOT EXISTS idx_session_expiry ON admin_session(expires_at);
CREATE TABLE IF NOT EXISTS airport (
 id INTEGER PRIMARY KEY, name TEXT NOT NULL, secret_url TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 1, note TEXT, refresh_interval_min INTEGER,
 last_ok_at TEXT, last_error TEXT, node_count INTEGER
);
CREATE TABLE IF NOT EXISTS server (
 id INTEGER PRIMARY KEY, name TEXT NOT NULL, address TEXT, note TEXT, tags TEXT
);
CREATE TABLE IF NOT EXISTS node (
 id INTEGER PRIMARY KEY, source_type TEXT NOT NULL CHECK(source_type IN ('airport','server')),
 source_id INTEGER NOT NULL, protocol TEXT NOT NULL CHECK(protocol IN ('hy2','vless','tuic','anytls','mieru')),
 raw_name TEXT NOT NULL, fingerprint TEXT NOT NULL, config TEXT NOT NULL,
 enabled INTEGER NOT NULL DEFAULT 1, missing INTEGER NOT NULL DEFAULT 0,
 is_new INTEGER NOT NULL DEFAULT 0, tags TEXT
);
CREATE INDEX IF NOT EXISTS idx_node_source ON node(source_type,source_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_node_fp ON node(source_type,source_id,fingerprint);
CREATE TABLE IF NOT EXISTS template (
 id INTEGER PRIMARY KEY, name TEXT NOT NULL, target TEXT NOT NULL DEFAULT 'mihomo',
 content TEXT NOT NULL, last_check TEXT
);
CREATE TABLE IF NOT EXISTS profile (
 id INTEGER PRIMARY KEY, name TEXT NOT NULL, note TEXT,
 template_id INTEGER NOT NULL REFERENCES template(id), enabled INTEGER NOT NULL DEFAULT 1,
 naming_rule TEXT NOT NULL, secret_token TEXT NOT NULL UNIQUE,
 token_hash TEXT NOT NULL UNIQUE, last_fetch_at TEXT
);
CREATE TABLE IF NOT EXISTS profile_node (
 profile_id INTEGER NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
 node_id INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
 sort_order INTEGER NOT NULL, name_override TEXT,
 PRIMARY KEY(profile_id,node_id)
);
CREATE TABLE IF NOT EXISTS profile_item (
 profile_id INTEGER NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('node','source')),
 ref_id INTEGER NOT NULL,
 source_type TEXT NOT NULL DEFAULT '',
 sort_order INTEGER NOT NULL,
 name_override TEXT,
 PRIMARY KEY(profile_id,kind,ref_id,source_type)
);
CREATE INDEX IF NOT EXISTS idx_profile_item_order ON profile_item(profile_id,sort_order);
CREATE INDEX IF NOT EXISTS idx_profile_item_source ON profile_item(kind,source_type,ref_id);
CREATE TABLE IF NOT EXISTS backup_record (
 id INTEGER PRIMARY KEY, sha256 TEXT NOT NULL UNIQUE, size_bytes INTEGER NOT NULL,
 downloaded_at TEXT, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit (
 id INTEGER PRIMARY KEY, at TEXT NOT NULL, actor TEXT, action TEXT NOT NULL, detail TEXT
);
INSERT INTO instance(id,name,version,key_salt) VALUES(1,'prod','v0.1.1',randomblob(16));
INSERT INTO template(id,name,content) VALUES(1,'tpl','');
INSERT INTO profile(id,name,template_id,naming_rule,secret_token,token_hash)
 VALUES(1,'main',1,'{}','tok','hash');
INSERT INTO node(id,source_type,source_id,protocol,raw_name,fingerprint,config)
 VALUES(1,'airport',1,'hy2','HK-01','fp1','{"protocol":"hy2","host":"hk.example.com","port":443,"auth":{"password":"pw"}}');
INSERT INTO profile_node(profile_id,node_id,sort_order) VALUES(1,1,0);`)
	if err != nil { t.Fatal(err) }
	if err := db.Close(); err != nil { t.Fatal(err) }

	st, err := Open(path)
	if err != nil { t.Fatalf("Open v0.1.1 db: %v", err) }
	defer st.Close()
	var proto, name string
	if err := st.DB().QueryRow("SELECT protocol, raw_name FROM node WHERE id=1").Scan(&proto, &name); err != nil { t.Fatal(err) }
	if proto != "hy2" || name != "HK-01" { t.Fatalf("node lost: %s %s", proto, name) }
	var items int
	if err := st.DB().QueryRow("SELECT count(*) FROM profile_item WHERE profile_id=1").Scan(&items); err != nil { t.Fatal(err) }
	if items != 1 { t.Fatalf("profile_item rows=%d want 1", items) }
	var legacy int
	if err := st.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE name='profile_node'").Scan(&legacy); err != nil { t.Fatal(err) }
	if legacy != 0 { t.Fatal("profile_node remains") }
	// 再次 Open 验证幂等。
	if err := st.Close(); err != nil { t.Fatal(err) }
	st2, err := Open(path)
	if err != nil { t.Fatalf("second Open: %v", err) }
	st2.Close()
	t.Log("v0.1.1 → current migration OK, idempotent")
}
