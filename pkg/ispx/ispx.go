package ispx

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	_ "github.com/goplus/spx/v2"
	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/pkg/ispx/internal/memfs"
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
	mu         sync.Mutex
	ixgoCtx    *ixgo.Context
	ixgoInterp *ixgo.Interp
	runDone    chan struct{}
)

// defaultPackagesToImport is the list of packages that are always imported by ispx.
var defaultPackagesToImport = []string{
	"fmt",
	"io",
	"io/fs",
	"math",
	"os",
	"reflect",
	"strconv",
	"strings",
	"sync",
	"sync/atomic",
	"time",
	"github.com/goplus/spx/v2",
	"github.com/qiniu/x/osx",
	"github.com/qiniu/x/stringslice",
	"github.com/qiniu/x/stringutil",
	"github.com/qiniu/x/xgo",
	"github.com/qiniu/x/xgo/ng",
}

// Init initializes the interpreter with the given ctx, which must not be
// modified after used. It can only be called once.
//
// If ctx is nil, a default [ixgo.Context] will be created.
//
// If ctx.Lookup is nil, a default lookup function will be set.
//
// packagesToImport specifies additional packages to import on top of the
// default set. See [defaultPackagesToImport] for the default list.
func Init(ctx *ixgo.Context, packagesToImport []string) error {
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

func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget, ok := GetWidget(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch - " + name)
	}
	return widget
}
`); err != nil {
		return fmt.Errorf("failed to register spx patch: %w", err)
	}

	ixgoCtx = ctx

	allPackages := append(defaultPackagesToImport, packagesToImport...)
	for _, pkg := range allPackages {
		ixgoCtx.Loader.Import(pkg)
	}

	return nil
}

// Build builds the spx code from the provided files into the interpreter.
func Build(files map[string][]byte) error {
	return BuildFS(memfs.New(files))
}

// BuildFS builds the spx code from the provided file system into the interpreter.
func BuildFS(fsys fs.FS) error {
	// Stop the game if running.
	if err := Shutdown(); err != nil {
		return err
	}

	mu.Lock()
	defer mu.Unlock()

	if ixgoCtx == nil {
		panic("ispx: not initialized")
	}

	// Release previous interpreter resources if any.
	if ixgoInterp != nil {
		ixgoInterp.UnsafeRelease()
		ixgoInterp = nil
	}

	spxfs.RegisterSchema("", func(path string) (spxfs.Dir, error) {
		return newSpxDir(fsys, path), nil
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
	if runDone != nil {
		mu.Unlock()
		panic("ispx: already running")
	}
	ctx, interp := ixgoCtx, ixgoInterp
	runDone = make(chan struct{})
	mu.Unlock()

	defer func() {
		mu.Lock()
		close(runDone)
		runDone = nil
		mu.Unlock()
	}()

	return ctx.RunInterp(interp, "main.go", nil)
}

// Shutdown requests the game to stop and waits for it to exit.
func Shutdown() error {
	mu.Lock()

	// If running, wait for it to stop.
	for runDone != nil {
		done := runDone
		mu.Unlock()

		engine.RequestExit(0)
		<-done

		mu.Lock()
	}

	mu.Unlock()
	return nil
}
