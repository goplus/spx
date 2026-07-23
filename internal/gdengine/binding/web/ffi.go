//go:build js

/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package webffi

import (
	"syscall/js"

	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
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
	registerContactEventQueue()
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
