#!/bin/sh
GOOS=js GOARCH=wasm go build -tags canvas -o main.wasm main.go