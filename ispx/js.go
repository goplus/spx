//go:build js && wasm

package ispx

import (
	"errors"
	"fmt"
	"syscall/js"
	_ "unsafe"
)

// JavaScript built-in types.
var (
	jsTypeObject      = js.Global().Get("Object")
	jsTypeError       = js.Global().Get("Error")
	jsTypeArrayBuffer = js.Global().Get("ArrayBuffer")
	jsTypeUint8Array  = js.Global().Get("Uint8Array")
)

func init() {
	gdspxEngineRegisterFFI()
	js.Global().Set("ispx_build", jsFuncOfWithError(ispxBuild))
	js.Global().Set("ispx_start", jsFuncOfWithError(ispxStart))
	js.Global().Set("ispx_stop", jsFuncOfWithError(ispxStop))

	// Deprecated: Use ispx_build and ispx_start instead.
	//
	// FIXME: Remove these aliases in future releases.
	js.Global().Set("ixgo_build", jsFuncOfWithError(ispxBuild))
	js.Global().Set("ixgo_run", jsFuncOfWithError(ispxStart))
}

// defaultIXGoContextLookup is the default [ixgo.Context.Lookup] when none is
// provided. It reports a package import error and requests a runtime reset.
func defaultIXGoContextLookup(root, path string) (dir string, found bool) {
	reportRuntimeError(fmt.Errorf("failed to resolve package import %q", path))
	return
}

// gdspxEngineRegisterFFI registers engine callback functions (gdspx_on_*) to
// the JavaScript global scope, enabling the Godot engine to invoke Go code for
// handling engine lifecycle, input, collision, and UI events.
//
//go:linkname gdspxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func gdspxEngineRegisterFFI()

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
	go func() {
		defer func() {
			if r := recover(); r != nil {
				reportRuntimeError(fmt.Errorf("interpreter exited with panic: %v", r))
			}
		}()

		exitCode, err := Run()
		if err != nil {
			reportRuntimeError(fmt.Errorf("interpreter exited with code %d: %w", exitCode, err))
			return
		}
	}()
	return nil
}

// ispxStop stops the interpreter asynchronously. It calls [Shutdown] in a
// goroutine to avoid blocking the JavaScript main thread.
func ispxStop(this js.Value, args []js.Value) any {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				reportRuntimeError(fmt.Errorf("shutdown exited with panic: %v", r))
			}
		}()

		if err := Shutdown(); err != nil {
			reportRuntimeError(fmt.Errorf("failed to shutdown: %w", err))
			return
		}
	}()
	return nil
}

// reportRuntimeError reports a runtime error to JavaScript and requests a reset.
func reportRuntimeError(err error) {
	fmt.Println(err)
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

		var jsData js.Value
		switch v := input.Get(name); {
		case v.InstanceOf(jsTypeUint8Array):
			jsData = v
		case v.InstanceOf(jsTypeArrayBuffer):
			jsData = jsTypeUint8Array.New(v)
		default:
			return nil, fmt.Errorf("unsupported file value type for %q", name)
		}
		length := jsData.Get("length").Int()
		data := make([]byte, length)
		js.CopyBytesToGo(data, jsData)

		files[name] = data
	}
	return files, nil
}
