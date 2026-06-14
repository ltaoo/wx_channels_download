#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat >&2 <<'EOF'
Usage:
  scripts/dockertest.sh <version>
  scripts/dockertest.sh <image>

Examples:
  scripts/dockertest.sh 26061402
  scripts/dockertest.sh ghcr.io/ltaoo/wx_video_download:v260614
  IMAGE=ghcr.io/ltaoo/wx_video_download:v260614 scripts/dockertest.sh

Environment:
  IMAGE                Full image name. Defaults to wx_video_download:<version>.
  NAME                 Container name. Defaults to wx_download_test_<version>.
  CONFIG_DIR           Host config directory. Defaults to ./wxchannelsdata/<NAME>.
  WEB_PORT             Host web desktop port. Defaults to 3000.
  API_PORT             Host API port. Defaults to 2022.
  PROXY_PORT           Host proxy port. Defaults to 2023.
  CONTAINER_HOSTNAME   Container hostname. Defaults to wx-linux.
EOF
}

VERSION="${VERSION:-}"
if [ -n "${1:-}" ] && [[ "${1}" == *[/:]* ]]; then
    IMAGE="${IMAGE:-${1}}"
elif [ -n "${1:-}" ]; then
    VERSION="${1}"
fi

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
    usage
    exit 0
fi

if [ -z "${IMAGE:-}" ] && [ -z "$VERSION" ]; then
    usage
    exit 1
fi

IMAGE="${IMAGE:-wx_video_download:${VERSION}}"
SAFE_VERSION="${VERSION:-custom}"
SAFE_VERSION="${SAFE_VERSION//[^a-zA-Z0-9_.-]/_}"
NAME="${NAME:-wx_download_test_${SAFE_VERSION}}"
CONFIG_DIR="${CONFIG_DIR:-$(pwd)/wxchannelsdata/${NAME}}"
WEB_PORT="${WEB_PORT:-3000}"
API_PORT="${API_PORT:-2022}"
PROXY_PORT="${PROXY_PORT:-2023}"
CONTAINER_HOSTNAME="${CONTAINER_HOSTNAME:-wx-linux}"
TZ_VALUE="${TZ:-Asia/Shanghai}"
RESOLUTION="${RESOLUTION:-1920x1080x24}"
PUID_VALUE="${PUID:-1000}"
PGID_VALUE="${PGID:-1000}"

if docker ps -a --format '{{.Names}}' | grep -qx "$NAME"; then
    echo "Container already exists: $NAME" >&2
    echo "Use NAME=another_name or remove the existing container first." >&2
    exit 1
fi

mkdir -p "$CONFIG_DIR"

container_id="$(
    docker run -d \
        --name="$NAME" \
        --restart=unless-stopped \
        --hostname="$CONTAINER_HOSTNAME" \
        --security-opt seccomp=unconfined \
        --cap-add=NET_ADMIN \
        --device /dev/net/tun \
        -e "PUID=${PUID_VALUE}" \
        -e "PGID=${PGID_VALUE}" \
        -e "TZ=${TZ_VALUE}" \
        -e "RESOLUTION=${RESOLUTION}" \
        -p "${WEB_PORT}:3000" \
        -p "${API_PORT}:2022" \
        -p "${PROXY_PORT}:2023" \
        -v "${CONFIG_DIR}:/config" \
        "$IMAGE"
)"

echo "Started ${NAME}: ${container_id}"
echo "Image: ${IMAGE}"
echo "Web desktop: http://127.0.0.1:${WEB_PORT}"
echo "API: http://127.0.0.1:${API_PORT}"
echo "Proxy: 127.0.0.1:${PROXY_PORT}"
echo "Config directory: ${CONFIG_DIR}"
