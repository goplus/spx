//go:build js && wasm

package launcher

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall/js"
	_ "unsafe"

	"github.com/goplus/spx/v2/cmd/igox/plugin"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	_ "github.com/goplus/spx/v2"
	"github.com/goplus/spx/v2/cmd/igox/memfs"
	goxfs "github.com/goplus/spx/v2/fs"
)

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

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

var defaultRunner *SpxRunner = NewSpxRunner()

// SpxRunner encapsulates the build and run functionality for SPX code.
type SpxRunner struct {
	ctx   *ixgo.Context
	entry *interpCacheEntry
	debug bool
}

// interpCacheEntry stores the build result.
type interpCacheEntry struct {
	interp *ixgo.Interp
	fs     *memfs.MemFs
}

// SpxRunner encapsulates the build and run functionality for SPX code with hash-based caching.
// It maintains a single cached build result and automatically rebuilds when file content changes.
//
// Caching: Build() computes SHA256 hash of input files and caches the interpreter.
// Only one build is cached; new builds invalidate previous cache.
func NewSpxRunner() *SpxRunner {
	// Initialize ixgo context
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		err := fmt.Errorf("Failed to resolve package import %q", path)
		js.Global().Call("gdspx_ext_on_runtime_panic", err.Error())
		js.Global().Call("gdspx_ext_request_reset", 1)
		return
	}
	ctx.SetPanic(logWithPanicInfo)

	RegisterExtFuns(ctx)

	// NOTE(everyone): Keep sync with the config in spx [gop.mod](https://github.com/goplus/spx/blob/main/gop.mod)
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v2", "math"},
	})

	// Register patch for spx to support functions with generic type like `Gopt_Game_Gopx_GetWidget`.
	// See details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805
	if err := ctx.RegisterPatch("github.com/goplus/spx/v2", `
package spx

import . "github.com/goplus/spx/v2"

func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name string) *T {
	widget := GetWidget_(sg, name)
	if result, ok := widget.(any).(*T); ok {
		return result
	} else {
		panic("GetWidget: type mismatch")
	}
}
`); err != nil {
		return nil
	}

	if err := plugin.GetPluginManager().RegisterPatch(ctx); err != nil {
		return nil
	}

	return &SpxRunner{
		ctx:   ctx,
		debug: false,
	}
}

func RegisterExtFuns(ctx *ixgo.Context) {
	ctx.RegisterExternal("fmt.Print", func(frame *ixgo.Frame, a ...any) (n int, err error) {
		msg := fmt.Sprint(a...)
		logWithCallerInfo(msg, frame)
		return len(msg), nil
	})
	ctx.RegisterExternal("fmt.Printf", func(frame *ixgo.Frame, format string, a ...any) (n int, err error) {
		msg := fmt.Sprintf(format, a...)
		logWithCallerInfo(msg, frame)
		return len(msg), nil
	})
	ctx.RegisterExternal("fmt.Println", func(frame *ixgo.Frame, a ...any) (n int, err error) {
		msg := fmt.Sprintln(a...)
		logWithCallerInfo(msg, frame)
		return len(msg), nil
	})
}

// Build builds the SPX code from the provided files object.
// It uses the provided hash to cache the build result.
//
// Parameters:
//
//	args[0]: Uint8Array - zip data of the project files
//
// Returns: nil on success, error on build failure.
func (r *SpxRunner) Build(this js.Value, args []js.Value) any {
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

	filesMap, err := ConvertJSFilesToMap(input)
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
		fs:     fs,
	}
	return nil
}

// Run executes the cached interpreter, automatically building if necessary.
//
// Behavior:
//  1. Executes the interpreter
//
// Returns: nil on success, error on build or execution failure.
//
// Note: This method is idempotent - won't rebuild unnecessarily.
func (r *SpxRunner) Run(this js.Value, args []js.Value) any {
	if r.entry == nil || r.entry.interp == nil {
		return errors.New("Run: Build() must be called first")
	}

	plugin.GetPluginManager().Init()
	// Run interp in background goroutine (non-blocking)
	go func() {
		handleErr := func(msg string) {
			fmt.Println(msg)
			js.Global().Call("gdspx_ext_on_runtime_panic", msg)
			js.Global().Call("gdspx_ext_request_reset", 1)
		}

		defer func() {
			if rec := recover(); rec != nil {
				err := fmt.Errorf("panic in RunInterp: %v", rec)
				handleErr(err.Error())
			}
		}()

		interp := r.entry.interp
		code, runErr := r.ctx.RunInterp(interp, "main.go", nil)

		if runErr != nil {
			msg := fmt.Sprintf("Failed to run XGo source (code %d): %v", code, runErr)
			handleErr(msg)
			return
		}
	}()

	return nil
}

// Release releases resources held by the SpxRunner.
func (r *SpxRunner) Release() {
	// Clear context
	r.ctx.RunContext = nil
	if r.entry != nil && r.entry.interp != nil {
		r.entry.interp.UnsafeRelease()
		r.entry.fs.Close()
		r.entry = nil
	}
}

// ConvertJSFilesToMap converts a JavaScript object containing file data into a Go map.
// The input object should map file paths (strings) to file contents (Uint8Array or ArrayBuffer).
//
// Returns an error if any value is not a Uint8Array or ArrayBuffer.
func ConvertJSFilesToMap(input js.Value) (map[string][]byte, error) {
	keys := js.Global().Get("Object").Call("keys", input)
	n := keys.Length()
	filesMap := make(map[string][]byte, n)
	uint8ArrayType := js.Global().Get("Uint8Array")
	arrayBufferType := js.Global().Get("ArrayBuffer")
	for i := 0; i < n; i++ {
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

func logWithCallerInfo(msg string, frame *ixgo.Frame) {
	if frs := frame.CallerFrames(); len(frs) > 0 {
		fr := frs[0]
		logger.Info(
			msg,
			"function", fr.Function,
			"file", fr.File,
			"line", fr.Line,
		)
	}
}

func logWithPanicInfo(info *ixgo.PanicInfo) {
	position := info.Position()
	logger.Error(
		"panic",
		"error", info.Error,
		"function", info.String(),
		"file", position.Filename,
		"line", position.Line,
		"column", position.Column,
	)
}

// JSFuncOfWithError wraps js.Func and converts error returns to JS Error objects.
func JSFuncOfWithError(fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		result := fn(this, args)
		if err, ok := result.(error); ok {
			return js.Global().Get("Error").New(err.Error())
		}
		return result
	})
}

type Plugin struct {
	Name   string
	Plugin plugin.Plugin
}

type TestPlugin struct {
}

func (p *TestPlugin) RegisterJS() {
}
func (p *TestPlugin) RegisterPatch(ctx *ixgo.Context) error {
	return nil
}
func (p *TestPlugin) Init() {
}

func Run(plugins ...Plugin) {
	for _, info := range plugins {
		plugin.GetPluginManager().RegisterPlugin(info.Name, info.Plugin)
	}
	plugin.GetPluginManager().RegisterJS()
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
	js.Global().Set("ixgo_build", JSFuncOfWithError(defaultRunner.Build))
	js.Global().Set("ixgo_run", JSFuncOfWithError(defaultRunner.Run))

	// Keep WASM running select {} will block the main goroutine forever
	exitChan := make(chan struct{})
	<-exitChan
}

//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
