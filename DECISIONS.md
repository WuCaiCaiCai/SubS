# DECISIONS.md — 决策日志

按时间顺序追加，新决策覆盖旧决策。AI 会话不得推翻已有决策，只能建议人类补充。

| 日期 | 决策 | 备注 |
|---|---|---|
| 2025-11 | 后端 Go 单二进制，前端 Vite+React+TS+Tailwind+shadcn/ui | 契合"一条命令部署" |
| 2025-11 | 数据库 SQLite（WAL） | 单文件易备份 |
| 2025-11 | 目标客户端：Mihomo（主力）+ sing-box + iOS 客户端 | 订阅按 User-Agent 内容协商 |
| 2025-11 | iOS 客户端确认为 **Loon**（不是 Shadowrocket/Stash） | 2025-11-25 用户确认 |
| 2025-11 | 协议支持：hy2 / VLESS / TUIC / AnyTLS / Mieru | Mieru 需检查 sing-box/Loon 支持度，见协议规范 |
| 2025-11 | Web 框架选 **chi**（`github.com/go-chi/chi/v5`） | 比 net/http 原生路由更易写中间件链，轻量无依赖 |
| 2025-11 | `airport.refresh_interval_min` **Phase 1 不启用后台调度** | 字段保留备用；Phase 1 仅手动刷新，Phase 2 视需求启用定时 goroutine |
| 2026-11 | 安装器默认监听 `0.0.0.0:8787`，并在安装结束时打印初始化密钥 | 用户要求免 SSH 查文件；直连为明文 HTTP，安装输出与文档均提示改用反代 + HTTPS |
| 2026-11 | 安装器把监听地址与端口拆成两次询问，端口可自定义（`SUBS_PORT` / `SUBS_LISTEN`） | 端口常需避开已占用或被扫描的默认值；地址与端口语义不同，分开问更清楚 |
| 2026-11 | 安装器改为 `enable` + `restart`，并按出口 IP 打印访问地址 | `enable --now` 不会重启已在运行的服务，升级后仍监听旧端口；回环地址无法复制到浏览器 |
| 2026-11 | WebUI 首版管理受限 Caddy 配置：仅域名 + 本机 SubS 上游，随 `.sbox` 加密迁移；不自动安装/重载 Caddy，也不迁移证书私钥 | 避免给 Web 服务增加 root/systemd 控制权；恢复配置后由运维人员部署，Caddy 在目标机重新签发证书 |
