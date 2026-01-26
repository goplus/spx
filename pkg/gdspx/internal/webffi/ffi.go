package webffi

import (
	"syscall/js"

	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var (
	callbacks     engine.CallbackInfo
	hasInitEngine bool
	exitChan      chan struct{}
)

func RegisterFuncs() {
	registerFuncPtr2Js()
}

func Link() bool {
	js.Global().Set("goWasmInit", js.FuncOf(goWasmInit))
	registerFuncPtr2Js()
	API.loadProcAddresses()
	return !hasInitEngine
}
func Linked() {
	if !hasInitEngine { // adapt for ixgo
		gdspxOnEngineStart(js.Value{}, nil)
	}

	exitChan = make(chan struct{})
	<-exitChan
}

func Unlink() {
	if exitChan != nil {
		close(exitChan)
		exitChan = nil
	}
	hasInitEngine = false
}

// this function will only be called in wasm mode, it will not be called in ixgo (interpreter) mode.
func goWasmInit(this js.Value, args []js.Value) any {
	spxlog.Info("Go wasm init success!")
	hasInitEngine = true
	registerFuncPtr2Js()
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
