#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
	cat >&2 <<'EOF'
用法:
  ./scripts/issue-license.sh <机器码> <有效天数>

示例:
  ./scripts/issue-license.sh AOPS-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX 30d
  ./scripts/issue-license.sh AOPS-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX 90d

环境变量:
  AUTOOPS_LICENSE_PRIVATE_KEY  私钥路径，默认 ~/.autoops-license/ed25519-private.pem
  AUTOOPS_LICENSE_CUSTOMER     客户名称，默认 Linux授权
  LICENSECTL_BIN               已编译的 licensectl 路径；不设置时优先搜索 PATH
EOF
}

if [[ $# -ne 2 ]]; then
	usage
	exit 2
fi

machine_code="$(printf '%s' "$1" | tr '[:lower:]' '[:upper:]')"
validity="$2"

if [[ ! "$machine_code" =~ ^AOPS-([A-F0-9]{4}-){7}[A-F0-9]{4}$ ]]; then
	echo "错误: 机器码格式无效" >&2
	exit 2
fi

if [[ ! "$validity" =~ ^([1-9][0-9]*)d$ ]]; then
	echo "错误: 激活时间必须是正整数天数，例如 30d、90d、180d、365d" >&2
	exit 2
fi

days="${BASH_REMATCH[1]}"
private_key="${AUTOOPS_LICENSE_PRIVATE_KEY:-$HOME/.autoops-license/ed25519-private.pem}"
customer="${AUTOOPS_LICENSE_CUSTOMER:-Linux授权}"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd -- "$script_dir/.." && pwd)"

if [[ ! -f "$private_key" ]]; then
	echo "错误: 找不到发行私钥: $private_key" >&2
	exit 1
fi

if [[ "$(stat -c '%a' "$private_key")" != "600" ]]; then
	echo "错误: 私钥权限必须为 600，请执行: chmod 600 '$private_key'" >&2
	exit 1
fi

expires_at="$(date -u -d "+${days} days" '+%Y-%m-%dT%H:%M:%SZ')"

if [[ -n "${LICENSECTL_BIN:-}" ]]; then
	if [[ ! -x "$LICENSECTL_BIN" ]]; then
		echo "错误: LICENSECTL_BIN 不可执行: $LICENSECTL_BIN" >&2
		exit 1
	fi
	license_command=("$LICENSECTL_BIN")
elif command -v licensectl >/dev/null 2>&1; then
	license_command=(licensectl)
else
	license_command=(go run ./cmd/licensectl)
fi

echo "机器码: $machine_code" >&2
echo "有效期: ${days} 天" >&2
echo "到期时间: $expires_at (UTC)" >&2
echo "授权码:" >&2

cd "$repo_dir"
"${license_command[@]}" issue \
	--private "$private_key" \
	--machine "$machine_code" \
	--expires-at "$expires_at" \
	--customer "$customer"
