//go:build js

package webffi

import (
	"syscall/js"

	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

var (
	callbacks                engine.CallbackInfo
	hasInitEngine            bool
	exitChan                 chan struct{}
	goWasmInitCallbackHandle js.Func
	callbackDispatcherHandle js.Func
)

func Link() bool {
	registerWebGlobals()
	API.resolveAPIFunctions()
	return !hasInitEngine
}
func Linked() {
	if !hasInitEngine { // adapt for ixgo
		gdspxDispatch(js.Value{}, []js.Value{jsEventOnEngineStart})
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

func BindCallback(info engine.CallbackInfo) {
	callbacks = info
}

func resolveJSFunc(funcName string) js.Value {
	val := js.Global().Get(funcName)
	if val.IsUndefined() || val.IsNull() {
		panic("JS function not found: " + funcName)
	}
	return val
}

func registerWebGlobals() {
	if goWasmInitCallbackHandle.Type() == js.TypeUndefined {
		goWasmInitCallbackHandle = js.FuncOf(goWasmInit)
		js.Global().Set("go_wasm_init", goWasmInitCallbackHandle)
	}
	registerCallbackDispatcher()
}

func registerCallbackDispatcher() {
	if callbackDispatcherHandle.Type() == js.TypeUndefined {
		callbackDispatcherHandle = js.FuncOf(gdspxDispatch)
		js.Global().Set("gdspx_dispatch", callbackDispatcherHandle)
	}
}

// goWasmInit is only called in worker mode.
func goWasmInit(this js.Value, args []js.Value) any {
	spxlog.Info("Go wasm init success!")
	hasInitEngine = true
	return js.ValueOf(nil)
}
