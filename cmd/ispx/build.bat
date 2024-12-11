@echo off
setlocal

set GOOS=js
set GOARCH=wasm

go build -tags canvas -o main.wasm main.go

endlocal
