# 部署与 CLI 命令规范

## 1. 一条命令安装

```bash
curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash
```

推送 `v*` tag 会触发 `.github/workflows/release.yml`：运行前端构建与 Go 测试，构建 Linux x64 二进制，并将安装脚本、压缩包和 `checksums.txt` 上传到 GitHub Release（v0.1beta 起已验证）。

安装器行为（每步打印到 stdout，失败即停并说明回滚）：

1. 检查 OS（仅 Linux）与 CPU 架构（amd64/arm64），读取最新 GitHub Release
2. 下载二进制与 `checksums.txt`，sha256 校验
3. 安装到 `/usr/local/bin/subs`（若已存在且版本不同，提示升级而非覆盖数据）
4. 创建系统用户 `subs`（无登录 shell）
5. 建数据目录 `/var/lib/subs/`（owner subs，0700）
6. 生成一次性初始化密钥：写入 `/var/lib/subs/init-token`（0600，属主 subs），安装结束时直接打印在终端；程序启动时若未传 `--init-token` 则自动读取该文件
7. 交互询问监听地址与端口：地址默认 `0.0.0.0`（对外直连），填 `127.0.0.1` 则仅本机 + 反代；端口默认 `8787`，校验 1-65535。也可用 `SUBS_BIND` / `SUBS_PORT` 预置，或用 `SUBS_LISTEN=host:port` 一次性指定
8. 写 systemd unit（`/etc/systemd/system/subs.service`），`Restart=on-failure`，`ProtectSystem=strict` + `ReadWritePaths=/var/lib/subs`，`ExecStart` 带 `--data-dir` 与 `--listen`
9. 启动服务并健康检查，输出访问地址、初始化密钥、防火墙放行提示与 HTTPS 反代警告

升级：重跑安装命令 → 备份数据库快照到 `/var/lib/subs/pre-upgrade-<ts>.db` → 替换二进制（旧的保留为 `/usr/local/bin/subs.old`）→ 重启；失败可手动回滚二进制。
卸载：`subs uninstall` → 询问是否保留数据，停服务删 unit/二进制/用户，按选择处理数据目录。

## 2. 本地 CLI 子命令（需 root 或 subs 用户，在 VPS 上执行）

| 命令 | 作用 |
|---|---|
| `subs reset-password` | 交互重置管理员密码，使所有会话失效 |
| `subs backup -o file [-p pass]` | 无网页时的本地备份 |
| `subs restore file [-p pass]` | 本地恢复（恢复前自动快照当前库） |
| `subs uninstall [--purge]` | 停服务、删 unit 与二进制；`--purge` 同时删数据目录与系统用户 |
| `subs version` / `subs status` | 版本 / 实例信息与数据统计 |

CLI 不提供 `destroy`：销毁必须走网页（强确认流程）或 `subs uninstall --purge`（要求交互输入实例名，--purge 含数据目录）。

## 3. 销毁流程（网页端）

1. POST `/api/destroy` 仅接受 `{confirmPhrase: <实例名>, backupSha256?}`
2. 未提供备份 sha256 → 409 拒绝；提供了 → 与最近一次备份下载记录核对，不匹配则拒绝
3. 通过后：停写 → 生成最终备份到临时路径并留在 `/var/lib/subs/final-backup.sbox`（用户可再取一次）→ 删除数据库、设置、密钥 → systemd 卸载（保留二进制，除非 --purge）
4. 前端在进入销毁页面前强制展示"销毁边界"说明（不退订 VPS、不清云快照）
