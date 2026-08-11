#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# Load server settings from the root .env. This script intentionally does not
# use SERVER_PASSWORD; run it in a terminal and type the SSH password manually.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

: "${SERVER_HOST:?Need to set SERVER_HOST in .env}"
: "${SERVER_PORT:=22}"
: "${SERVER_DIR:?Need to set SERVER_DIR in .env}"

BINARY_NAME="${MIGRATION_BINARY_NAME:-migrate-gopeed}"
GOARCH_TARGET="${MIGRATION_GOARCH:-amd64}"
GOOS_TARGET="${MIGRATION_GOOS:-linux}"
REMOTE_DIR="${MIGRATION_REMOTE_DIR:-$SERVER_DIR}"
DIST_DIR="dist/${BINARY_NAME}-${GOOS_TARGET}-${GOARCH_TARGET}"
BINARY_PATH="${DIST_DIR}/${BINARY_NAME}"
BUILD_TAGS="${MIGRATION_BUILD_TAGS:-embed_frontend_inject}"

SERVER_HOST="${SERVER_HOST#http://}"
SERVER_HOST="${SERVER_HOST#https://}"
SERVER_HOST="${SERVER_HOST%/}"
SERVER_TARGET="$SERVER_HOST"
if [ -n "${SERVER_USER:-}" ] && [[ "$SERVER_TARGET" != *@* ]]; then
  SERVER_TARGET="${SERVER_USER}@${SERVER_TARGET}"
fi

SSH_OPTS=("-p" "$SERVER_PORT")
SCP_OPTS=("-P" "$SERVER_PORT" "-O")
if [ -n "${SERVER_SSH_KEY:-}" ]; then
  SSH_OPTS+=("-i" "$SERVER_SSH_KEY")
  SCP_OPTS+=("-i" "$SERVER_SSH_KEY")
fi

mkdir -p "$DIST_DIR"

GIT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "Building ${BINARY_NAME} for ${GOOS_TARGET}/${GOARCH_TARGET} (${GIT_COMMIT}, ${BUILD_TIME}, tags: ${BUILD_TAGS})..."
(
  cd migrate-gopeed
  CGO_ENABLED=0 GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" \
    go build -trimpath -tags "$BUILD_TAGS" -ldflags="-s -w" -o "../${BINARY_PATH}" .
)

if [ ! -f "$BINARY_PATH" ]; then
  echo "Build failed: ${BINARY_PATH} was not created" >&2
  exit 1
fi

echo "Built binary:"
ls -lh "$BINARY_PATH"
if command -v file >/dev/null 2>&1; then
  file "$BINARY_PATH"
fi

SOCKET_DIR="/tmp/${BINARY_NAME}_deploy_$$"
SOCKET_PATH="$SOCKET_DIR/socket"
mkdir -p "$SOCKET_DIR"

cleanup() {
  ssh -S "$SOCKET_PATH" -O exit "$SERVER_TARGET" >/dev/null 2>&1 || true
  rm -rf "$SOCKET_DIR"
}
trap cleanup EXIT

echo "Connecting to ${SERVER_TARGET}..."
ssh -M -S "$SOCKET_PATH" -fN "${SSH_OPTS[@]}" "$SERVER_TARGET"
SSH_OPTS+=("-o" "ControlPath=$SOCKET_PATH")
SCP_OPTS+=("-o" "ControlPath=$SOCKET_PATH")

echo "Preparing remote directory: ${REMOTE_DIR}"
ssh "${SSH_OPTS[@]}" "$SERVER_TARGET" "mkdir -p '$REMOTE_DIR'"

echo "Uploading ${BINARY_PATH}..."
scp "${SCP_OPTS[@]}" "$BINARY_PATH" "$SERVER_TARGET:$REMOTE_DIR/${BINARY_NAME}.new"

echo "Replacing remote binary..."
ssh "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd '$REMOTE_DIR' && mv '${BINARY_NAME}.new' '$BINARY_NAME' && chmod +x '$BINARY_NAME' && ls -lh '$BINARY_NAME' && (command -v file >/dev/null 2>&1 && file '$BINARY_NAME' || true)"

echo "Done."
echo "Remote binary: ${SERVER_TARGET}:${REMOTE_DIR}/${BINARY_NAME}"
echo "Run example:"
echo "  ssh -p ${SERVER_PORT} ${SERVER_TARGET} \"cd '${REMOTE_DIR}' && ./${BINARY_NAME}\""
