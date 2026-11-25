# 数据模型与 API 规范

前后端共同契约。改此文档必须与实现代码同一提交。

## 1. SQLite Schema

启用 WAL。所有表含 `created_at`/`updated_at`（RFC3339 文本）。敏感字段以 `secret_` 前缀，存加密密文（见技术架构 secrets 模块）。

```sql
-- 实例设置（单行）
CREATE TABLE instance (
  id              INTEGER PRIMARY KEY CHECK (id = 1),
  name            TEXT NOT NULL DEFAULT 'my-subs', -- 实例名，销毁确认短语用
  listen_host     TEXT NOT NULL DEFAULT '127.0.0.1',
  listen_port     INTEGER NOT NULL DEFAULT 8787,
  external_url    TEXT,              -- 可选，生成订阅链接用
  version         TEXT NOT NULL,
  key_salt        BLOB NOT NULL      -- 应用密钥派生盐
);

CREATE TABLE admin (
  id           INTEGER PRIMARY KEY CHECK (id = 1),
  username     TEXT NOT NULL,
  secret_password_hash TEXT NOT NULL,  -- argon2id
  totp_secret  TEXT                    -- 预留，可选
);

-- 机场来源
CREATE TABLE airport (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,
  secret_url   TEXT NOT NULL,          -- 加密
  enabled      INTEGER NOT NULL DEFAULT 1,
  note         TEXT,
  refresh_interval_min INTEGER,        -- NULL = 仅手动
  last_ok_at   TEXT,
  last_error   TEXT,                   -- 脱敏后的错误摘要
  node_count   INTEGER
);

-- VPS 分组（与 airport 平级）
CREATE TABLE server (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,
  address     TEXT,                    -- 标识用，非必填
  note         TEXT,
  tags         TEXT                    -- JSON 数组
);

-- 节点（机场解析 + 自建统一存放）
CREATE TABLE node (
  id           INTEGER PRIMARY KEY,
  source_type  TEXT NOT NULL CHECK (source_type IN ('airport','server')),
  source_id    INTEGER NOT NULL,
  protocol     TEXT NOT NULL CHECK (protocol IN ('hy2','vless','tuic','anytls','mieru')),
  raw_name     TEXT NOT NULL,          -- 原始名（机场给定或手填）
  fingerprint  TEXT NOT NULL,          -- 指纹：protocol+host+port+关键标识，用于刷新合并
  config       TEXT NOT NULL,          -- JSON，结构见协议规范（含明文凭据，整库加密盘上文件权限 0600 保护）
  enabled      INTEGER NOT NULL DEFAULT 1,
  missing      INTEGER NOT NULL DEFAULT 0,   -- 机场刷新后消失
  is_new       INTEGER NOT NULL DEFAULT 0,   -- 刷新新出现待确认
  tags         TEXT
);
CREATE INDEX idx_node_source ON node(source_type, source_id);
CREATE UNIQUE INDEX idx_node_fp ON node(source_id, fingerprint);

-- 模板
CREATE TABLE template (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,
  target       TEXT NOT NULL DEFAULT 'mihomo',
  content      TEXT NOT NULL,
  last_check   TEXT                    -- 最近校验结果（ok / 错误信息）
);

-- 编排（Profile）
CREATE TABLE profile (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,
  note         TEXT,
  template_id  INTEGER NOT NULL REFERENCES template(id),
  enabled      INTEGER NOT NULL DEFAULT 1,
  naming_rule  TEXT NOT NULL,          -- 命名规则 JSON，见命名规范
  secret_token TEXT NOT NULL UNIQUE,   -- 加密存储；查找用其 sha256 索引列
  token_hash   TEXT NOT NULL UNIQUE,
  last_fetch_at TEXT
);

-- 编排内节点选择与顺序
CREATE TABLE profile_node (
  profile_id   INTEGER NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
  node_id      INTEGER NOT NULL REFERENCES node(id) ON DELETE CASCADE,
  sort_order   INTEGER NOT NULL,
  name_override TEXT,                  -- 单节点覆盖输出名
  PRIMARY KEY (profile_id, node_id)
);

-- 管理会话（服务端 session，reset-password 时整表清空）
CREATE TABLE admin_session (
  id         TEXT PRIMARY KEY,        -- 32 字节随机 base62
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  ip         TEXT
);

-- 备份下载记录（销毁时核对 sha256 用）
CREATE TABLE backup_record (
  id            INTEGER PRIMARY KEY,
  sha256        TEXT NOT NULL UNIQUE,
  size_bytes    INTEGER NOT NULL,
  downloaded_at TEXT,                 -- NULL = 生成后未下载
  created_at    TEXT NOT NULL
);

-- 审计（不含任何秘密）
CREATE TABLE audit (
  id INTEGER PRIMARY KEY, at TEXT NOT NULL,
  actor TEXT, action TEXT NOT NULL, detail TEXT
);
```

