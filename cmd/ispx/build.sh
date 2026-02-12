#!/bin/bash
set -e

# Generate code before building
(cd ../../pkg/ispx && go generate)

# Build the WASM binary
GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w -checklinkname=0" -o ispx.wasm
