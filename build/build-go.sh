#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$SCRIPT_DIR/tools"

# Build the helper for the host before running it with the target build
# environment. `go run` honors GOOS/GOARCH, so cross-compilation would
# otherwise create a helper binary that cannot run on the host.
HOST_GOOS="$(go env GOHOSTOS)"
HOST_GOARCH="$(go env GOHOSTARCH)"
BUILDGO_DIR="$(mktemp -d "${TMPDIR:-/tmp}/wx-buildgo.XXXXXX")"
BUILDGO_BIN="$BUILDGO_DIR/buildgo"

cleanup() {
    rm -rf "$BUILDGO_DIR"
}
trap cleanup EXIT

env -u GOOS -u GOARCH -u CC \
    CGO_ENABLED=0 GOOS="$HOST_GOOS" GOARCH="$HOST_GOARCH" \
    go build -o "$BUILDGO_BIN" ./cmd/buildgo

"$BUILDGO_BIN" -root "$PROJECT_ROOT" -- "$@"
