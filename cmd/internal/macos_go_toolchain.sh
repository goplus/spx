#!/bin/bash

configure_macos_go_toolchain() {
    if [ "$(go env GOOS)" != "darwin" ]; then
        return 0
    fi

    # Use Apple's SDK-aware compiler by default. This avoids stale Homebrew LLVM
    # sysroot configs after macOS or Command Line Tools upgrades.
    if [ -z "${CC:-}" ]; then
        export CC="$(xcrun --find clang)"
    fi
    if [ -z "${CXX:-}" ]; then
        export CXX="$(xcrun --find clang++)"
    fi
    if [ -z "${SDKROOT:-}" ] || [ ! -d "${SDKROOT}" ]; then
        export SDKROOT="$(xcrun --show-sdk-path)"
    fi
}
