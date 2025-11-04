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

// SpxRunner 包装了 SPX 代码的构建和运行功能
type SpxRunner struct {
	ctx   *ixgo.Context
	entry *interpCacheEntry
	debug bool
}

// interpCacheEntry 存储构建结果
type interpCacheEntry struct {
	hash   string
	interp *ixgo.Interp
	fs     *zipfs.ZipFs
}

// NewSpxRunner 创建一个新的 SpxRunner 实例 (WASM 接口)
func NewSpxRunner(this js.Value, args []js.Value) any {
	debug := false
	if len(args) > 0 {
		debug = args[0].Bool()
	}

	// 初始化 ixgo context
	ctx := ixgo.NewContext(0)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		logErrorAndExit(fmt.Errorf("Failed to resolve package import %q", path))
		return
	}
	ctx.SetPanic(logWithPanicInfo)

	// 注册外部函数
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

	runner := &SpxRunner{
		ctx:   ctx,
		debug: debug,
	}

	return js.ValueOf(map[string]any{
		"build": JSFuncOfWithError(runner.Build),
		"run":   JSFuncOfWithError(runner.Run),
	})
}

// JSUint8ArrayToBytes 将 JS Uint8Array 转换为 Go []byte
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

// computeFilesHash 计算 files 对象的 hash
func computeFilesHash(files js.Value) (string, error) {
	if files.Type() != js.TypeObject {
		return "", errors.New("files must be an object")
	}

	// 获取所有文件路径并排序（保证 hash 稳定）
	keys := js.Global().Get("Object").Call("keys", files)
	var paths []string
	for i := range keys.Length() {
		paths = append(paths, keys.Index(i).String())
	}
	sort.Strings(paths)

	// 计算 hash
	h := sha256.New()
	for _, path := range paths {
		fileObj := files.Get(path)
		if !fileObj.InstanceOf(js.Global().Get("Object")) {
			continue
		}

		// 获取文件内容
		content := JSUint8ArrayToBytes(fileObj.Get("content"))
		modTime := int64(fileObj.Get("modTime").Int())

		// 写入路径
		h.Write([]byte(path))
		h.Write([]byte{0}) // 分隔符

		// 写入内容
		h.Write(content)
		h.Write([]byte{0}) // 分隔符

		// 写入修改时间（可选，增强 hash 唯一性）
		h.Write([]byte(fmt.Sprintf("%d", modTime)))
		h.Write([]byte{0}) // 分隔符
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// filesMapToZipData 将 JS files 对象转换为 zip 数据
func filesMapToZipData(files js.Value) ([]byte, error) {
	if files.Type() != js.TypeObject {
		return nil, errors.New("files must be an object")
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// 获取所有文件路径
	keys := js.Global().Get("Object").Call("keys", files)
	for i := range keys.Length() {
		path := keys.Index(i).String()
		fileObj := files.Get(path)

		if !fileObj.InstanceOf(js.Global().Get("Object")) {
			continue
		}

		// 获取文件内容（Uint8Array）
		content := JSUint8ArrayToBytes(fileObj.Get("content"))
		modTime := time.UnixMilli(int64(fileObj.Get("modTime").Int()))

		// 写入 zip 文件
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

// Build 构建 SPX 代码，创建可执行的 interp (WASM 接口)
func (r *SpxRunner) Build(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("Build: missing files argument")
	}

	// 获取 files 对象（Map<string, string>）
	files := args[0]
	if files.Type() != js.TypeObject {
		return errors.New("Build: files argument must be an object")
	}

	// 计算 files hash
	filesHash, err := computeFilesHash(files)
	if err != nil {
		return fmt.Errorf("Build: failed to compute files hash: %w", err)
	}

	fmt.Printf("Files hash: %s\n", filesHash)

	// 检查缓存
	if r.entry != nil {
		if r.entry.hash == filesHash {
			fmt.Printf("Using cached build for hash: %s\n", filesHash)
			return nil
		} else {
			fmt.Printf("Cache miss, rebuilding for hash: %s\n", filesHash)
			r.entry.interp.UnsafeRelease()
		}
	}

	// 将 files 转换为 zip 数据
	zipData, err := filesMapToZipData(files)
	if err != nil {
		return fmt.Errorf("Build: %w", err)
	}

	// 初始化 zip 文件系统
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("Failed to read zip data: %w", err)
	}
	fs := zipfs.NewZipFsFromReader(zipReader)
	// Configure spx to load project files from zip-based file system.
	goxfs.RegisterSchema("", func(path string) (goxfs.Dir, error) {
		return fs.Chrooted(path), nil
	})

	// 使用 SpxRunner 的共享 context
	ctx := r.ctx

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
	if err := xgobuild.RegisterPackagePatch(ctx, "github.com/goplus/spx/v2", `
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

	if err := xgobuild.RegisterPackagePatch(ctx, "github.com/goplus/builder/tools/ai", `
package ai

import . "github.com/goplus/builder/tools/ai"

func Gopt_Player_Gopx_OnCmd[T any](p *Player, handler func(cmd T) error) {
	var cmd T
	PlayerOnCmd_(p, cmd, handler)
}
`); err != nil {
		return fmt.Errorf("Failed to register package patch for github.com/goplus/builder/tools/ai: %w", err)
	}

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

	// 缓存构建结果
	r.entry = &interpCacheEntry{
		hash:   filesHash,
		interp: interp,
		fs:     fs,
	}

	fmt.Printf("Build completed and cached with hash: %s\n", filesHash)

	return nil
}

// Run 运行构建完的 interp (WASM 接口)
func (r *SpxRunner) Run(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("Run: missing files argument")
	}

	// 获取 files 对象
	files := args[0]
	if files.Type() != js.TypeObject {
		return errors.New("Run: files argument must be an object")
	}

	// 计算 files hash
	filesHash, err := computeFilesHash(files)
	if err != nil {
		return fmt.Errorf("Run: failed to compute files hash: %w", err)
	}

	fmt.Printf("Run with files hash: %s\n", filesHash)

	// 查找缓存的 interp
	if r.entry.hash != filesHash {
		// 缓存未命中，需要先构建
		fmt.Printf("Cache miss, building for hash: %s\n", filesHash)
		if buildErr := r.Build(this, args); buildErr != nil {
			return buildErr
		}
	} else {
		fmt.Printf("Cache hit, using cached interp for hash: %s\n", filesHash)
	}

	// 运行 interp
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

// JSFuncOfWithError 包装 js.Func，如果返回 error 则转换为 JS Error 对象
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
	// 注册 AI 相关函数
	js.Global().Set("setAIDescription", js.FuncOf(setAIDescription))
	js.Global().Set("setAIInteractionAPIEndpoint", js.FuncOf(setAIInteractionAPIEndpoint))
	js.Global().Set("setAIInteractionAPITokenProvider", js.FuncOf(setAIInteractionAPITokenProvider))

	// 注册旧的数据加载函数（保持向后兼容）
	js.Global().Set("goLoadData", js.FuncOf(loadData))

	// 注册引擎回调函数
	js.Global().Set("goWasmInit", js.FuncOf(goWasmInit))
	js.Global().Set("gdspx_on_engine_start", js.FuncOf(gdspxOnEngineStart))
	js.Global().Set("gdspx_on_engine_update", js.FuncOf(gdspxOnEngineUpdate))
	js.Global().Set("gdspx_on_engine_fixed_update", js.FuncOf(gdspxOnEngineFixedUpdate))
	js.Global().Set("gdspx_on_engine_destroy", js.FuncOf(gdspxOnEngineDestroy))
	js.Global().Set("gdspx_on_engine_pause", js.FuncOf(gdspxOnEnginePause))

	// 注册 SpxRunner WASM 接口
	js.Global().Set("NewSpxRunner", JSFuncOfWithError(NewSpxRunner))

	// register FFI for worker mode
	spxEngineRegisterFFI()

	// 保持 WASM 运行
	select {}
}

//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
