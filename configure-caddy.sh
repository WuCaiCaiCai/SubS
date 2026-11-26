#!/usr/bin/env bash
# Install the restored SubS Caddyfile snippet without replacing other sites.
# Usage: sudo ./configure-caddy.sh [--data-dir DIR] [--caddyfile PATH]
set -euo pipefail

DATA_DIR="${SUBS_DATA_DIR:-/var/lib/subs}"
MAIN_FILE="${CADDY_MAIN_FILE:-/etc/caddy/Caddyfile}"
SNIPPET="${CADDY_SUBS_FILE:-/etc/caddy/subs.Caddyfile}"
SNIPPET_IMPORT="${SNIPPET#"$(dirname "$MAIN_FILE")"/}"
[[ "$SNIPPET_IMPORT" != "$SNIPPET" ]] || SNIPPET_IMPORT="$SNIPPET"
SERVICE="${CADDY_SERVICE:-caddy}"
BIN="${CADDY_BIN:-}"

usage() {
  echo "用法: sudo $0 [--data-dir DIR] [--caddyfile PATH]"
  echo "环境变量: CADDY_MAIN_FILE CADDY_SUBS_FILE CADDY_SERVICE CADDY_BIN"
}
while (($#)); do
  case "$1" in
    --data-dir) (($# >= 2)) || { usage >&2; exit 2; }; DATA_DIR="$2"; shift 2 ;;
    --caddyfile) (($# >= 2)) || { usage >&2; exit 2; }; MAIN_FILE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数: $1" >&2; usage >&2; exit 2 ;;
  esac
done

fail() { echo "[错误] $*" >&2; exit 1; }
[[ ${EUID} -eq 0 ]] || fail "请使用 sudo 运行"
SOURCE="$DATA_DIR/Caddyfile"
[[ -f "$SOURCE" && ! -L "$SOURCE" ]] || fail "找不到恢复的配置文件：$SOURCE"
[[ -s "$SOURCE" ]] || fail "配置文件为空：$SOURCE"
if [[ -z "$BIN" ]]; then BIN="$(command -v caddy || true)"; fi
[[ -n "$BIN" && -x "$BIN" ]] || fail "未找到 Caddy，请先安装 Caddy"

# Keep the Caddyfile intentionally simple and reject unexpected directives or
# import tricks. The WebUI generates exactly one site block and local upstream.
if ! grep -Eq '^[[:alnum:]][[:alnum:].-]*\.[[:alpha:]]{2,63} \{$' "$SOURCE" || \
   ! grep -Eq '^[[:space:]]*reverse_proxy 127\.0\.0\.1:[0-9]{1,5}$' "$SOURCE" || \
   grep -Eq '^[[:space:]]*(import|@|handle|route|redir|respond|file_server|tls)[[:space:]]' "$SOURCE"; then
  fail "配置不符合 SubS 生成格式；拒绝执行，避免导入任意 Caddy 指令"
fi

MAIN_DIR="$(dirname "$MAIN_FILE")"
SNIP_DIR="$(dirname "$SNIPPET")"
mkdir -p "$MAIN_DIR" "$SNIP_DIR"
[[ ! -L "$MAIN_FILE" && ! -L "$SNIPPET" ]] || fail "配置路径不能是符号链接"

STAMP="$(date +%Y%m%d%H%M%S)"
TMP_SNIP="$(mktemp "$SNIP_DIR/.subs.Caddyfile.XXXXXX")"
TMP_MAIN="$(mktemp "$MAIN_DIR/.Caddyfile.XXXXXX")"
BACKUP="${MAIN_FILE}.subs-backup-${STAMP}"
SNIP_BACKUP="${SNIPPET}.subs-backup-${STAMP}"
HAD_MAIN=0
HAD_SNIP=0
[[ ! -e "$MAIN_FILE" ]] || HAD_MAIN=1
[[ ! -e "$SNIPPET" ]] || HAD_SNIP=1
cleanup() { rm -f "$TMP_SNIP" "$TMP_MAIN"; }
trap cleanup EXIT

install -m 0644 "$SOURCE" "$TMP_SNIP"
if ((HAD_MAIN)); then
  cp -a "$MAIN_FILE" "$BACKUP"
  cp -a "$MAIN_FILE" "$TMP_MAIN"
else
  printf '# SubS reverse proxy managed import\n' > "$TMP_MAIN"
fi
if ! grep -Fqx "import $SNIPPET_IMPORT" "$TMP_MAIN"; then
  printf '\n# SubS reverse proxy managed import\nimport %s\n' "$SNIPPET_IMPORT" >> "$TMP_MAIN"
fi
if ((HAD_SNIP)); then cp -a "$SNIPPET" "$SNIP_BACKUP"; fi

# Validate a staged pair, not the currently installed site snippet. Rewrite
# only the managed import in the temporary main config to point at that pair.
STAGED_SNIP="${SNIPPET}.validate-$$"
STAGED_MAIN="${MAIN_FILE}.validate-$$"
cp "$TMP_SNIP" "$STAGED_SNIP"
chmod 0644 "$STAGED_SNIP"
sed "s|^import ${SNIPPET_IMPORT//|/\\|}$|import $STAGED_SNIP|" "$TMP_MAIN" > "$STAGED_MAIN"
chmod 0644 "$STAGED_MAIN"
trap 'rm -f "$TMP_SNIP" "$TMP_MAIN" "$STAGED_SNIP" "$STAGED_MAIN"' EXIT
if ! "$BIN" validate --config "$STAGED_MAIN" --adapter caddyfile; then
  fail "Caddy 配置校验失败；未修改当前生效配置"
fi

# Commit both files, then reload. On failure restore the prior files and try to
# reload the old config so existing sites remain available.
install -m 0644 "$TMP_SNIP" "$SNIPPET.new"
install -m 0644 "$TMP_MAIN" "$MAIN_FILE.new"
mv -f "$SNIPPET.new" "$SNIPPET"
mv -f "$MAIN_FILE.new" "$MAIN_FILE"
if ! systemctl reload "$SERVICE"; then
  echo "[警告] reload 失败，回滚 Caddy 配置" >&2
  if ((HAD_MAIN)); then cp -a "$BACKUP" "$MAIN_FILE"; else rm -f "$MAIN_FILE"; fi
  if ((HAD_SNIP)); then cp -a "$SNIP_BACKUP" "$SNIPPET"; else rm -f "$SNIPPET"; fi
  systemctl reload "$SERVICE" || systemctl restart "$SERVICE" || true
  fail "Caddy 重载失败，已恢复旧配置"
fi

echo "SubS Caddy 配置已应用。"
echo "主配置: $MAIN_FILE"
echo "站点片段: $SNIPPET"
if ((HAD_MAIN)); then echo "原配置备份: $BACKUP"; fi
if ((HAD_SNIP)); then echo "原片段备份: $SNIP_BACKUP"; fi
echo "请确认 DNS 指向本机，且 Caddy 可通过公网完成证书验证。"
