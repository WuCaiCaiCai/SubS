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

> 直连是明文 HTTP。公网使用请务必自行配置反向代理（Caddy / Nginx 等）启用 HTTPS，不要把管理面板继续暴露在明文 HTTP 上；随后可在「设置 → 仅支持 HTTPS 访问」开启强校验。

### 升级

沿用原数据目录重新执行安装脚本即可原地升级，**不会丢失数据**：

```bash
curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash
```

- **数据保留**：`/var/lib/subs` 下的 `subs.db`、`master.key` 等文件会原样保留，服务启动时自动完成数据库结构迁移，既有账号、节点、编排与订阅链接均不受影响。
- **旧版本回退**：安装脚本会把旧二进制备份为 `/usr/local/bin/subs.old`；升级前也可先在网页「备份」或用 `subs backup` 导出一份 `.sbox` 备份。
- **自定义参数**：若之前修改过数据目录或监听地址/端口，升级时在交互提示中保持一致即可（或通过 `SUBS_DATA_DIR`、`SUBS_BIND`、`SUBS_PORT`、`SUBS_VERSION=vX.Y.Z` 指定；加 `SUBS_YES=1` 可跳过交互直接按默认值覆盖升级）。

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

官方没有分享链接约定，本项目定义如下（同时兼容常见变体，包括 scheme 写成 `mierus://` 的版本）：

```
mieru://username:password@host:port?transport=TCP&multiplexing=MULTIPLEXING_LOW&handshake-mode=HANDSHAKE_STANDARD&traffic-pattern=base64#节点名
```

- 端口可写单端口 `:9000`，也可写范围 `:2090-2099`（Mihomo 输出为 `port-range`）
- 凭据也可以放在 query 里：`?username=u&password=p`
- `transport`：`TCP`（默认）或 `UDP`
- `multiplexing`：`MULTIPLEXING_OFF/LOW/MIDDLE/HIGH`
- `handshake-mode`：`HANDSHAKE_STANDARD`（默认）或 `HANDSHAKE_NO_WAIT`（0-RTT）
- 用户名与密码是凭据，节点列表里只显示主机与端口

## 启用 HTTPS（反向代理由你自行部署）

SubS 本身不监听 HTTPS，由你自己的反向代理（Caddy、Nginx、宝塔、Traefik 等）做 TLS 终止。推荐流程：

1. **让 SubS 只监听回环**：安装时监听地址填 `127.0.0.1`，或编辑 systemd unit 里的 `--listen` 后 `systemctl restart subs`。
2. **在反代上配置站点**：以 Caddy 为例，一行就够（证书自动签发续期）：

   ```caddyfile
   subs.example.com {
       reverse_proxy 127.0.0.1:8787
   }
   ```

   使用 Nginx 时请同时传递 `proxy_set_header X-Forwarded-Proto https;`。
3. **开启「仅支持 HTTPS 访问」**：在 WebUI「设置」页打开开关后，所有明文 HTTP 请求（含订阅链接）一律返回 426，只接受直连 TLS 或反代声明的 `X-Forwarded-Proto: https`；`/healthz` 探活不受影响。开关只能在 HTTPS 连接下开启，避免把自己锁在门外。

订阅链接与备份恢复均与反代无关：换代理、换证书不需要改 SubS 任何配置。

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
```

设计文档见 `docs/` 目录：`docs/设计说明.md`、`docs/数据模型与API规范.md`、`docs/协议解析与配置生成规范.md` 等。

## 免责声明与合规说明

1. **纯配置管理与格式转换工具**：本项目仅用于个人或团队在自有设备、自有服务器上进行网络配置文件的整理、命名规范化、模板渲染与格式转换。本项目**不提供、不内置、不存储、不分发任何代理服务器、网络节点、订阅链接或翻墙服务**，亦不包含任何主动突破网络访问限制的功能。
2. **本地自托管与零中心化**：本项目为单机/私有部署程序，无任何中心服务器、云端账号系统或遥测上报；所有配置数据均保存在使用者自行部署的本地环境中，作者无法接触、获取或控制使用者导入的任何内容。
3. **合法合规使用义务**：下载、编译、部署或使用本项目的任何人，均须严格遵守所在国家及地区的法律法规，并确保其所管理的服务器资源及网络配置拥有合法授权（如合法的企业内网运维、研发测试环境管理等）。**严禁将本项目用于任何违反当地法律法规或侵犯他人合法权益的用途**；若使用者所在司法辖区禁止使用此类配置管理工具，请勿下载或使用。
4. **责任自负与无担保声明**：本项目按 [MIT](LICENSE) 协议以「现状（AS IS）」开源提供，作者及贡献者不对软件的适用性、稳定性或特定用途作任何明示或暗示的担保。因使用、修改、二次分发本项目或违反法律法规所产生的一切风险、损失及法律责任，均由使用者自行承担，作者及贡献者不承担任何直接、间接或连带责任。
5. **第三方兼容性说明**：文档中提及的客户端名称与配置格式（如 Mihomo、sing-box、Loon 等）仅用于说明输出配置文件的格式兼容性，本项目与上述第三方项目无任何隶属、合作或官方背书关系。

## 开源协议

[MIT](LICENSE)
