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

const jsCaptureBridgeName = "__spxCaptureScreenshot"

func init() {
	SetCaptureHandler(captureWithJSBridge)
}

func captureWithJSBridge(name string, check bool) (err error) {
	fn := js.Global().Get(jsCaptureBridgeName)
	if fn.Type() != js.TypeFunction {
		return fmt.Errorf("spx: JS capture bridge %s is not configured", jsCaptureBridgeName)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("spx: JS capture bridge failed: %v", r)
		}
	}()
	return captureBridgeResultError(fn.Invoke(name, check))
}

func captureBridgeResultError(result js.Value) error {
	switch result.Type() {
	case js.TypeUndefined, js.TypeNull:
		return nil
	case js.TypeBoolean:
		if result.Bool() {
			return nil
		}
		return fmt.Errorf("spx: JS capture bridge returned false")
	case js.TypeString:
		if msg := result.String(); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return nil
	case js.TypeObject:
		if msg := result.Get("error"); msg.Type() == js.TypeString && msg.String() != "" {
			return fmt.Errorf("%s", msg.String())
		}
		if ok := result.Get("ok"); ok.Type() == js.TypeBoolean && !ok.Bool() {
			return fmt.Errorf("spx: JS capture bridge returned ok=false")
		}
	}
	return nil
}
