#!/bin/bash

set -e

PLATFORM="${1:-$(uname -s)}"
ZIP_ENABLED=false
OUTPUT_DIR="${OUTPUT_DIR:-.}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
APP_VERSION="$(sed -n 's/^var AppVer = "\([^"]*\)".*/\1/p' "$PROJECT_ROOT/main.go")"
RELEASE_VERSION="${VERSION:-$APP_VERSION}"

if [[ -z "$RELEASE_VERSION" ]]; then
    echo "Unable to determine the release version from main.go"
    exit 1
fi

if [[ "$RELEASE_VERSION" != v* ]]; then
    RELEASE_VERSION="v$RELEASE_VERSION"
fi

if [[ ! "$RELEASE_VERSION" =~ ^v[0-9A-Za-z._-]+$ ]]; then
    echo "Invalid release version: $RELEASE_VERSION"
    exit 1
fi

case "$PLATFORM" in
    Darwin)
        PLATFORM="darwin"
        ;;
    Linux)
        PLATFORM="linux"
        ;;
esac

show_usage() {
    echo "Usage: $0 [platform] [--zip|--no-zip]"
    echo "  windows          - Windows x86_64"
    echo "  windows-sunnynet - Windows SunnyNet (requires Docker)"
    echo "  macos            - macOS current architecture"
    echo "  macos-x86_64     - macOS x86_64"
    echo "  macos-arm64      - macOS arm64"
    echo "  linux            - Linux x86_64"
    echo "  linux-arm64      - Linux arm64"
    echo "  all              - Build all platforms"
    echo "  --zip            - Generate a ZIP archive after building"
    echo "  --no-zip         - Only generate the binary (default)"
}

case "${2:-}" in
    --zip)
        ZIP_ENABLED=true
        ;;
    ""|--no-zip)
        ;;
    *)
        echo "Unknown option: $2"
        show_usage
        exit 1
        ;;
esac

