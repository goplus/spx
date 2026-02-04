#!/bin/bash
set -e

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

LDFLAGS="-checklinkname=0"
if [[ "${GOOS}" != "darwin" ]]; then
    LDFLAGS="${LDFLAGS} -extldflags=-Wl,--allow-multiple-definition"
fi

go build -buildmode c-shared -ldflags "${LDFLAGS}" -o "gdspx-${GOOS}-${GOARCH}.${EXT}"
