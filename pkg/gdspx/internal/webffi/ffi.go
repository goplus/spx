package webffi

import (
	"syscall/js"

	"github.com/goplus/spx/pkg/gdspx/pkg/engine"
)

var (
	callbacks     engine.CallbackInfo
	hasInitEngine bool
)

func Link() bool {
	js.Global().Set("goWasmInit", js.FuncOf(goWasmInit))
	API.loadProcAddresses()
	return !hasInitEngine
}
func Linked() {
	if !hasInitEngine { // adapt for igop
		resiterFuncPtr2Js()
		gdspxOnEngineStart(js.Value{}, nil)
	}

	// wasm need Block forever
	c := make(chan struct{})
	<-c
}

// this function will only be called in wasm mode, it will not be called in igop (interpreter) mode.
func goWasmInit(this js.Value, args []js.Value) interface{} {
	println("Go wasm init succ!")
	hasInitEngine = true
	resiterFuncPtr2Js()
	return js.ValueOf(nil)
}

func BindCallback(info engine.CallbackInfo) {
	callbacks = info
}

func dlsymGD(funcName string) js.Value {
	val := js.Global().Get(funcName)
	if val.IsUndefined() || val.IsNull() {
		panic("Js Function not found: " + funcName)
	}
	return val
}
