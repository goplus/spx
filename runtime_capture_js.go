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

package spx

import (
	"fmt"
	"syscall/js"
)

func captureFramePlatform(name string, check bool) error {
	global := js.Global()
	if fn := captureBridgeFunc(global, check); fn.Truthy() {
		return captureBridgeResultError(fn.Invoke(name))
	}
	if fn := global.Get("__spxCaptureFrame"); fn.Truthy() && fn.Type() == js.TypeFunction {
		return captureBridgeResultError(fn.Invoke(name, check))
	}
	if fn := global.Get("spxCaptureFrame"); fn.Truthy() && fn.Type() == js.TypeFunction {
		return captureBridgeResultError(fn.Invoke(name, check))
	}
	if callMainThread := global.Get("callMainThread"); callMainThread.Truthy() && callMainThread.Type() == js.TypeFunction {
		callMainThread.Invoke(captureMainThreadHandler(check), []any{name})
		return nil
	}
	return fmt.Errorf("spx screenshot bridge is not installed")
}

func captureBridgeFunc(global js.Value, check bool) js.Value {
	names := []string{"__spxCapture", "spxCapture"}
	if check {
		names = []string{"__spxCaptureAndCheck", "spxCaptureAndCheck"}
	}
	for _, name := range names {
		fn := global.Get(name)
		if fn.Truthy() && fn.Type() == js.TypeFunction {
			return fn
		}
	}
	return js.Undefined()
}

func captureMainThreadHandler(check bool) string {
	if check {
		return "spx_capture_and_check"
	}
	return "spx_capture"
}

func captureBridgeResultError(ret js.Value) error {
	switch ret.Type() {
	case js.TypeUndefined, js.TypeNull:
		return nil
	case js.TypeBoolean:
		if ret.Bool() {
			return nil
		}
		return fmt.Errorf("spx screenshot bridge returned false")
	case js.TypeString:
		if msg := ret.String(); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return nil
	case js.TypeObject:
		if ok := ret.Get("ok"); ok.Type() == js.TypeBoolean && ok.Bool() {
			return nil
		}
		if msg := ret.Get("error"); msg.Type() == js.TypeString && msg.String() != "" {
			return fmt.Errorf("%s", msg.String())
		}
		if msg := ret.Get("message"); msg.Type() == js.TypeString && msg.String() != "" {
			return fmt.Errorf("%s", msg.String())
		}
		return nil
	default:
		return nil
	}
}
