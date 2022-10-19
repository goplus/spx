//go:build js && !wasm
// +build js,!wasm

package spx

import (
	_ "github.com/visualfc/gopherjs-fixed/audio"
	_ "github.com/visualfc/gopherjs-fixed/ebiten"
	_ "github.com/visualfc/gopherjs-fixed/oto"
)
