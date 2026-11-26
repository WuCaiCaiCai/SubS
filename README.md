# SubS

私有订阅管理与分发工具。把机场订阅与自建 VPS 节点汇总成一处，统一命名、套用 Clash 模板，对外只给一条订阅链接。

单文件 Go 二进制 + SQLite，无外部依赖，适合放在自己的小 VPS 上。

## 它解决什么

- 机场订阅每次刷新节点名乱变，客户端里的策略组天天失效
- 自建节点散落在多个 VPS，协议五花八门（hy2 / vless / tuic / anytls / mieru）
- 想给手机、软路由、家人各发一份配置，又不想把机场原始订阅链接给出去

SubS 只做三件事：**收进来、理清楚、发出去**。

## 快速开始

### 部署到 VPS

```bash
curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash
```

推送 `v*` tag 后，GitHub Actions 会运行测试、构建 Linux x64 二进制并发布 Release，同时上传安装脚本与 SHA-256 校验文件（v0.1beta 起已验证可用）。安装时会询问监听地址与端口（默认 `0.0.0.0:8787`，端口可自定义，也可用 `SUBS_BIND` / `SUBS_PORT` 预置）；直连是明文 HTTP，建议改用 Nginx/Caddy 反代并启用 HTTPS（安装时填 `127.0.0.1:8787` 即仅本机监听）。初始化密钥会在安装结束时直接打印。

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

导入方式：分享链接逐行粘贴、Base64 订阅、Clash/Mihomo YAML，以及 mieru 客户端配置 JSON（`mieru export config` 的产物，可直接粘贴）。

目标不支持的节点不会静默丢失：输出文件头部会注释列出被跳过的节点名。

## mieru 导入格式

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

- 管理员密码 argon2id；机场/服务器凭据用 AES-256-GCM 加密后落库，密钥在数据目录 `master.key`（0600）
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

## 开源协议

[MIT](LICENSE)
