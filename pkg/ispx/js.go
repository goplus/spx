//go:build js && wasm

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

package ispx

import (
	"errors"
	"fmt"
	"syscall/js"
	_ "unsafe"

	"github.com/goplus/spx/v4"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
)

// JavaScript built-in types.
var (
	jsTypeObject      = js.Global().Get("Object")
	jsTypeError       = js.Global().Get("Error")
	jsTypeArrayBuffer = js.Global().Get("ArrayBuffer")
	jsTypeUint8Array  = js.Global().Get("Uint8Array")
)

func init() {
	js.Global().Set("ispx_build", jsFuncOfWithError(ispxBuild))
	js.Global().Set("ispx_start", jsFuncOfWithError(ispxStart))
	js.Global().Set("ispx_stop", jsFuncOfWithError(ispxStop))
	js.Global().Set("ispx_input_recording_finish", jsFuncOfWithError(ispxInputRecordingFinish))
	js.Global().Set("ispx_input_session_status", jsFuncOfWithError(ispxInputSessionStatus))
}

// defaultIXGoContextLookup is the default [ixgo.Context.Lookup] when none is
// provided. It reports a package import error and requests a runtime reset.
func defaultIXGoContextLookup(root, path string) (dir string, found bool) {
	reportRuntimeError(fmt.Errorf("failed to resolve package import %q", path))
	return
}

// ispxBuild is the JavaScript interface for [Build].
func ispxBuild(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("missing files argument")
	}

	filesArg := args[0]
	if !isPlainJSObject(filesArg) {
		return errors.New("invalid files argument type")
	}

	files, err := convertJSFilesToMap(filesArg)
	if err != nil {
		return fmt.Errorf("failed to convert files: %w", err)
	}

	if err := Build(files); err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}
	return nil
}

// ispxStart starts the interpreter asynchronously. It calls [Run] in a
// goroutine and reports any errors to JavaScript via [reportRuntimeError].
func ispxStart(this js.Value, args []js.Value) any {
	preparation, err := prepareHostInputSession(args)
	if err != nil {
		return err
	}
	go func() {
		defer preparation.Cancel()
		defer reportRuntimePanic("interpreter")

		exitCode, err := Run()
		if err != nil {
			reportRuntimeError(fmt.Errorf("interpreter exited with code %d: %w", exitCode, err))
		}
	}()
	return nil
}

func prepareHostInputSession(args []js.Value) (spx.InputSessionPreparation, error) {
	if len(args) == 0 || isJSNil(args[0]) {
		return spx.InputSessionPreparation{}, nil
	}
	input := args[0]
	if !isPlainJSObject(input) {
		return spx.InputSessionPreparation{}, errors.New("input session must be an object")
	}
	mode := input.Get("mode")
	if mode.Type() != js.TypeString {
		return spx.InputSessionPreparation{}, errors.New("input session mode must be a string")
	}
	options, err := hostInputSessionOptions(input)
	if err != nil {
		return spx.InputSessionPreparation{}, err
	}
	switch mode.String() {
	case "record":
		fps := float64(defaultHostInputRecordingFPS)
		value := input.Get("fps")
		if !isJSNil(value) {
			if value.Type() != js.TypeNumber {
				return spx.InputSessionPreparation{}, errors.New("input recording FPS must be a number")
			}
			fps = value.Float()
		}
		return prepareHostInputRecording(fps, options)
	case "replay":
		value := input.Get("data")
		if isJSNil(value) {
			return spx.InputSessionPreparation{}, errors.New("missing input replay data")
		}
		data, err := copyJSReplayData(value)
		if err != nil {
			return spx.InputSessionPreparation{}, err
		}
		return prepareHostInputReplay(data, options)
	default:
		return spx.InputSessionPreparation{}, fmt.Errorf("unsupported input session mode %q", mode.String())
	}
}

func hostInputSessionOptions(input js.Value) (spx.InputSessionOptions, error) {
	value := input.Get("captureKey")
	if isJSNil(value) {
		return spx.InputSessionOptions{}, nil
	}
	if value.Type() != js.TypeString {
		return spx.InputSessionOptions{}, errors.New("input session captureKey must be a key name string")
	}
	key := spx.KeyFromString(value.String())
	if key == spx.KeyMax || key == spx.KeyAny {
		return spx.InputSessionOptions{}, fmt.Errorf("unsupported input session captureKey %q", value.String())
	}
	return spx.InputSessionOptions{CaptureKey: key}, nil
}

