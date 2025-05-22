#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $SCRIPT_DIR

go mod tidy
if [ "$2" = "--opt" ]; then
    GOOS=js GOARCH=wasm go build -tags canvas  -trimpath -ldflags "-s -w -checklinkname=0 " -o gdspx_raw.wasm
    wasm-opt -Oz --enable-bulk-memory -o gdspx.wasm gdspx_raw.wasm
else 
    GOOS=js GOARCH=wasm go build -tags canvas -ldflags -checklinkname=0  -o gdspx.wasm
fi 
echo "gdspx.wasm has been created"