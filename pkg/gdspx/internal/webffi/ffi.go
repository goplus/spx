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

func Link() bool {
	js.Global().Set("go_wasm_init", js.FuncOf(goWasmInit))
	registerCallbackDispatcher()
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

// goWasmInit is only called in worker mode.
func goWasmInit(this js.Value, args []js.Value) any {
	spxlog.Info("Go wasm init success!")
	hasInitEngine = true
	registerCallbackDispatcher()
	return js.ValueOf(nil)
}
