#!/usr/bin/env bash
# SubS 安装器 —— 一条命令部署到自有 VPS。
#
#   curl -fsSL https://github.com/WuCaiCaiCai/SubS/releases/latest/download/install.sh | sudo bash
#
# 可选环境变量：
#   SUBS_REPO       GitHub 仓库（owner/name），默认 WuCaiCaiCai/SubS
#   SUBS_VERSION    指定版本标签（如 v0.1.0），默认取最新 Release
#   SUBS_DATA_DIR   数据目录，默认 /var/lib/subs
#   SUBS_SERVICE    服务名，默认 subs
#   SUBS_YES=1      跳过所有交互确认
set -euo pipefail

REPO="${SUBS_REPO:-WuCaiCaiCai/SubS}"
VERSION="${SUBS_VERSION:-latest}"
INSTALLER_VERSION="${SUBS_INSTALLER_VERSION:-}"
REQUESTED_VERSION="${SUBS_VERSION:-}"
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

if [[ -z "$INSTALLER_VERSION" ]]; then
  step "获取安装器版本"
  API_JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" || die "无法获取最新 Release"
  INSTALLER_VERSION="$(printf '%s' "$API_JSON" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
  [[ -n "$INSTALLER_VERSION" ]] || die "GitHub 未返回 Release 标签"
fi
VERSION="${REQUESTED_VERSION:-$INSTALLER_VERSION}"

if [[ "$VERSION" == "latest" ]]; then
  VERSION="$INSTALLER_VERSION"
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

DATA_DIR="$(ask "数据目录 [${DATA_DIR}]: " "$DATA_DIR")"

# 监听地址：默认对外监听，反代与直连都能用；直连为明文 HTTP，因此在下面提示。
LISTEN="${SUBS_LISTEN:-0.0.0.0:8787}"
LISTEN="$(ask "监听地址（0.0.0.0:8787 对外直连 / 127.0.0.1:8787 仅本机 + 反代）[${LISTEN}]: " "$LISTEN")"

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

# 必须在启动服务前生成：服务一跑起来就会建库，之后再判断“是否首次安装”
# 永远为假，密钥会被跳过。密钥写入 0600 文件，由程序启动时自行读取。
INIT_TOKEN=""
if [[ ! -e "$DATA_DIR/subs.db" && ! -e "$DATA_DIR/init-token" ]]; then
  step "生成一次性初始化密钥"
  INIT_TOKEN="$(head -c 24 /dev/urandom | base64 | tr -d '/+=' | cut -c1-32)"
  printf '%s\n' "$INIT_TOKEN" >"$DATA_DIR/init-token"
  chown "${SERVICE}:${SERVICE}" "$DATA_DIR/init-token"
  chmod 600 "$DATA_DIR/init-token"
else
  # 升级重新跑脚本时密钥已存在，仍读出来展示，避免用户再 SSH 查文件。
  if [[ -r "$DATA_DIR/init-token" ]]; then
    INIT_TOKEN="$(tr -d '\n' <"$DATA_DIR/init-token")"
  fi
fi
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

# 监听在 0.0.0.0/:: 时探测回环地址，避免依赖具体网卡。
PROBE_HOST="${LISTEN%:*}"
PROBE_PORT="${LISTEN##*:}"
case "$PROBE_HOST" in
  0.0.0.0|"::"|""|"*") PROBE_HOST=127.0.0.1 ;;
esac
HEALTHY=""
for _ in $(seq 1 20); do
  if curl -fsS -o /dev/null "http://${PROBE_HOST}:${PROBE_PORT}/healthz" 2>/dev/null; then HEALTHY=1; break; fi
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
${INIT_TOKEN:+初始化密钥：${INIT_TOKEN}}
${INIT_TOKEN:+  （请立即保存；首次初始化时与管理员密码一起提交）}
${INIT_TOKEN:-初始化密钥已存在：${DATA_DIR}/init-token（已完成初始化则无需理会）}

常用命令：
  systemctl status ${SERVICE}
  journalctl -u ${SERVICE} -f
  ${BIN_PATH} status --data-dir ${DATA_DIR}
  ${BIN_PATH} reset-password --data-dir ${DATA_DIR}

注意：
  · 当前监听 ${LISTEN}；需更改请编辑 systemd unit 的 --listen 参数并重启
  · 直连是明文 HTTP：登录密码与订阅 token 会裸奔在公网，强烈建议用 Nginx/Caddy 反代并启用 HTTPS
  · 若监听 0.0.0.0，需在云厂商安全组/防火墙放行 ${PROBE_PORT} 端口
  · 订阅链接中的节点信息是凭据，公开前务必确认域名不在公共索引里
  · 初始化密钥也保存在 ${DATA_DIR}/init-token（权限 0600）；升级时不会重置
  · 升级：重跑本脚本（会自动覆盖二进制并重启）
  · 卸载：${BIN_PATH} uninstall（保留数据）或 systemctl stop ${SERVICE} 后手动清理
INFO