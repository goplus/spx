# ispx for Go+ Builder

## Introduction

This package (ispx) is forked from [goplus/ispx](https://github.com/goplus/ispx) with some modification for fs, so that we can run a project packed in a zip file.

## Upgrade deps

If we want to upgrade deps like [spx](https://github.com/goplus/spx). First Modify `go.mod` to upgrade dependencies, then do

```sh
go mod tidy
go install github.com/goplus/igop/cmd/qexp@latest # `qexp` is required to do `go generate`
GOOS=js GOARCH=wasm go generate -v . # `qexp` will update `pkg/github.com/goplus/spx/export.go`, see detail in `main.go` (`//go:generate qexp ...`)
```