if [[ $# -gt 2 ]]; then
    show_usage
    exit 1
fi

mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd)"

package_zip() {
    local binary_path="$1"
    local platform_name="$2"
    local archive_prefix="${3:-wx_video_download}"
    local binary_name
    local archive_name
    local archive_path
    local package_dir

    if [[ "$ZIP_ENABLED" != true ]]; then
        return
    fi

    binary_name="$(basename "$binary_path")"
    archive_name="${archive_prefix}_${RELEASE_VERSION}_${platform_name}.zip"
    archive_path="$OUTPUT_DIR/$archive_name"
    package_dir="$(mktemp -d)"

    (
        trap 'rm -rf "$package_dir"' EXIT

        cp "$binary_path" "$package_dir/$binary_name"
        cp "$PROJECT_ROOT/internal/config/config.template.yaml" "$package_dir/config.yaml"
        cp "$PROJECT_ROOT/LICENSE" "$package_dir/LICENSE"

        cd "$package_dir"
        zip -q "$archive_name" "$binary_name" config.yaml LICENSE
        mv "$archive_name" "$archive_path"
    )

    echo "Done: $archive_path"
}

build_windows() {
    local binary_path="$OUTPUT_DIR/wx_video_download_windows_x86_64.exe"

    echo "Building Windows x86_64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 bash "$SCRIPT_DIR/build-go.sh" -trimpath -tags "with_gvisor,embed_inject,sqlite_only,embed_frontend_inject" -ldflags="-s -w -X main.Mode=release" -o "$binary_path"
    echo "Done: $binary_path"
    package_zip "$binary_path" "windows_x86_64"
}

build_windows_sunnynet() {
    local built_binary="$(pwd)/wx_video_download_sunnynet.exe"
    local binary_path="$OUTPUT_DIR/wx_video_download_sunnynet.exe"

    echo "Building Windows SunnyNet version..."
    go mod vendor

    docker run --rm \
        -v "$(pwd):/workspace" \
        -w /workspace \
        golang:1.20 \
        bash -c '
            apt-get update && apt-get install -y gcc-mingw-w64 g++-mingw-w64

            SUNNYNET_DIR="vendor/github.com/qtgolang/SunnyNet"
            mv $SUNNYNET_DIR/src/ProcessDrv/Proxifier/proxifier.hpp $SUNNYNET_DIR/src/ProcessDrv/Proxifier/Proxifier.hpp
            sed -i "s/typedef struct _MIB_TCPROW2 {/typedef struct _MIB_TCPROW2_CUSTOM {/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.h
            sed -i "s/} MIB_TCPROW2, \*PMIB_TCPROW2;/} MIB_TCPROW2_CUSTOM, *PMIB_TCPROW2_CUSTOM;/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.h
            sed -i "s/typedef struct _MIB_TCPTABLE2 {/typedef struct _MIB_TCPTABLE2_CUSTOM {/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.h
            sed -i "s/MIB_TCPROW2 table\[ANY_SIZE\];/MIB_TCPROW2_CUSTOM table[ANY_SIZE];/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.h
            sed -i "s/} MIB_TCPTABLE2, \*PMIB_TCPTABLE2;/} MIB_TCPTABLE2_CUSTOM, *PMIB_TCPTABLE2_CUSTOM;/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.h
            sed -i "s/typedef DWORD (WINAPI \* GetTcpTable2)(PMIB_TCPTABLE2 TcpTable, PULONG SizePointer, BOOL Order);/typedef DWORD (WINAPI * GetTcpTable2_CUSTOM)(PMIB_TCPTABLE2_CUSTOM TcpTable, PULONG SizePointer, BOOL Order);/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.h
            sed -i "s/GetTcpTable2 pGetTcpTable2;/GetTcpTable2_CUSTOM pGetTcpTable2;/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/(GetTcpTable2)/(GetTcpTable2_CUSTOM)/g" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/(PMIB_TCPTABLE2)/(PMIB_TCPTABLE2_CUSTOM)/g" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/PMIB_TCPTABLE2 pTcpTable;/PMIB_TCPTABLE2_CUSTOM pTcpTable;/" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/PMIB_TCPTABLE2 tcpTable/PMIB_TCPTABLE2_CUSTOM tcpTable/g" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/pTcpTable = (MIB_TCPTABLE2\*)/pTcpTable = (MIB_TCPTABLE2_CUSTOM*)/g" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/tcpTable = (MIB_TCPTABLE2\*)/tcpTable = (MIB_TCPTABLE2_CUSTOM*)/g" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c
            sed -i "s/MIB_TCPROW2 \*row/MIB_TCPROW2_CUSTOM *row/g" $SUNNYNET_DIR/src/iphlpapi/c_iphlpapi_tcp.c

            CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
            GOOS=windows GOARCH=amd64 \
            bash build/build-go.sh -mod=vendor -tags "sunnynet,embed_inject,embed_frontend_inject" -ldflags "-s -w -X main.Mode=release -extldflags \"-static\"" \
            -o wx_video_download_sunnynet.exe .
        '
    if [[ "$built_binary" != "$binary_path" ]]; then
        mv "$built_binary" "$binary_path"
    fi
    echo "Done: $binary_path"
    package_zip "$binary_path" "windows_x86_64" "wx_video_download_sunnynet"
}

build_macos_target() {
    local go_arch="$1"
    local archive_arch="$2"
    local binary_name="$3"
    local binary_path="$OUTPUT_DIR/$binary_name"

    echo "Building macOS $archive_arch..."
    CGO_ENABLED=1 GOOS=darwin GOARCH="$go_arch" SDKROOT=$(xcrun --sdk macosx --show-sdk-path) bash "$SCRIPT_DIR/build-go.sh" -trimpath -tags "embed_inject,sqlite_only,embed_frontend_inject" -ldflags="-s -w -X main.Mode=release" -o "$binary_path"
    echo "Done: $binary_path"
    package_zip "$binary_path" "darwin_$archive_arch"
}

build_macos() {
    local machine_arch

    machine_arch="$(uname -m)"
    case "$machine_arch" in
        arm64|aarch64)
            build_macos_target "arm64" "arm64" "wx_video_download_macos"
            ;;
        x86_64|amd64)
            build_macos_target "amd64" "x86_64" "wx_video_download_macos"
            ;;
        *)
            echo "Unsupported macOS architecture: $machine_arch"
            return 1
            ;;
    esac
}

build_macos_x86_64() {
    build_macos_target "amd64" "x86_64" "wx_video_download_macos"
}

build_macos_arm64() {
    build_macos_target "arm64" "arm64" "wx_video_download_macos_arm64"
}

build_linux() {
    local binary_path="$OUTPUT_DIR/wx_video_download_linux"

    echo "Building Linux x86_64..."
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 bash "$SCRIPT_DIR/build-go.sh" -trimpath -tags "embed_inject,sqlite_only,embed_frontend_inject" -ldflags="-s -w -X main.Mode=release" -o "$binary_path"
    echo "Done: $binary_path"
    package_zip "$binary_path" "linux_x86_64"
}

build_linux_arm64() {
    local binary_path="$OUTPUT_DIR/wx_video_download_linux_arm64"

    echo "Building Linux arm64..."
    CGO_ENABLED=1 GOOS=linux GOARCH=arm64 bash "$SCRIPT_DIR/build-go.sh" -trimpath -tags "embed_inject,sqlite_only,embed_frontend_inject" -ldflags="-s -w -X main.Mode=release" -o "$binary_path"
    echo "Done: $binary_path"
    package_zip "$binary_path" "linux_arm64"
}

case "$PLATFORM" in
    windows|win)
        build_windows
        ;;
    windows-sunnynet)
        build_windows_sunnynet
        ;;
    macos|darwin)
        build_macos
        ;;
    macos-x86_64|darwin-x86_64)
        build_macos_x86_64
        ;;
    macos-arm64|darwin-arm64)
        build_macos_arm64
        ;;
    linux)
        build_linux
        ;;
    linux-arm64)
        build_linux_arm64
        ;;
    all)
        echo "Building all platforms..."
        build_windows
        build_macos_x86_64
        build_macos_arm64
        build_linux
        build_linux_arm64
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
