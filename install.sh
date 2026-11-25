#!/usr/bin/env bash
# SubS 安装器 —— 一条命令部署到自有 VPS。
#
#   curl -fsSL https://raw.githubusercontent.com/wucai/subs/main/install.sh | bash
#
# 可选环境变量：
#   SUBS_REPO       GitHub 仓库（owner/name），默认 wucai/subs
#   SUBS_VERSION    指定版本标签（如 v0.1.0），默认取最新 Release
#   SUBS_LISTEN     监听地址，默认 127.0.0.1:8787
#   SUBS_DATA_DIR   数据目录，默认 /var/lib/subs
#   SUBS_SERVICE    服务名，默认 subs
#   SUBS_YES=1      跳过所有交互确认
set -euo pipefail

REPO="${SUBS_REPO:-wucai/subs}"
VERSION="${SUBS_VERSION:-latest}"
LISTEN="${SUBS_LISTEN:-127.0.0.1:8787}"
DATA_DIR="${SUBS_DATA_DIR:-/var/lib/subs}"
SERVICE="${SUBS_SERVICE:-subs}"
BIN_PATH="/usr/local/bin/subs"
UNIT_PATH="/etc/systemd/system/${SERVICE}.service"

step() { printf '\033[36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[!]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }

ask() {
  local prompt="$1" default="${2:-}" answer
  if [[ "${SUBS_YES:-0}" == "1" ]]; then printf '%s' "$default"; return; fi
  read -r -p "$prompt" answer </dev/tty || answer=""
  printf '%s' "${answer:-$default}"
}

[[ "${EUID}" -eq 0 ]] || die "请用 root 运行（sudo bash install.sh）"
[[ "$(uname -s)" == "Linux" ]] || die "仅支持 Linux"

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "不支持的架构：$(uname -m)（支持 amd64 / arm64）" ;;
esac
step "系统：Linux/${ARCH}"

need() { command -v "$1" >/dev/null 2>&1 || die "缺少依赖：$1"; }
need curl
need tar

if [[ "$VERSION" == "latest" ]]; then
  step "查询最新版本"
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)" || true
  [[ -n "$VERSION" ]] || die "无法获取最新版本，可用 SUBS_VERSION=v0.1.0 指定"
fi
step "目标版本：${VERSION}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
BASE="https://github.com/${REPO}/releases/download/${VERSION}"
ASSET="subs_${VERSION}_linux_${ARCH}.tar.gz"
step "下载 ${ASSET}"
curl -fsSL -o "$WORK/$ASSET" "${BASE}/${ASSET}" || die "下载失败：${BASE}/${ASSET}"
curl -fsSL -o "$WORK/checksums.txt" "${BASE}/checksums.txt" || die "下载校验文件失败"

step "校验 sha256"
( cd "$WORK" && grep " ${ASSET}\$" checksums.txt > sum.txt && sha256sum -c sum.txt >/dev/null ) \
  || die "校验失败，安装包可能被篡改"
tar -xzf "$WORK/$ASSET" -C "$WORK"

step "确认监听地址"
LISTEN="$(ask "监听地址 [${LISTEN}]: " "$LISTEN")"
[[ "$LISTEN" =~ ^[^:]+:[0-9]+$ ]] || die "监听地址格式应为 host:port"
DATA_DIR="$(ask "数据目录 [${DATA_DIR}]: " "$DATA_DIR")"

if id "$SERVICE" >/dev/null 2>&1; then
  step "系统用户 ${SERVICE} 已存在"
else
  step "创建系统用户 ${SERVICE}"
  useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin "$SERVICE"
fi

step "准备数据目录 ${DATA_DIR}"
mkdir -p "$DATA_DIR"
chown -R "${SERVICE}:${SERVICE}" "$DATA_DIR"
chmod 700 "$DATA_DIR"

if [[ -f "$BIN_PATH" ]]; then
  step "已存在旧二进制，保留为 ${BIN_PATH}.old"
  cp -f "$BIN_PATH" "${BIN_PATH}.old"
fi

step "安装二进制到 ${BIN_PATH}"
install -m 0755 "$WORK/subs" "$BIN_PATH"

step "写入 systemd unit"
cat >"$UNIT_PATH" <<UNIT
[Unit]
Description=SubS 私有订阅管理
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE}
Group=${SERVICE}
ExecStart=${BIN_PATH} --data-dir ${DATA_DIR} --listen ${LISTEN}
Restart=on-failure
RestartSec=3
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=${DATA_DIR}
AmbientCapabilities=
CapabilityBoundingSet=

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable --now "$SERVICE"

step "生成一次性初始化密钥"
INIT_TOKEN="$(head -c 24 /dev/urandom | base64 | tr -d '/+=' | cut -c1-32)"
printf '%s\n' "$INIT_TOKEN" >"$DATA_DIR/init-token"
chown "${SERVICE}:${SERVICE}" "$DATA_DIR/init-token"
chmod 600 "$DATA_DIR/init-token"

PROBE_HOST="${LISTEN%%:*}"
PROBE_PORT="${LISTEN##*:}"
[[ "$PROBE_HOST" == "0.0.0.0" ]] && PROBE_HOST=127.0.0.1
HEALTHY=""
for _ in $(seq 1 20); do
  if curl -fsS -o /dev/null "http://${PROBE_HOST}:${PROBE_PORT}/healthz"; then HEALTHY=1; break; fi
  sleep 0.5
done

echo
if [[ -n "$HEALTHY" ]]; then
  printf '\033[32m安装完成\033[0m\n'
else
  warn "服务已安装但健康检查未通过，查看日志：journalctl -u ${SERVICE} -n 50"
fi
cat <<INFO

控制台：http://${PROBE_HOST}:${PROBE_PORT}/
初始化密钥：${INIT_TOKEN}
  （24 小时内一次性使用；首次登录时与管理员密码一起提交）

常用命令：
  systemctl status ${SERVICE}
  journalctl -u ${SERVICE} -f
  ${BIN_PATH} status --data-dir ${DATA_DIR}
  ${BIN_PATH} reset-password --data-dir ${DATA_DIR}

注意：
  · 默认只监听本地回环，建议用 Nginx/Caddy 反代并启用 HTTPS 后再对外
  · 订阅链接中的节点信息是凭据，公开前务必确认域名不在公共索引里
  · 升级：重跑本脚本（会自动覆盖二进制并重启）
  · 卸载：${BIN_PATH} uninstall（保留数据）或 systemctl stop ${SERVICE} 后手动清理
INFO