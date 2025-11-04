//go:build js && wasm

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"syscall/js"
	"time"
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

var dataChannel = make(chan []byte)

func loadData(this js.Value, args []js.Value) any {
	inputArray := args[0]

	// Convert Uint8Array to Go byte slice
	length := inputArray.Get("length").Int()
	goBytes := make([]byte, length)
	js.CopyBytesToGo(goBytes, inputArray)

	dataChannel <- goBytes
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

// SpxRunner encapsulates the build and run functionality for SPX code.
type SpxRunner struct {
	ctx   *ixgo.Context
	entry *interpCacheEntry
	debug bool
}

// interpCacheEntry stores the build result.
type interpCacheEntry struct {
	hash   string
	interp *ixgo.Interp
	fs     *zipfs.ZipFs
}

// NewSpxRunner creates a new SpxRunner instance (WASM interface).
func NewSpxRunner(this js.Value, args []js.Value) any {
	debug := false
	if len(args) > 0 {
		debug = args[0].Bool()
	}

	// Initialize ixgo context
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		logErrorAndExit(fmt.Errorf("Failed to resolve package import %q", path))
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
		return fmt.Errorf("Failed to register package patch for github.com/goplus/spx: %w", err)
	}

	if err := ctx.RegisterPatch("github.com/goplus/builder/tools/ai", `
package ai

import . "github.com/goplus/builder/tools/ai"

func Gopt_Player_Gopx_OnCmd[T any](p *Player, handler func(cmd T) error) {
	var cmd T
	PlayerOnCmd_(p, cmd, handler)
}
`); err != nil {
		return fmt.Errorf("Failed to register package patch for github.com/goplus/builder/tools/ai: %w", err)
	}

	runner := &SpxRunner{
		ctx:   ctx,
		debug: debug,
	}

	return js.ValueOf(map[string]any{
		"build": JSFuncOfWithError(runner.Build),
		"run":   JSFuncOfWithError(runner.Run),
	})
}

// JSUint8ArrayToBytes converts JS Uint8Array to Go []byte.
func JSUint8ArrayToBytes(value js.Value) []byte {
	if value.IsUndefined() || value.IsNull() {
		return nil
	}
	length := value.Get("length").Int()
	if length == 0 {
		return nil
	}
	data := make([]byte, length)
	js.CopyBytesToGo(data, value)
	return data
}

