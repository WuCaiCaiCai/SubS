# 工作日志

按时间倒序记录已完成的改动与未完事项。产品决策见 `DECISIONS.md`，待办见 `TODO.md`。

---

## 2025-11 会话：订阅渲染修复 + 开关交互修复

### 状态：已暂停维护

代码停在本地 `main` 的 `63fdf23`，**未推送、未打 tag**。线上最新 Release 是 v0.2.3。
恢复维护时直接看 `TODO.md` 末尾的「恢复维护时下一步」。

### 一、订阅「导入成功但客户端不可用」

- 根因：`renderMihomo` 只在 `# subs:nodes` 标记处注入节点名，WebUI 新建模板默认内容既无标记也无 `include-all: true` → 代理组只剩 `DIRECT`，`MATCH,PROXY` 全部直连。
- 修复：`internal/render/mihomo.go` 加回退——没有任何组经标记拿到节点名时，对 `proxy-groups` 下 `proxies` 列表存在的组自动追加节点名，保留 `DIRECT` 等原有成员。带标记 / `include-all` 的模板不受影响。
- 踩坑：首版实现误匹配了顶层 `proxies` 列表，污染节点定义（`TestRenderMihomoMieruPortRange` 报 `cannot unmarshal !!str into map[string]interface{}`）。修正为只作用于 `proxy-groups` 下。

### 二、以真实订阅为基准做字段对齐

基准文件是用户提供的真实 Mihomo 订阅（`Client.yaml`）。方法：解析其中节点 → 整份文件当模板 → 渲染 → 逐节点逐字段深比较。

发现并修复的缺口：

| 协议 | 缺失 | 修在哪 |
|---|---|---|
| vless/vmess | `servername` | 解析端原先只读 `sni`，mihomo 的 TLS SNI 字段名是 `servername`；改为双读 + 回退 |
| 全协议 | `udp` | 解析端读 `udp`，QUIC 系（hy2/tuic）缺省时默认开；渲染端统一输出 |
| tuic | `version: 5`、`tls: true` | `version` 存 `Extra`，渲染端输出 |
| hy2 | `alpn: [h3]` | 解析端读列表压成字符串，渲染端还原为列表 |

另外修了**层级组污染**：`fixGroupsWithoutNodes` 曾给 `[Stable, DIRECT]` 这类成员全是组引用的层级组追加全部节点名。现在先收集组名集合，含组引用的组直接跳过。

**有意豁免**：参考文件里 hysteria2 同时写了 `auth`，那是 sing-box 系别名，mihomo 认 `password`，不补。

**测试资产**：真实配置的脱敏副本存为 `internal/render/testdata/reference.yaml`（uuid / 域名 / IP / 10 个密码全部替换为文档专用假值，协议特征保留），配套 `internal/render/reference_test.go` 做逐字段断言 + proxy-groups / rules / 顶层键完整性校验。CI 可复跑，当前输出与基准完全一致。

**参考资料**：对照了 `eun1e/sublinkE` 的 `node/clash.go` 确认方向一致——它对每个代理都显式带 `udp`，vless 用 `Servername`，hy2 凭据用 `Password`，tuic 不写 `auth`。但它的组注入策略是「全部组追加全部节点」，没有层级组保护；我们的实现更保守，也更符合该基准文件的层级语义。

### 三、移除 Caddy 集成

- 删除 `internal/server/caddy.go`、`caddy_test.go`、`configure-caddy.sh`，以及 `/api/caddy`、`/api/caddyfile` 路由、备份快照/恢复、schema 列 `caddy_domain`/`caddy_port`（旧库遗留列不读）。
- 旧备份里的 `caddy/Caddyfile` 恢复时静默跳过。
- 顺带修了 `release.yml`：它还在 `cp configure-caddy.sh`，文件删除后 CI 会直接失败。校验和现在只含 tar.gz 与 install.sh。

### 四、HTTPS-only 开关

- `instance.https_only` 列 + `httpsOnlyMiddleware`：非加密请求返回 426（中文提示页），`/healthz` 始终放行。
- 信任模型：直连 TLS，或**回环地址**的反代声明 `X-Forwarded-Proto: https`。
- 防自锁：只在当前连接已是 HTTPS 时才允许开启（后端 400 `not_secure` + 前端 `window.location.protocol` 预检）。

### 五、订阅名 slug

- `profile.slug`：小写字母/数字/连字符，≤32 字符，唯一（部分索引，在 `migrate()` 里建而非 schema 阶段——schema 先于 migrate 执行，旧库没有该列会报 `no such column`）。
- 路由：`/s/<slug>/<token>` 与 `/s/<token>` 并存（chi 无可选路径段，需分别注册）。
- 下载文件名、`bundle.zip`、客户端导入名优先用 slug；冲突返回 409（预检查 SELECT + `isUniqueViolation` 文本匹配兜底，因为 modernc.org/sqlite 的错误文本不含索引名）。
- 保留字：`bundle.zip` / `healthz` / `api` / `assets`。

### 六、开关点击无响应（最新改动，未发布）

| 症状 | 根因 | 修复 |
|---|---|---|
| 仅 HTTPS 开关点了没反应，报「实例名需为 1-64 个字符」 | 后端 `updateSystem` 强制 `instanceName` 非空，前端 toggle 只发 `{ httpsOnly }` | 实例名留空 = 保持不变，传了才校验 |
| 节点开关要点两次 / 看起来要刷新 | `toggleNode` 成功后 `nodes.reload()`，期间 `nodes.loading` 为真、列表退回骨架屏 | 改乐观更新：立即翻转 → 服务器返回值覆盖 → 失败回弹 |

Settings 的 httpsOnly 开关同样改为乐观更新；`saveGeneral` 改用接口返回值回填，不再依赖 `system.reload()`。

验证：`go build` / `go vet` / `go test ./...` / `tsc --noEmit` / `npm run build` 全绿。
**未做**：浏览器实点验证（成本较高，且该改动的失败模式直观——要么立即折叠、要么回弹报错）。

### 过程中踩到的坑（备忘）

- 必须用 `go build -o <out> ./cmd/subs`，从仓库根直接 `go build` 得到的二进制是坏档案（`!<arch>`）。
- `cmd/subs` 没有 `init` 子命令；无参运行会直接启动服务并阻塞终端。
- `modernc.org/sqlite` 不暴露错误码，唯一约束冲突只能靠错误文本判断，且文本里不含索引名。
- 测试里用 `testServer()` 构造的 server 没有 instance 行；需要 `https_only` 的用例得自己 `INSERT INTO instance(...)`。
