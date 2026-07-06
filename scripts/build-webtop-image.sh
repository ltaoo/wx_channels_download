#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-wx_video_download:v260607}"
WECHAT_DEB="${WECHAT_DEB:-/Users/litao/Downloads/WeChatLinux_arm64.deb}"
TARGETARCH="${TARGETARCH:-arm64}"
PLATFORM="${PLATFORM:-linux/${TARGETARCH}}"
GOCACHE="${GOCACHE:-/tmp/wx-go-build-cache}"
CONFIG_FILE="${CONFIG_FILE:-$ROOT_DIR/internal/config/config.template.yaml}"
GLOBAL_SCRIPT="${GLOBAL_SCRIPT:-}"

if [ "$TARGETARCH" != "arm64" ]; then
    echo "Only TARGETARCH=arm64 is supported because the provided WeChat deb is arm64." >&2
    exit 1
fi

if [ ! -f "$WECHAT_DEB" ]; then
    echo "WeChat deb not found: $WECHAT_DEB" >&2
    exit 1
fi

BUILD_DIR="$(mktemp -d "${TMPDIR:-/tmp}/wx-webtop-build.XXXXXX")"
cleanup() {
    rm -rf "$BUILD_DIR"
}
trap cleanup EXIT

mkdir -p "$BUILD_DIR/rootfs"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Config file not found: $CONFIG_FILE" >&2
    exit 1
fi

if [ -z "$GLOBAL_SCRIPT" ] && [ -f "$ROOT_DIR/global.js" ]; then
    GLOBAL_SCRIPT="$ROOT_DIR/global.js"
fi

if [ -n "$GLOBAL_SCRIPT" ] && [ ! -f "$GLOBAL_SCRIPT" ]; then
    echo "Global script not found: $GLOBAL_SCRIPT" >&2
    exit 1
fi

echo "Building wx_video_download for ${PLATFORM}..."
(
    cd "$ROOT_DIR"
    env GOCACHE="$GOCACHE" CGO_ENABLED=0 GOOS=linux GOARCH="$TARGETARCH" \
        go build -trimpath -tags "with_gvisor,embed_inject" -ldflags="-s -w -X main.Mode=release" \
        -o "$BUILD_DIR/wx_video_download" .
)

cp "$ROOT_DIR/docker/webtop/Dockerfile" "$BUILD_DIR/Dockerfile"
cp "$CONFIG_FILE" "$BUILD_DIR/config.yaml"
cp "$ROOT_DIR/docs/public/SunnyRoot.cer" "$BUILD_DIR/SunnyRoot.cer"
if [ -n "$GLOBAL_SCRIPT" ]; then
    cp "$GLOBAL_SCRIPT" "$BUILD_DIR/global.js"
else
    : > "$BUILD_DIR/global.js"
fi
cp "$WECHAT_DEB" "$BUILD_DIR/WeChatLinux_arm64.deb"
cp -R "$ROOT_DIR/docker/webtop/rootfs/." "$BUILD_DIR/rootfs/"

echo "Building Docker image ${IMAGE}..."
docker build --platform "$PLATFORM" -t "$IMAGE" "$BUILD_DIR"
echo "Built image: ${IMAGE}"
