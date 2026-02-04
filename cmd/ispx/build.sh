#!/bin/bash
set -e

GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w -checklinkname=0" -o ispx.wasm
