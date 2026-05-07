#!/bin/bash

set -e

PLATFORM="${1:-$(uname -s)}"
OUTPUT_DIR="${OUTPUT_DIR:-.}"

build_windows() {
    echo "Building Windows x86_64..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$OUTPUT_DIR/wx_video_download_windows_x86_64.exe"
    echo "Done: $OUTPUT_DIR/wx_video_download_windows_x86_64.exe"
}

build_windows_sunnynet() {
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
            go build -mod=vendor -tags sunnynet -ldflags "-s -w -extldflags \"-static\"" \
            -o wx_video_download_sunnynet.exe .
        '
    echo "Done: wx_video_download_sunnynet.exe"
}

build_macos() {
    echo "Building macOS..."
    CGO_ENABLED=1 GOOS=darwin SDKROOT=$(xcrun --sdk macosx --show-sdk-path) go build -trimpath -ldflags="-s -w" -o "$OUTPUT_DIR/wx_video_download_macos"
    echo "Done: $OUTPUT_DIR/wx_video_download_macos"
}

build_macos_arm64() {
    echo "Building macOS arm64..."
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 SDKROOT=$(xcrun --sdk macosx --show-sdk-path) go build -trimpath -ldflags="-s -w" -o "$OUTPUT_DIR/wx_video_download_macos_arm64"
    echo "Done: $OUTPUT_DIR/wx_video_download_macos_arm64"
}

build_linux() {
    echo "Building Linux x86_64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$OUTPUT_DIR/wx_video_download_linux"
    echo "Done: $OUTPUT_DIR/wx_video_download_linux"
}

build_linux_arm64() {
    echo "Building Linux arm64..."
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o "$OUTPUT_DIR/wx_video_download_linux_arm64"
    echo "Done: $OUTPUT_DIR/wx_video_download_linux_arm64"
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
        build_macos
        build_macos_arm64
        build_linux
        build_linux_arm64
        ;;
    *)
        echo "Usage: $0 [platform]"
        echo "  windows         - Windows x86_64"
        echo "  windows-sunnynet - Windows SunnyNet (requires Docker)"
        echo "  macos          - macOS x86_64"
        echo "  macos-arm64   - macOS arm64"
        echo "  linux         - Linux x86_64"
        echo "  linux-arm64   - Linux arm64"
        echo "  all           - Build all platforms"
        exit 1
        ;;
esac