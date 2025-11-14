//go:build js && wasm

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"syscall/js"
	_ "unsafe"

	"github.com/goplus/builder/tools/ai"
	"github.com/goplus/builder/tools/ai/wasmtrans"
	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	_ "github.com/goplus/spx/v2"
	"github.com/goplus/spx/v2/cmd/igox/zipfs"
	goxfs "github.com/goplus/spx/v2/fs"
)

var aiDescription string

func setAIDescription(this js.Value, args []js.Value) any {
	if len(args) > 0 {
		aiDescription = args[0].String()
	}
	return nil
}

var aiInteractionAPIEndpoint string

func setAIInteractionAPIEndpoint(this js.Value, args []js.Value) any {
	if len(args) > 0 {
		aiInteractionAPIEndpoint = args[0].String()
	}
	return nil
}

var aiInteractionAPITokenProvider func() string

func setAIInteractionAPITokenProvider(this js.Value, args []js.Value) any {
	if len(args) > 0 && args[0].Type() == js.TypeFunction {
		tokenProviderFunc := args[0]
		aiInteractionAPITokenProvider = func() string {
			result := tokenProviderFunc.Invoke()
			if result.Type() != js.TypeObject || result.Get("then").IsUndefined() {
				return result.String()
			}

			resultChan := make(chan string, 1)
			then := js.FuncOf(func(this js.Value, args []js.Value) any {
				var result string
				if len(args) > 0 {
					result = args[0].String()
				}
				resultChan <- result
				return nil
			})
			defer then.Release()

			errChan := make(chan error, 1)
			catch := js.FuncOf(func(this js.Value, args []js.Value) any {
				errMsg := "promise rejected"
				if len(args) > 0 {
					errVal := args[0]
					if errVal.Type() == js.TypeObject && errVal.Get("message").Type() == js.TypeString {
						errMsg = fmt.Sprintf("promise rejected: %s", errVal.Get("message"))
					} else if errVal.Type() == js.TypeString {
						errMsg = fmt.Sprintf("promise rejected: %s", errVal)
					} else {
						errMsg = fmt.Sprintf("promise rejected: %v", errVal)
					}
				}
				errChan <- errors.New(errMsg)
				return nil
			})
			defer catch.Release()

			result.Call("then", then).Call("catch", catch)
			select {
			case result := <-resultChan:
				return result
			case err := <-errChan:
				log.Printf("failed to get token: %v", err)
				return ""
			}
		}
	}
	return nil
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
	fs     *zipfs.ZipFs
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
		js.Global().Call("gdspx_ext_request_exit", 1)
		return
	}
	ctx.SetPanic(logWithPanicInfo)

	// Register external functions
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

	// NOTE(everyone): Keep sync with the config in spx [gop.mod](https://github.com/goplus/spx/blob/main/gop.mod)
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v2", "math"},
		Import:   []*modfile.Import{{Name: "ai", Path: "github.com/goplus/builder/tools/ai"}},
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

	if err := ctx.RegisterPatch("github.com/goplus/builder/tools/ai", `
package ai

import . "github.com/goplus/builder/tools/ai"

func Gopt_Player_Gopx_OnCmd[T any](p *Player, handler func(cmd T) error) {
	var cmd T
	PlayerOnCmd_(p, cmd, handler)
}
`); err != nil {
		return nil
	}

	return &SpxRunner{
		ctx:   ctx,
		debug: false,
	}
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

	// Get files object
	inputArray := args[0]

	// Convert Uint8Array to Go byte slice
	length := inputArray.Get("length").Int()
	zipData := make([]byte, length)
	js.CopyBytesToGo(zipData, inputArray)

	// Initialize zip file system
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("Failed to read zip data: %w", err)
	}
	fs := zipfs.NewZipFsFromReader(zipReader)
	// Configure spx to load project files from zip-based file system.
	goxfs.RegisterSchema("", func(path string) (goxfs.Dir, error) {
		return fs.Chrooted(path), nil
	})

	// Use SpxRunner's shared context
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

	// Cache build result
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

	ai.SetDefaultTransport(wasmtrans.New(
		wasmtrans.WithEndpoint(aiInteractionAPIEndpoint),
		wasmtrans.WithTokenProvider(aiInteractionAPITokenProvider),
	))
	ai.SetDefaultKnowledgeBase(map[string]any{
		"AI-generated descriptive summary of the game world": aiDescription,
	})
	// Run interp in background goroutine (non-blocking)
	go func() {
		interp := r.entry.interp
		code, runErr := r.ctx.RunInterp(interp, "main.go", nil)

		if runErr != nil {
			fmt.Printf("Failed to run XGo source (code %d): %v\n", code, runErr)
			js.Global().Call("gdspx_ext_on_runtime_panic", runErr.Error())
			js.Global().Call("gdspx_ext_request_exit", 1)
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

func main() {

	// Register AI-related functions
	js.Global().Set("setAIDescription", js.FuncOf(setAIDescription))
	js.Global().Set("setAIInteractionAPIEndpoint", js.FuncOf(setAIInteractionAPIEndpoint))
	js.Global().Set("setAIInteractionAPITokenProvider", js.FuncOf(setAIInteractionAPITokenProvider))

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
	c := make(chan struct{})
	<-c
}

//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
