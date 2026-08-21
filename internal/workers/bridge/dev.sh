#!/usr/bin/env bash

set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
admin_dir="$script_dir/admin"
worker_port=${BRIDGE_WORKER_PORT:-8787}
pages_port=${BRIDGE_PAGES_PORT:-8788}
wrangler_version=${WRANGLER_VERSION:-latest}

# These predictable values are only for local development. Override them through
# the environment when a test needs different credentials.
export BRIDGE_TOKEN=${BRIDGE_TOKEN:-local-bridge-token}
export BRIDGE_ADMIN_TOKEN=${BRIDGE_ADMIN_TOKEN:-local-bridge-admin-token}

worker_pid=""
pages_pid=""
cleaned_up=0

cleanup() {
  if [[ "$cleaned_up" -eq 1 ]]; then
    return
  fi
  cleaned_up=1
  trap - INT TERM HUP

  for process_id in "$pages_pid" "$worker_pid"; do
    if [[ -n "$process_id" ]] && kill -0 "$process_id" 2>/dev/null; then
      kill "$process_id" 2>/dev/null || true
    fi
  done
  for process_id in "$pages_pid" "$worker_pid"; do
    if [[ -n "$process_id" ]]; then
      wait "$process_id" 2>/dev/null || true
    fi
  done
}

wait_for_service() {
  local url=$1
  local process_id=$2
  local attempt=0
  while [[ "$attempt" -lt 60 ]]; do
    if curl --silent --show-error "$url" >/dev/null 2>&1; then
      return 0
    fi
    if ! kill -0 "$process_id" 2>/dev/null; then
      return 1
    fi
    attempt=$((attempt + 1))
    sleep 0.5
  done
  return 1
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

command -v npx >/dev/null 2>&1 || {
  echo "错误: 未找到 npx，请先安装 Node.js 22 或更高版本。" >&2
  exit 1
}
command -v curl >/dev/null 2>&1 || {
  echo "错误: 未找到 curl，无法检查本地 Worker 是否启动成功。" >&2
  exit 1
}

echo "正在构建 Bridge Admin Pages..."
"$admin_dir/build.sh"

echo "正在准备 Wrangler $wrangler_version..."
npx --yes "wrangler@$wrangler_version" --version >/dev/null

echo "正在启动本地 Worker: http://127.0.0.1:$worker_port"
(
  cd "$script_dir"
  exec npx --yes "wrangler@$wrangler_version" dev --config wrangler.jsonc --port "$worker_port"
) &
worker_pid=$!

if ! wait_for_service "http://127.0.0.1:$worker_port/health" "$worker_pid"; then
  echo "错误: 本地 Worker 未能在 30 秒内启动。" >&2
  exit 1
fi

echo "正在启动本地 Pages: http://127.0.0.1:$pages_port"
(
  cd "$admin_dir"
  exec npx --yes "wrangler@$wrangler_version" pages dev --port "$pages_port"
) &
pages_pid=$!

if ! wait_for_service "http://127.0.0.1:$pages_port/" "$pages_pid"; then
  echo "错误: 本地 Pages 未能在 30 秒内启动。" >&2
  exit 1
fi

echo
echo "Bridge 本地开发环境已启动："
echo "  Worker: http://127.0.0.1:$worker_port"
echo "  Pages:  http://127.0.0.1:$pages_port"
echo "  管理页用户名: admin"
echo "按 Ctrl-C 停止两个服务。"

exit_status=0
while kill -0 "$worker_pid" 2>/dev/null && kill -0 "$pages_pid" 2>/dev/null; do
  sleep 1
done

if ! kill -0 "$worker_pid" 2>/dev/null; then
  wait "$worker_pid" || exit_status=$?
  echo "本地 Worker 已退出。" >&2
else
  wait "$pages_pid" || exit_status=$?
  echo "本地 Pages 已退出。" >&2
fi
exit "$exit_status"
