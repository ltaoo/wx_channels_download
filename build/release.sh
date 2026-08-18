#!/bin/bash

set -e

cd "$(dirname "$0")/.."

# Load environment variables from .env file
if [ -f .env ]; then
  set -a
  source .env
  set +a
fi

# Configuration
: "${SERVER_HOST:?Need to set SERVER_HOST in .env}"
: "${SERVER_PORT:=22}"
: "${SERVER_DIR:?Need to set SERVER_DIR in .env}"

BINARY_NAME="wx_video_download"
PID_FILE="${BINARY_NAME}.pid"
BUILD_IMAGE="${SERVER_BUILD_IMAGE:-golang:1.20-bullseye}"
SERVER_HOST="${SERVER_HOST#http://}"
SERVER_HOST="${SERVER_HOST#https://}"
SERVER_HOST="${SERVER_HOST%/}"
SERVER_TARGET="$SERVER_HOST"
if [ -n "${SERVER_USER:-}" ] && [[ "$SERVER_TARGET" != *@* ]]; then
  SERVER_TARGET="${SERVER_USER}@${SERVER_TARGET}"
fi

mkdir -p dist

# Build
APP_VER=$(grep 'var AppVer' main.go | cut -d'"' -f2)
GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_VERSION="${APP_VER}-${GIT_COMMIT}"

echo "🚧 Building for Linux/amd64 (Version: ${BUILD_VERSION})..."
build_binary() {
  local ldflags="-s -w -linkmode external -extldflags=-static -X main.AppVer=${BUILD_VERSION} -X main.Mode=release"
  local tags="with_gvisor,embed_frontend_inject"
  local docker_mode="${SERVER_BUILD_WITH_DOCKER:-auto}"
  local docker_ready="false"
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    docker_ready="true"
  fi

  docker_build() {
    docker run --rm --platform linux/amd64 \
      -v "$PWD:/workspace" \
      -w /workspace \
      "$BUILD_IMAGE" \
      bash -lc "CGO_ENABLED=1 GOOS=linux GOARCH=amd64 bash build/build-go.sh -trimpath -tags '$tags' -ldflags '$ldflags' -o 'dist/$BINARY_NAME' main.go"
  }

  go_build() {
    local cc="${1:-}"
    if [ -n "$cc" ]; then
      echo "   Using CGO_ENABLED=1 with CC=$cc"
      CC="$cc" CGO_ENABLED=1 GOOS=linux GOARCH=amd64 bash build/build-go.sh -trimpath -tags "$tags" -ldflags="$ldflags" -o "dist/$BINARY_NAME" main.go
    else
      echo "   Using CGO_ENABLED=1"
      CGO_ENABLED=1 GOOS=linux GOARCH=amd64 bash build/build-go.sh -trimpath -tags "$tags" -ldflags="$ldflags" -o "dist/$BINARY_NAME" main.go
    fi
  }

  if [ "$docker_mode" = "1" ]; then
    if [ "$docker_ready" != "true" ]; then
      echo "❌ SERVER_BUILD_WITH_DOCKER=1 but Docker is not running."
      exit 1
    fi
    docker_build
    return
  fi

  if [ -n "${SERVER_CC:-}" ]; then
    go_build "$SERVER_CC"
    return
  fi

  if [ "$(go env GOOS)" = "linux" ] && [ "$(go env GOARCH)" = "amd64" ]; then
    go_build
    return
  fi

  if command -v x86_64-linux-gnu-gcc >/dev/null 2>&1; then
    go_build "x86_64-linux-gnu-gcc"
    return
  fi
  if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then
    go_build "x86_64-linux-musl-gcc"
    return
  fi
  if command -v zig >/dev/null 2>&1; then
    go_build "zig cc -target x86_64-linux-gnu"
    return
  fi

  if [ "$docker_mode" != "0" ] && [ "$docker_ready" = "true" ]; then
    docker_build
    return
  fi

  echo "❌ Linux/amd64 build requires CGO_ENABLED=1 and a Linux C toolchain."
  echo "   Use one of these:"
  echo "   - Start Docker/OrbStack and retry"
  echo "   - Install zig"
  echo "   - Set SERVER_CC to a Linux amd64 cross compiler"
  echo "   - Run this script on a Linux/amd64 host"
  exit 1
}

build_binary