func ispxInputRecordingFinish(this js.Value, args []js.Value) any {
	data, err := finishHostInputRecording()
	if err != nil {
		return err
	}
	return copyBytesToJS(data)
}

func ispxInputSessionStatus(this js.Value, args []js.Value) any {
	return inputSessionStatusToJS(spx.GetInputSessionStatus())
}

func inputSessionStatusToJS(status spx.InputSessionStatus) js.Value {
	result := jsTypeObject.New()
	result.Set("mode", string(status.Mode))
	result.Set("phase", string(status.Phase))
	result.Set("completed", status.Completed)
	result.Set("exhausted", status.Exhausted)
	result.Set("currentTick", nil)
	if status.HasCurrentTick {
		result.Set("currentTick", float64(status.CurrentTick))
	}
	result.Set("nextFrame", float64(status.NextFrame))
	result.Set("frameCount", status.FrameCount)
	if status.Error != "" {
		result.Set("error", status.Error)
	}
	return result
}

func copyJSReplayData(value js.Value) ([]byte, error) {
	if value.Type() == js.TypeString {
		return []byte(value.String()), nil
	}
	if data, ok := copyJSBinaryData(value); ok {
		return data, nil
	}
	return nil, errors.New("expected string, Uint8Array, or ArrayBuffer")
}

func copyJSBinaryData(value js.Value) ([]byte, bool) {
	switch {
	case value.InstanceOf(jsTypeUint8Array):
	case value.InstanceOf(jsTypeArrayBuffer):
		value = jsTypeUint8Array.New(value)
	default:
		return nil, false
	}
	data := make([]byte, value.Get("byteLength").Int())
	js.CopyBytesToGo(data, value)
	return data, true
}

func copyBytesToJS(data []byte) js.Value {
	result := jsTypeUint8Array.New(len(data))
	js.CopyBytesToJS(result, data)
	return result
}

// ispxStop stops the interpreter asynchronously. It calls [Shutdown] in a
// goroutine to avoid blocking the JavaScript main thread.
func ispxStop(this js.Value, args []js.Value) any {
	go func() {
		defer reportRuntimePanic("shutdown")

		if err := Shutdown(); err != nil {
			reportRuntimeError(fmt.Errorf("failed to shutdown: %w", err))
		}
	}()
	return nil
}

// reportRuntimePanic must be deferred directly by the asynchronous entry point.
func reportRuntimePanic(operation string) {
	if r := recover(); r != nil {
		reportRuntimeError(fmt.Errorf("%s exited with panic: %v", operation, r))
	}
}

func isJSNil(value js.Value) bool {
	return value.IsUndefined() || value.IsNull()
}

// reportRuntimeError reports a runtime error to JavaScript and requests a reset.
func reportRuntimeError(err error) {
	spxlog.Error("%v", err)
	js.Global().Call("gdspx_ext_on_runtime_panic", err.Error())
	js.Global().Call("gdspx_ext_request_reset", 1)
}

// jsFuncOfWithError is like [js.FuncOf] but wraps Go errors as JavaScript Error objects.
func jsFuncOfWithError(fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		result := fn(this, args)
		if err, ok := result.(error); ok {
			return jsTypeError.New(err.Error())
		}
		return result
	})
}

// isPlainJSObject reports whether v is a plain JavaScript object, not an array,
// typed array, or other built-in object type.
func isPlainJSObject(v js.Value) bool {
	return jsTypeObject.Get("prototype").Get("toString").Call("call", v).String() == "[object Object]"
}

// convertJSFilesToMap converts a JavaScript object mapping file paths to
// Uint8Array or ArrayBuffer into a Go map.
func convertJSFilesToMap(input js.Value) (map[string][]byte, error) {
	keys := jsTypeObject.Call("keys", input)
	n := keys.Length()
	files := make(map[string][]byte, n)
	for i := range n {
		name := keys.Index(i).String()

		data, ok := copyJSBinaryData(input.Get(name))
		if !ok {
			return nil, fmt.Errorf("unsupported file value type for %q", name)
		}

		files[name] = data
	}
	return files, nil
}
