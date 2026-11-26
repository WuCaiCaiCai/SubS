# SubS

私有订阅管理与分发工具：把机场订阅与自建 VPS 节点汇总到一处，统一命名、套用 Clash 模板，对外只暴露一条订阅链接。

单文件 Go 二进制 + 内嵌前端 + SQLite，无运行时外部依赖，适合跑在一台小 VPS 上。

## 它解决什么

- 机场订阅每次刷新节点名乱变，客户端里的策略组天天失效
- 自建节点散落在多台 VPS，协议五花八门（hy2 / vless / tuic / anytls / mieru / Shadowsocks / VMess / Trojan）
- 想给手机、软路由、家人各发一份配置，又不想把机场原始订阅链接给出去

SubS 只做三件事：**收进来、理清楚、发出去**。

## 快速开始

### 一键部署到 VPS

```bash
curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash
```

推送 `v*` tag 后，GitHub Actions 会运行测试、构建 Linux amd64 二进制并发布 Release。安装时交互询问：

- **监听地址**：默认 `0.0.0.0`（对外直连）；填 `127.0.0.1` 则仅本机可访问，配合反向代理使用
- **端口**：默认 `8787`，可自定义（`1–65535`）

也可用环境变量预置：`SUBS_BIND`、`SUBS_PORT`，或一次性指定 `SUBS_LISTEN=host:port`。安装结束会直接打印初始化密钥与可复制的访问地址。

> 直连是明文 HTTP。公网使用请务必走下面的 Caddy 反向代理启用 HTTPS，不要把管理面板继续暴露在明文 HTTP 上。

### 本地跑起来

```bash
make web      # 构建前端
make build    # 产出 ./subs（前端已嵌入）
./subs --data-dir ./data --listen 127.0.0.1:8787 --init-token my-token
```

打开 <http://127.0.0.1:8787/>，用初始化密钥 + 管理员密码完成初始化。

只想改前端：

```bash
cd web && npm run dev   # 5173，已配置 /api 与 /s/ 代理到 8787
```

## 支持的协议

| 协议 | 导入 | Mihomo | sing-box | Loon |
|---|---|---|---|---|
| hysteria2 | ✅ | ✅ | ✅ | ✅ |
| vless（含 Reality） | ✅ | ✅ | ✅ | ✅ |
| tuic | ✅ | ✅ | ✅ | ✅ |
| anytls | ✅ | ✅ | ✅ | ✅ |
| mieru | ✅ | ✅（需 Mihomo ≥ 1.19） | 跳过 | 跳过 |
| Shadowsocks | ✅ | ✅ | ✅ | ✅ |
| VMess | ✅ | ✅ | ✅ | ✅ |
| Trojan | ✅ | ✅ | ✅ | ✅ |

导入方式：分享链接逐行粘贴、Base64 订阅、Clash/Mihomo YAML，以及 mieru 客户端配置 JSON（`mieru export config` 的产物，可直接粘贴）。

目标客户端不支持的节点不会静默丢失：输出文件头部会注释列出被跳过的节点名。

### mieru 导入格式

官方没有分享链接约定，本项目定义如下（同时兼容常见变体）：

```
mieru://username:password@host:port?transport=TCP&multiplexing=MULTIPLEXING_LOW&handshake-mode=HANDSHAKE_STANDARD&traffic-pattern=base64#节点名
```

- 端口可写单端口 `:9000`，也可写范围 `:2090-2099`（Mihomo 输出为 `port-range`）
- 凭据也可以放在 query 里：`?username=u&password=p`
- `transport`：`TCP`（默认）或 `UDP`
- `multiplexing`：`MULTIPLEXING_OFF/LOW/MIDDLE/HIGH`
- `handshake-mode`：`HANDSHAKE_STANDARD`（默认）或 `HANDSHAKE_NO_WAIT`（0-RTT）
- 用户名与密码是凭据，节点列表里只显示主机与端口

## 配置 Caddy 反向代理（HTTPS）

SubS 本身不监听 HTTPS，需要由 Caddy 做 TLS 终止并把流量反代回本机。分三步：

### 1. 让 SubS 只监听回环

安装时监听地址填 `127.0.0.1`，或编辑 systemd unit 里的 `--listen` 后 `systemctl restart subs`。这样只有本机进程能直连，公网流量只经过 Caddy。

