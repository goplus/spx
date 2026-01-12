//go:build js && wasm

package runtime

import (
	"errors"
	"fmt"
	"syscall/js"

	goxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/pkg/ispx/memfs"
	"github.com/goplus/spx/v2/pkg/ispx/plugin"

	"github.com/goplus/ixgo"
	_ "github.com/goplus/ixgo/pkg/syscall/js"
	"github.com/goplus/ixgo/xgobuild"
)

// wasmPlatform implements Platform for WASM
type wasmPlatform struct{}

func init() {
	defaultPlatform = &wasmPlatform{}
}

func (p *wasmPlatform) HandleLookupError(err error) {
	js.Global().Call("gdspx_ext_on_runtime_panic", err.Error())
	js.Global().Call("gdspx_ext_request_reset", 1)
}

// Launch starts the WASM runtime
func Launch(cfg *Config) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	for _, info := range cfg.Plugins {
		plugin.GetPluginManager().RegisterPlugin(info.Name, info.Plugin)
	}
	plugin.GetPluginManager().RegisterJS()

	// Create runner with config
	runner := newSpxRunnerWithConfig(cfg)

	// Register engine callback functions
	js.Global().Set("goWasmInit", js.FuncOf(goWasmInit))
	js.Global().Set("gdspx_on_engine_start", js.FuncOf(gdspxOnEngineStart))
	js.Global().Set("gdspx_on_engine_update", js.FuncOf(gdspxOnEngineUpdate))
	js.Global().Set("gdspx_on_engine_fixed_update", js.FuncOf(gdspxOnEngineFixedUpdate))
	js.Global().Set("gdspx_on_engine_destroy", js.FuncOf(gdspxOnEngineDestroy))
	js.Global().Set("gdspx_on_engine_pause", js.FuncOf(gdspxOnEnginePause))

	// register FFI for worker mode
	spxEngineRegisterFFI()

	// Register SpxRunner WASM interface
	js.Global().Set("ixgo_build", jsFuncOfWithError(runner.build))
	js.Global().Set("ixgo_run", jsFuncOfWithError(runner.run))

	// Keep WASM running
	exitChan := make(chan struct{})
	<-exitChan
}

func goWasmInit(this js.Value, args []js.Value) any {
	return js.ValueOf(nil)
}

func gdspxOnEngineStart(this js.Value, args []js.Value) any {
	return nil
}

func gdspxOnEngineUpdate(this js.Value, args []js.Value) any {
	return nil
}

func gdspxOnEngineFixedUpdate(this js.Value, args []js.Value) any {
	return nil
}

func gdspxOnEngineDestroy(this js.Value, args []js.Value) any {
	return nil
}

func gdspxOnEnginePause(this js.Value, args []js.Value) any {
	return nil
}

// handleLookupError handles package lookup errors for WASM platform
func handleLookupError(err error) {
	defaultPlatform.HandleLookupError(err)
}

// build builds the SPX code from the provided files object
func (r *SpxRunner) build(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("Build: missing files argument")
	}

	if r.entry != nil && r.entry.interp != nil {
		r.Release()
	}

	input := args[0]
	if input.Type() != js.TypeObject || !input.Get("length").IsUndefined() {
		return errors.New("Build: only support object map[path]Uint8Array")
	}

	filesMap, err := convertJSFilesToMap(input)
	if err != nil {
		return fmt.Errorf("Build: failed to get files: %w", err)
	}

	fs := memfs.NewMemFs(filesMap)
	goxfs.RegisterSchema("", func(path string) (goxfs.Dir, error) {
		return fs.Chroot(path)
	})

	ctx := r.ctx
	source, err := xgobuild.BuildFSDir(ctx, fs, "")
	if err != nil {
		return fmt.Errorf("Failed to build XGo source: %w", err)
	}

	pkg, err := ctx.LoadFile("main.go", source)
	if err != nil {
		return fmt.Errorf("Failed to load XGo source: %w", err)
	}

	interp, err := ctx.NewInterp(pkg)
	if err != nil {
		return fmt.Errorf("Failed to create interp: %w", err)
	}

	if r.debug {
		capacity, allocate, available := ixgo.IcallStat()
		fmt.Printf("Icall Capacity: %d, Allocate: %d, Available: %d\n", capacity, allocate, available)
	}

	r.entry = &interpCacheEntry{
		interp: interp,
		closer: func() error { return fs.Close() },
	}
	return nil
}

// run executes the cached interpreter
func (r *SpxRunner) run(this js.Value, args []js.Value) any {
	return r.RunInterp(func(msg string) {
		fmt.Println(msg)
		js.Global().Call("gdspx_ext_on_runtime_panic", msg)
		js.Global().Call("gdspx_ext_request_reset", 1)
	})
}

// convertJSFilesToMap converts JavaScript files object to Go map
func convertJSFilesToMap(input js.Value) (map[string][]byte, error) {
	keys := js.Global().Get("Object").Call("keys", input)
	n := keys.Length()
	filesMap := make(map[string][]byte, n)
	uint8ArrayType := js.Global().Get("Uint8Array")
	arrayBufferType := js.Global().Get("ArrayBuffer")

	for i := range n {
		name := keys.Index(i).String()
		val := input.Get(name)
		var u8 js.Value

		if val.InstanceOf(uint8ArrayType) {
			u8 = val
		} else if val.InstanceOf(arrayBufferType) {
			u8 = uint8ArrayType.New(val)
		} else {
			return nil, fmt.Errorf("Build: unsupported file value type for %s", name)
		}

		length := u8.Get("length").Int()
		data := make([]byte, length)
		js.CopyBytesToGo(data, u8)
		filesMap[name] = data
	}
	return filesMap, nil
}

// jsFuncOfWithError wraps js.Func and converts error returns to JS Error objects
func jsFuncOfWithError(fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		result := fn(this, args)
		if err, ok := result.(error); ok {
			return js.Global().Get("Error").New(err.Error())
		}
		return result
	})
}
