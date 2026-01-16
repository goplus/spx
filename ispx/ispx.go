package ispx

import (
	"fmt"

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
	ixgoCtx     *ixgo.Context
	ixgoInterp  *ixgo.Interp
	ixgoFS      *memfs.MemFs
	ixgoRunning bool
)

// Init initializes the interpreter with the given ctx, which must not be
// modified after used. It can only be called once.
//
// If ctx is nil, a default [ixgo.Context] will be created.
//
// If ctx.Lookup is nil, a default lookup function will be set.
func Init(ctx *ixgo.Context) error {
	if ixgoCtx != nil {
		panic("ispx: already initialized")
	}

	if ctx == nil {
		ctx = ixgo.NewContext(ixgo.SupportMultipleInterp)
	}
	if ctx.Lookup == nil {
		ctx.Lookup = defaultIXGoContextLookup
	}

	// Register patch for spx to support functions with generic type like [spx.Gopt_Game_Gopx_GetWidget].
	//
	// See https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
	if err := ctx.RegisterPatch("github.com/goplus/spx/v2", `
package spx

import . "github.com/goplus/spx/v2"

func Gopt_Game_Gopx_GetWidget[T any](sg ShapeGetter, name string) *T {
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
	if ixgoCtx == nil {
		panic("ispx: not initialized")
	}

	// Release previous resources if any.
	if ixgoInterp != nil {
		UnsafeRelease()
	}

	fs := memfs.NewMemFs(files)
	spxfs.RegisterSchema("", func(path string) (spxfs.Dir, error) {
		return fs.Chroot(path)
	})

	source, err := xgobuild.BuildFSDir(ixgoCtx, fs, "")
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
	ixgoFS = fs
	return nil
}

// Run runs the interpreter. It blocks until the interpreter exits. After it
// returns, the interpreter must be rebuilt before running again.
func Run() (exitCode int, err error) {
	if ixgoInterp == nil {
		panic("ispx: not built")
	}
	if ixgoRunning {
		panic("ispx: already running")
	}

	ixgoRunning = true
	defer func() { ixgoRunning = false }()

	return ixgoCtx.RunInterp(ixgoInterp, "main.go", nil)
}

// UnsafeRelease releases the interpreter's resources. It is unsafe to call
// while the interpreter is running.
func UnsafeRelease() {
	if ixgoInterp != nil {
		ixgoInterp.UnsafeRelease()
		ixgoInterp = nil
	}
	if ixgoFS != nil {
		ixgoFS.Close()
		ixgoFS = nil
	}
}
