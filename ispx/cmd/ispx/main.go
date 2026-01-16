//go:build js && wasm

package main

import "github.com/goplus/spx/ispx"

func main() {
	if err := ispx.Init(nil); err != nil {
		panic(err)
	}
	select {}
}
