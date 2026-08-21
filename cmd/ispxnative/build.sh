#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../internal/macos_go_toolchain.sh"
configure_macos_go_toolchain

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"

case "${GOOS}" in
    darwin)  EXT="dylib" ;;
    linux)   EXT="so" ;;
    windows) EXT="dll" ;;
    *)
        echo "Unsupported OS: ${GOOS}" >&2
        exit 1
        ;;
esac

LDFLAGS=""
if [[ "${GOOS}" == "windows" ]]; then
    LDFLAGS="-extldflags=-Wl,--allow-multiple-definition"
fi

go build -buildmode c-shared -ldflags "${LDFLAGS}" -o "gdspx-${GOOS}-${GOARCH}.${EXT}"