### 2. 在 WebUI 保存域名与上游端口

进入 **设置 →「Caddy HTTPS 反向代理」**，填写：

- **公网域名**：例如 `subs.example.com`（DNS 需指向本机）
- **本机上游端口**：SubS 实际监听的端口（如 `8787`），必须与第 1 步一致

保存后会自动生成本机的 Caddyfile，内容形如：

```caddyfile
subs.example.com {
	encode zstd gzip
	reverse_proxy 127.0.0.1:8787
}
```

页面上可预览、下载这份 Caddyfile。它只包含域名与 `127.0.0.1:<端口>` 上游，不含任何其他指令。

### 3. SSH 运行配置脚本接入 Caddy

```bash
sudo bash configure-caddy.sh
```

脚本会：

- 读取 `/var/lib/subs/Caddyfile`，校验内容确属 SubS 生成的受限格式（拒绝任意导入/路由等指令）
- 把它写成本机独立站点片段 `/etc/caddy/subs.Caddyfile`，并在主配置 `/etc/caddy/Caddyfile` 里追加一条 `import`，**不动其他站点**
- 先 `caddy validate` 校验暂存配置，再替换并 `systemctl reload caddy`；任一步失败都会回滚并保留原配置备份
- 通过后打印主配置与站点片段路径，以及原配置的 `.subs-backup-<时间戳>` 备份位置

自定义路径/服务可用参数或环境变量覆盖：

```bash
sudo ./configure-caddy.sh --data-dir /var/lib/subs --caddyfile /etc/caddy/Caddyfile
# 环境变量：CADDY_MAIN_FILE / CADDY_SUBS_FILE / CADDY_SERVICE / CADDY_BIN
```

完成后确认 DNS 已指向本机、80/443 端口可达（Caddy 自动签发证书需要），即可从 `https://你的域名` 登录。

### 迁移时的说明

- WebUI 的 Caddy 配置会随 `.sbox` 加密备份一起迁移（域名 + 上游端口 + 生成的 Caddyfile），但**不包含 Caddy 证书或私钥**
- 迁移到新机后：先跑 SubS 安装脚本 → WebUI 恢复 `.sbox` → SSH 运行 `configure-caddy.sh` → 在新机重新签发证书
- 脚本只接入 SubS 自己的站点片段，不覆盖目标机现有的其他 Caddy 站点；DNS、防火墙、其他站点仍由你自行处理

## 常用命令

```bash
./subs status      --data-dir ./data          # 实例信息与计数
./subs backup      -o b.sbox -p '密码'        # 网页不可用时的本地备份
./subs restore     b.sbox -p '密码'           # 恢复前自动快照当前库
./subs reset-password --data-dir ./data       # 重置密码并使全部会话失效
# 修改监听请编辑 systemd unit 中的 `--listen` 参数后重启服务
./subs uninstall   [--purge]                  # 卸载（--purge 连数据一起删）
```

## 安全边界

- 管理员密码用 argon2id；机场/服务器凭据用 AES-256-GCM 加密后落库，密钥在数据目录 `master.key`（0600）
- 订阅 token 可一键轮换；旧链接立即失效
- 拉取机场订阅时做 SSRF 防护：解析 DNS 后拒绝回环/内网/链路本地地址，8 MiB 体积上限，最多 5 次跳转
- 备份是 age 加密的 tar.gz（`.sbox`），密码不落盘
- 销毁实例需要输入实例名，且必须已存在匹配 sha256 的备份记录
- 没有遥测，不往日志里写密钥

## 开发

```bash
make check    # vet + go test + 前端构建
go test ./... # 后端测试
cd web && npm run build
```

结构：

```
cmd/subs/          入口与 CLI 子命令
internal/server/   HTTP 路由与处理器
internal/store/    SQLite 存储
internal/protocol/ 分享链接 / Clash YAML 解析
internal/render/   模板渲染与三端输出
internal/profile/  命名规则
internal/backup/   .sbox 归档
internal/secrets/  加密与口令
web/               React + Tailwind 前端
configure-caddy.sh Caddy 反向代理接入脚本
```

设计文档见 `docs/` 目录：`docs/设计说明.md`、`docs/数据模型与API规范.md`、`docs/协议解析与配置生成规范.md` 等。

## 开源协议

[MIT](LICENSE)
