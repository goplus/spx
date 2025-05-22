#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR
go install github.com/goplus/igop/cmd/qexp@latest
GOOS=js GOARCH=wasm go generate -v
go mod tidy
if [ "$1" = "--opt" ]; then
    GOOS=js GOARCH=wasm go build -tags canvas  -trimpath -ldflags "-s -w -checklinkname=0 " -o gdspx_raw.wasm
    wasm-opt -Oz --enable-bulk-memory -o gdspx.wasm gdspx_raw.wasm
else 
    GOOS=js GOARCH=wasm go build -tags canvas -ldflags -checklinkname=0  -o gdspx.wasm
fi 
echo "gdspx.wasm has been created"