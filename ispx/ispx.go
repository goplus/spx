package ispx

import (
	"fmt"
	"sync"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	"github.com/goplus/spx/ispx/internal/memfs"
	_ "github.com/goplus/spx/v2"
	spxfs "github.com/goplus/spx/v2/fs"
)

func init() {
	// NOTE: Keep in sync with the config in spx's gop.mod.
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v2", "math"},
	})
}

var (
	mu          sync.Mutex
	ixgoCtx     *ixgo.Context
	ixgoInterp  *ixgo.Interp
	ixgoRunning bool
	ixgoRunID   int64
)

// Init initializes the interpreter with the given ctx, which must not be
// modified after used. It can only be called once.
//
// If ctx is nil, a default [ixgo.Context] will be created.
//
// If ctx.Lookup is nil, a default lookup function will be set.
func Init(ctx *ixgo.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if ixgoCtx != nil {
		panic("ispx: already initialized")
	}

	if ctx == nil {
		ctx = ixgo.NewContext(ixgo.SupportMultipleInterp)
	}
	if ctx.Lookup == nil {
		ctx.Lookup = defaultIXGoContextLookup
	}

	// Register patch for spx to support functions with generic type like [spx.XGot_Game_XGox_GetWidget].
	//
	// See https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
	if err := ctx.RegisterPatch("github.com/goplus/spx/v2", `
package spx

import . "github.com/goplus/spx/v2"

func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name string) *T {
	widget, ok := GetWidget_(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch")
	}
	return widget
}
`); err != nil {
		return fmt.Errorf("failed to register spx patch: %w", err)
	}

	ixgoCtx = ctx
	return nil
}

// Build builds the spx code from the provided files into the interpreter.
func Build(files map[string][]byte) error {
	mu.Lock()
	defer mu.Unlock()

	if ixgoCtx == nil {
		panic("ispx: not initialized")
	}
	if ixgoRunning {
		panic("ispx: cannot build while running")
	}

	// Release previous resources if any.
	unsafeRelease()

	fsys := memfs.New(files)
	spxfs.RegisterSchema("", func(path string) (spxfs.Dir, error) {
		return newSPXDir(fsys, path), nil
	})

	source, err := xgobuild.BuildFSDir(ixgoCtx, newXGoParserFS(fsys), ".")
	if err != nil {
		return fmt.Errorf("failed to build XGo source: %w", err)
	}

	pkg, err := ixgoCtx.LoadFile("main.go", source)
	if err != nil {
		return fmt.Errorf("failed to load XGo source: %w", err)
	}

	interp, err := ixgoCtx.NewInterp(pkg)
	if err != nil {
		return fmt.Errorf("failed to create interp: %w", err)
	}

	ixgoInterp = interp
	return nil
}

// Run runs the interpreter. It blocks until the interpreter exits. After it
// returns, the interpreter must be rebuilt before running again.
func Run() (exitCode int, err error) {
	mu.Lock()
	if ixgoInterp == nil {
		mu.Unlock()
		panic("ispx: not built")
	}
	if ixgoRunning {
		mu.Unlock()
		panic("ispx: already running")
	}
	ixgoRunning = true
	ixgoRunID++
	runID := ixgoRunID
	ctx, interp := ixgoCtx, ixgoInterp
	mu.Unlock()

	defer func() {
		mu.Lock()
		if ixgoRunID == runID {
			ixgoRunning = false
		}
		mu.Unlock()
	}()

	return ctx.RunInterp(interp, "main.go", nil)
}

// UnsafeRelease releases the interpreter's resources. It is unsafe to call
// while the interpreter is running.
func UnsafeRelease() {
	mu.Lock()
	defer mu.Unlock()
	unsafeRelease()
}

// unsafeRelease releases the interpreter's resources. It must be called with mu held.
func unsafeRelease() {
	ixgoRunning = false
	if ixgoInterp != nil {
		ixgoInterp.UnsafeRelease()
		ixgoInterp = nil
	}
}
