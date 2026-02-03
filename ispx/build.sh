#!/bin/bash
set -e

cd "$(dirname "${BASH_SOURCE[0]}")"

GOPATH="$(go env GOPATH)"

# Build Wasm
echo "Building ispx.wasm..."
GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w -checklinkname=0" -o ispx.wasm ./cmd/ispx
cp ispx.wasm "$GOPATH/bin/ispx.wasm"
echo "Installed ispx.wasm to $GOPATH/bin/"

# Build PC shared library
echo "Building PC shared library..."
GOARCH="$(go env GOARCH)"
case "$(uname -s)" in
    Darwin)
        LIB_NAME="gdspx-darwin-${GOARCH}.dylib"
        LDFLAGS="-checklinkname=0"
        ;;
    Linux)
        LIB_NAME="gdspx-linux-${GOARCH}.so"
        LDFLAGS="-checklinkname=0 -extldflags=-Wl,--allow-multiple-definition"
        ;;
    MINGW*|CYGWIN*|MSYS*)
        LIB_NAME="gdspx-windows-${GOARCH}.dll"
        LDFLAGS="-checklinkname=0 -extldflags=-Wl,--allow-multiple-definition"
        ;;
    *)
        echo "Unsupported OS: $(uname -s)"
        exit 1
        ;;
esac
go build -buildmode c-shared -ldflags "$LDFLAGS" -o "$LIB_NAME" ./cmd/ispxpc
cp "$LIB_NAME" "$GOPATH/bin/"
echo "Installed $LIB_NAME to $GOPATH/bin/"

echo "Done!"