删除语义：删 airport → 其 node 级联删除前，检查 profile_node 引用，有引用则要求 `?force=1` 并把对应 profile_node 标记不可用（保留行，输出时跳过并计入告警）。删 server 同理。

## 2. REST API（前缀 `/api`，管理 Cookie 认证 + CSRF header `X-CSRF-Token`）

约定：错误统一 `{ "error": { "code": "...", "message": "人类可读中文" } }`；列表分页 `?page=&size=`（默认 size=100）；写操作返回更新后实体。

### 认证与会话
```
POST /api/auth/init        {username, password}            仅首次
POST /api/auth/login       {username, password}            限速 5/min
POST /api/auth/logout      {}
GET  /api/auth/me          → {username, csrfToken}
```

### 机场
```
GET    /api/airports
POST   /api/airports         {name, url, note?, refreshIntervalMin?}
GET    /api/airports/:id     → 含 last_ok_at、node_count、last_error（url 遮蔽）
PUT    /api/airports/:id     改名/改刷新/启停（改 url 触发重新拉取）
DELETE /api/airports/:id[?force=1]
POST   /api/airports/:id/refresh      → 同步拉取并返回 {added, matched, missing, errors[]}
POST   /api/airports/:id/confirm-new  → 清除 is_new 标记（或纳入指定 profile）
```

### VPS 分组
```
GET/POST /api/servers
GET/PUT/DELETE /api/servers/:id[?force=1]
```

### 节点
```
GET    /api/nodes?sourceType=&sourceId=&protocol=&q=&enabled=&isNew=&missing=
POST   /api/nodes            {sourceId, protocol, rawName, config}   自建
POST   /api/nodes/import     {sourceId, text}      → {parsed: [{rawName, protocol, host, port, previewConfig}], errors: [{line, reason}]} 预览
PUT    /api/nodes/:id
DELETE /api/nodes/:id
POST   /api/nodes/:id/duplicate
```

### 模板
```
GET/POST /api/templates
GET/PUT/DELETE /api/templates/:id
POST   /api/templates/:id/validate  → {ok, errors[{line, message}]}
```

### 编排
```
GET/POST /api/profiles
GET/PUT/DELETE /api/profiles/:id
PUT    /api/profiles/:id/nodes       [{nodeId, nameOverride?}] 有序数组整体替换
POST   /api/profiles/:id/preview     {target} → 渲染文本（不落库）
POST   /api/profiles/:id/rotate-token → 新 token
GET    /api/profiles/:id/naming-preview → 命名规则应用到当前节点集
```

### 备份 / 销毁 / 系统
```
POST /api/backup           {password} → {downloadId, sha256}（短时一次性）
GET  /api/backup/:downloadId  文件流（no-store）
POST /api/restore          multipart 备份包 + password（校验后进入向导）
POST /api/destroy          {confirmPhrase, backupSha256?}  必须 confirmPhrase=实例名，若未提供已下载备份的 sha256 则拒绝（除非 dryRun）
GET  /api/system           → 版本、监听、磁盘、机场/节点统计
PUT  /api/system/listen    {host, port}
```

### 订阅分发（无管理认证）
```
GET /s/:token
  200 text/yaml  (UA 含 clash/mihomo)
  200 application/json (UA 含 sing-box)
  200 text/plain conf (UA 含 Loon/Surge)
  404 空 body（不区分"不存在/已禁用"，防枚举）
```
统一响应头：`Cache-Control: private, no-store`、`Subscription-Userinfo: upload=...`（如实现用量透传）。

## 3. 并发与一致性

- 所有写 API 经 store 单写互斥；备份导出期间写 API 返回 409。
- 机场 refresh 与 profile_node 合并在一个事务：指纹匹配 → 更新字段；新 fingerprint 插入 `is_new=1`；本轮未出现的置 `missing=1`（不清除 config，用户可恢复或删除）。
