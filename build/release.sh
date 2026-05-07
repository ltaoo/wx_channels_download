#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

# Configuration
: "${NAS_HOST:?Need to set NAS_HOST in .env}"
: "${NAS_PORT:=22}"
: "${NAS_DIR:?Need to set NAS_DIR in .env}"

BINARY_NAME="wx_video_download"

set -e

cd "$(dirname "$0")"

mkdir -p dist

# Build
APP_VER=$(grep 'var AppVer' main.go | cut -d'"' -f2)
GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_VERSION="${APP_VER}-${GIT_COMMIT}"

echo "🚧 Building for Linux/amd64 (Version: ${BUILD_VERSION})..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.AppVer=${BUILD_VERSION}" -o dist/$BINARY_NAME main.go

if [ ! -f "dist/$BINARY_NAME" ]; then
    echo "❌ Build failed!"
    exit 1
fi

# SSH options
SSH_OPTS=""
if [ -n "$NAS_SSH_KEY" ]; then
  SSH_OPTS="-i $NAS_SSH_KEY"
fi

# SSH multiplexing
SOCKET_DIR="/tmp/wx_deploy_$$"
mkdir -p "$SOCKET_DIR"
SOCKET_PATH="$SOCKET_DIR/socket"

cleanup() {
    ssh -S "$SOCKET_PATH" -O exit "$NAS_HOST" >/dev/null 2>&1 || true
    rm -rf "$SOCKET_DIR"
}
trap cleanup EXIT

echo "🔌 Connecting to $NAS_HOST..."
ssh -M -S "$SOCKET_PATH" -fN -p "$NAS_PORT" $SSH_OPTS "$NAS_HOST"
SSH_OPTS="$SSH_OPTS -o ControlPath=$SOCKET_PATH"

# Upload as .new
echo "🚀 Uploading to NAS..."
scp -O -P $NAS_PORT $SSH_OPTS dist/$BINARY_NAME $NAS_HOST:$NAS_DIR/${BINARY_NAME}.new

# Stop old service
echo "🛑 Stopping old service..."
ssh -p $NAS_PORT $SSH_OPTS $NAS_HOST "cd $NAS_DIR && ./$BINARY_NAME server stop || true"

# Wait for process to exit
echo "⏳ Waiting for process to exit..."
ssh -p $NAS_PORT $SSH_OPTS $NAS_HOST "for i in \$(seq 1 20); do pgrep -f '$BINARY_NAME' > /dev/null || break; sleep 0.5; done; if pgrep -f '$BINARY_NAME' > /dev/null; then echo '❌ Process still running, force kill'; pkill -9 -f '$BINARY_NAME'; sleep 1; fi"

# Rename and set permission
echo "🔄 Replacing binary..."
ssh -p $NAS_PORT $SSH_OPTS $NAS_HOST "cd $NAS_DIR && mv ${BINARY_NAME}.new $BINARY_NAME && chmod +x $BINARY_NAME"

# Start new service
echo "🚀 Starting service..."
ssh -p $NAS_PORT $SSH_OPTS $NAS_HOST "cd $NAS_DIR && ./$BINARY_NAME server -d"

# Health check
echo "🔍 Checking status..."
sleep 2
ssh -p $NAS_PORT $SSH_OPTS $NAS_HOST "cd $NAS_DIR && ./$BINARY_NAME status || (echo '❌ Failed! Check logs:' && cat server.log 2>/dev/null || true)"

echo "✅ Done!"
