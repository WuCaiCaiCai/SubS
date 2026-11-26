package store

const schema = `
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS instance (
 id INTEGER PRIMARY KEY CHECK(id=1), name TEXT NOT NULL DEFAULT 'my-subs',
 external_url TEXT, caddy_domain TEXT NOT NULL DEFAULT '', caddy_port INTEGER NOT NULL DEFAULT 8787,
 version TEXT NOT NULL DEFAULT 'dev', key_salt BLOB NOT NULL
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
 source_id INTEGER NOT NULL, protocol TEXT NOT NULL CHECK(protocol IN ('hy2','vless','tuic','anytls','mieru','ss','vmess','trojan')),
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
-- profile_item 取代 profile_node：编排可以引用单个节点（kind='node'），
-- 也可以引用整个来源（kind='source'），后者在渲染时展开成该来源的当前节点，
-- 因此机场新增节点会自动进入已选整源的编排。
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
`
