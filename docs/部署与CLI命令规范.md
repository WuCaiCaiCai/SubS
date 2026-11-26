# 部署与 CLI 命令规范

## 1. 一条命令安装

```bash
curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash
```

当前仓库尚无已验证的正式 Release；发布流程必须将 `install.sh` 与二进制压缩包、`checksums.txt` 一起上传。完成首次 Release 验证前，README 命令属于预定入口，不应宣称已可用。

安装器行为（每步打印到 stdout，失败即停并说明回滚）：

1. 检查 OS（仅 Linux）与 CPU 架构（amd64/arm64），读取最新 GitHub Release
2. 下载二进制与 `checksums.txt`，sha256 校验
3. 安装到 `/usr/local/bin/subs`（若已存在且版本不同，提示升级而非覆盖数据）
4. 创建系统用户 `subs`（无登录 shell）
5. 建数据目录 `/var/lib/subs/`（owner subs，0700）
6. 写 systemd unit（`/etc/systemd/system/subs.service`），`Restart=on-failure`，`ProtectSystem=strict` + `ReadWritePaths=/var/lib/subs`
7. 默认监听 `127.0.0.1:8787`；需更改时由运维人员调整 systemd unit 的 `--listen` 参数
8. 启动服务，生成一次性初始化密钥，仅输出到安装终端并保存在权限为 0600 的数据目录文件中（24h 后失效）
9. 输出访问地址、防火墙提示、HTTPS 反代建议

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
