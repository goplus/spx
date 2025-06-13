# ispx for XBuilder

## Introduction

This package (ispx) is forked from [goplus/ispx](https://github.com/goplus/ispx) with some modification for fs, so that we can run a project packed in a zip file.

## Upgrade deps

If we want to upgrade deps like [spx](https://github.com/goplus/spx). First Modify `go.mod` to upgrade dependencies, then do

```sh
go mod tidy
GOOS=js GOARCH=wasm go generate -v . # `qexp` will update `pkg/github.com/goplus/spx/export.go`, see detail in `main.go` (`//go:generate qexp ...`)
```