if [ ! -f "dist/$BINARY_NAME" ]; then
    echo "❌ Build failed!"
    exit 1
fi
if command -v file >/dev/null 2>&1; then
  file "dist/$BINARY_NAME"
fi

# SSH options
SSH_CMD=(ssh)
SCP_CMD=(scp -O)
SSH_OPTS=()

if [ -n "${SERVER_PASSWORD:-}" ]; then
  if command -v sshpass >/dev/null 2>&1; then
    export SSHPASS="$SERVER_PASSWORD"
    SSH_CMD=(sshpass -e ssh)
    SCP_CMD=(sshpass -e scp -O)
  else
    echo "⚠️ SERVER_PASSWORD is set but sshpass is not installed; falling back to SSH prompt."
  fi
fi

if [ -n "${SERVER_SSH_KEY:-}" ]; then
  SSH_OPTS+=("-i" "$SERVER_SSH_KEY")
fi

# SSH multiplexing
SOCKET_DIR="/tmp/wx_deploy_$$"
mkdir -p "$SOCKET_DIR"
SOCKET_PATH="$SOCKET_DIR/socket"

cleanup() {
    "${SSH_CMD[@]}" -S "$SOCKET_PATH" -O exit "$SERVER_TARGET" >/dev/null 2>&1 || true
    rm -rf "$SOCKET_DIR"
}
trap cleanup EXIT

echo "🔌 Connecting to $SERVER_TARGET..."
"${SSH_CMD[@]}" -M -S "$SOCKET_PATH" -fN -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET"
SSH_OPTS+=("-o" "ControlPath=$SOCKET_PATH")

echo "📁 Preparing remote directory..."
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "mkdir -p $SERVER_DIR"

# Upload as .new
echo "🚀 Uploading to server..."
"${SCP_CMD[@]}" -P "$SERVER_PORT" "${SSH_OPTS[@]}" "dist/$BINARY_NAME" "$SERVER_TARGET:$SERVER_DIR/${BINARY_NAME}.new"

# Stop old service
echo "🛑 Stopping old service..."
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd $SERVER_DIR && BINARY_NAME='$BINARY_NAME' PID_FILE='$PID_FILE'; if [ -x ./\$BINARY_NAME ]; then ./\$BINARY_NAME server stop >/dev/null 2>&1 || true; else echo 'ℹ️ No existing binary, skip stop command'; fi; PID=\$(cat \"\$PID_FILE\" 2>/dev/null || true); case \"\$PID\" in ''|*[!0-9]*) exit 0;; esac; if kill -0 \"\$PID\" 2>/dev/null; then kill -TERM \"\$PID\" 2>/dev/null || true; fi"

# Wait for process to exit
echo "⏳ Waiting for process to exit..."
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd $SERVER_DIR && PID_FILE='$PID_FILE'; PID=\$(cat \"\$PID_FILE\" 2>/dev/null || true); case \"\$PID\" in ''|*[!0-9]*) exit 0;; esac; i=0; while [ \"\$i\" -lt 20 ]; do kill -0 \"\$PID\" 2>/dev/null || { rm -f \"\$PID_FILE\"; exit 0; }; sleep 0.5; i=\$((i + 1)); done; echo '❌ Process still running, force kill'; kill -KILL \"\$PID\" 2>/dev/null || true; rm -f \"\$PID_FILE\"; sleep 1"

# Rename and set permission
echo "🔄 Replacing binary..."
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd $SERVER_DIR && mv ${BINARY_NAME}.new $BINARY_NAME && chmod +x $BINARY_NAME"

echo "🔎 Inspecting remote binary..."
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd $SERVER_DIR && ls -lh $BINARY_NAME && uname -m && (command -v file >/dev/null 2>&1 && file $BINARY_NAME || true)"

# Start new service
echo "🚀 Starting service..."
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd $SERVER_DIR && nohup ./$BINARY_NAME server > server.log 2>&1 < /dev/null &"

# Health check
echo "🔍 Checking status..."
sleep 2
"${SSH_CMD[@]}" -p "$SERVER_PORT" "${SSH_OPTS[@]}" "$SERVER_TARGET" "cd $SERVER_DIR && ./$BINARY_NAME server status --quiet || (echo '❌ Failed! Check logs:'; cat server.log 2>/dev/null || true; echo; echo 'Status:'; ./$BINARY_NAME server status || true; exit 1)"

echo "✅ Done!"
