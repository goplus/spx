#!/bin/bash
set -e

# Build the WASM binary
GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" -o ispx.wasm
