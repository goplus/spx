//go:build js && wasm

package main

import "github.com/goplus/spx/v2/pkg/ispx"

func main() {
	if err := ispx.Init(nil, nil); err != nil {
		panic(err)
	}
	select {}
}
