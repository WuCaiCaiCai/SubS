# DECISIONS.md — 决策日志

按时间顺序追加，新决策覆盖旧决策。AI 会话不得推翻已有决策，只能建议人类补充。

| 日期 | 决策 | 备注 |
|---|---|---|
| 2025-11 | 后端 Go 单二进制，前端 Vite+React+TS+Tailwind+shadcn/ui | 契合"一条命令部署" |
| 2025-11 | 数据库 SQLite（WAL） | 单文件易备份 |
| 2025-11 | 目标客户端：Mihomo（主力）+ sing-box + iOS 客户端 | 订阅按 User-Agent 内容协商 |
| 2025-11 | iOS 客户端确认为 **Loon**（不是 Shadowrocket/Stash） | 2025-11-25 用户确认 |
| 2025-11 | 协议支持：hy2 / VLESS / TUIC / AnyTLS / Mieru | Mieru 需检查 sing-box/Loon 支持度，见协议规范 |
