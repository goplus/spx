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

	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type LinkSession struct {
	exit chan struct{}
}

var (
	callbacks engine.CallbackInfo

	// Set by the worker's go_wasm_init handshake.
	workerHandshakeComplete  bool
	goWasmInitCallbackHandle js.Func
	callbackDispatcherHandle js.Func
)

func Link() *LinkSession {
	registerWebGlobals()
	API.resolveAPIFunctions()
	return &LinkSession{exit: make(chan struct{})}
}

func (s *LinkSession) Run(ready func()) {
	if !workerHandshakeComplete {
		// Non-worker startup needs an explicit start event.
		gdspxDispatch(js.Value{}, []js.Value{jsEventOnEngineStart})
	}
	ready()
	<-s.exit
}

func (s *LinkSession) Unlink() {
	close(s.exit)
	workerHandshakeComplete = false
}

func BindCallbacks(info engine.CallbackInfo) {
	contactEventGeneration++
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
	registerContactEventDispatcher()
}

func registerCallbackDispatcher() {
	if callbackDispatcherHandle.Type() == js.TypeUndefined {
		callbackDispatcherHandle = js.FuncOf(gdspxDispatch)
		js.Global().Set("gdspx_dispatch", callbackDispatcherHandle)
	}
}

// goWasmInit is only called in worker mode.
func goWasmInit(this js.Value, args []js.Value) any {
	spxlog.Info("WASM initialized")
	workerHandshakeComplete = true
	return js.ValueOf(nil)
}