// computeFilesHash computes the hash of the files object.
func computeFilesHash(files js.Value) (string, error) {
	if files.Type() != js.TypeObject {
		return "", errors.New("files must be an object")
	}

	// Get all file paths and sort them (ensure stable hash)
	keys := js.Global().Get("Object").Call("keys", files)
	var paths []string
	for i := range keys.Length() {
		paths = append(paths, keys.Index(i).String())
	}
	sort.Strings(paths)

	// Compute hash
	h := sha256.New()
	for _, path := range paths {
		fileObj := files.Get(path)
		if !fileObj.InstanceOf(js.Global().Get("Object")) {
			continue
		}

		// Get file content
		content := JSUint8ArrayToBytes(fileObj.Get("content"))
		modTime := int64(fileObj.Get("modTime").Int())

		// Write path
		h.Write([]byte(path))
		h.Write([]byte{0}) // separator

		// Write content
		h.Write(content)
		h.Write([]byte{0}) // separator

		// Write modification time (optional, enhances hash uniqueness)
		h.Write([]byte(fmt.Sprintf("%d", modTime)))
		h.Write([]byte{0}) // separator
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// filesMapToZipData converts JS files object to zip data.
func filesMapToZipData(files js.Value) ([]byte, error) {
	if files.Type() != js.TypeObject {
		return nil, errors.New("files must be an object")
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Get all file paths
	keys := js.Global().Get("Object").Call("keys", files)
	for i := range keys.Length() {
		path := keys.Index(i).String()
		fileObj := files.Get(path)

		if !fileObj.InstanceOf(js.Global().Get("Object")) {
			continue
		}

		// Get file content (Uint8Array)
		content := JSUint8ArrayToBytes(fileObj.Get("content"))
		modTime := time.UnixMilli(int64(fileObj.Get("modTime").Int()))

		// Write to zip file
		header := &zip.FileHeader{
			Name:     path,
			Method:   zip.Deflate,
			Modified: modTime,
		}

		fw, err := zw.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry %s: %w", path, err)
		}

		if _, err := fw.Write(content); err != nil {
			return nil, fmt.Errorf("failed to write zip entry %s: %w", path, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// Build builds SPX code and creates an executable interp (WASM interface).
func (r *SpxRunner) Build(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("Build: missing files argument")
	}

	// Get files object
	files := args[0]
	if files.Type() != js.TypeObject {
		return errors.New("Build: files argument must be an object")
	}

	// Compute files hash
	filesHash, err := computeFilesHash(files)
	if err != nil {
		return fmt.Errorf("Build: failed to compute files hash: %w", err)
	}

	fmt.Printf("Files hash: %s\n", filesHash)

	// Check cache
	if r.entry != nil {
		if r.entry.hash == filesHash {
			fmt.Printf("Using cached build for hash: %s\n", filesHash)
			return nil
		} else {
			fmt.Printf("Cache miss, rebuilding for hash: %s\n", filesHash)
			r.entry.interp.UnsafeRelease()
		}
	}

	// Convert files to zip data
	zipData, err := filesMapToZipData(files)
	if err != nil {
		return fmt.Errorf("Build: %w", err)
	}

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

	ai.SetDefaultTransport(wasmtrans.New(
		wasmtrans.WithEndpoint(aiInteractionAPIEndpoint),
		wasmtrans.WithTokenProvider(aiInteractionAPITokenProvider),
	))
	ai.SetDefaultKnowledgeBase(map[string]any{
		"AI-generated descriptive summary of the game world": aiDescription,
	})

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
		hash:   filesHash,
		interp: interp,
		fs:     fs,
	}

	fmt.Printf("Build completed and cached with hash: %s\n", filesHash)

	return nil
}

// Run executes the built interp (WASM interface).
func (r *SpxRunner) Run(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("Run: missing files argument")
	}

	// Get files object
	files := args[0]
	if files.Type() != js.TypeObject {
		return errors.New("Run: files argument must be an object")
	}

	// Compute files hash
	filesHash, err := computeFilesHash(files)
	if err != nil {
		return fmt.Errorf("Run: failed to compute files hash: %w", err)
	}

	fmt.Printf("Run with files hash: %s\n", filesHash)

	// Look for cached interp
	if r.entry == nil || r.entry.hash != filesHash {
		// Cache miss, need to build first
		fmt.Printf("Cache miss, building for hash: %s\n", filesHash)
		if buildErr := r.Build(this, args); buildErr != nil {
			return buildErr
		}
	} else {
		fmt.Printf("Cache hit, using cached interp for hash: %s\n", filesHash)
	}

	// Run interp
	interp := r.entry.interp
	code, runErr := r.ctx.RunInterp(interp, "main.go", nil)
	if runErr != nil {
		return fmt.Errorf("Failed to run XGo source (code %d): %w", code, runErr)
	}

	return nil
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

func logErrorAndExit(err error) {
	fmt.Println(err)
	js.Global().Call("gdspx_ext_on_runtime_panic", err.Error())
	js.Global().Call("gdspx_ext_request_exit", 1)
	os.Exit(1)
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

	// Register legacy data loading function (for backward compatibility)
	js.Global().Set("goLoadData", js.FuncOf(loadData))

	// Register engine callback functions
	js.Global().Set("goWasmInit", js.FuncOf(goWasmInit))
	js.Global().Set("gdspx_on_engine_start", js.FuncOf(gdspxOnEngineStart))
	js.Global().Set("gdspx_on_engine_update", js.FuncOf(gdspxOnEngineUpdate))
	js.Global().Set("gdspx_on_engine_fixed_update", js.FuncOf(gdspxOnEngineFixedUpdate))
	js.Global().Set("gdspx_on_engine_destroy", js.FuncOf(gdspxOnEngineDestroy))
	js.Global().Set("gdspx_on_engine_pause", js.FuncOf(gdspxOnEnginePause))

	// Register SpxRunner WASM interface
	js.Global().Set("NewSpxRunner", JSFuncOfWithError(NewSpxRunner))

	// register FFI for worker mode
	spxEngineRegisterFFI()

	// Keep WASM running
	select {}
}

//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
