package launcher

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	_ "unsafe"

	"github.com/goplus/spx/v2/cmd/igox/plugin"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	_ "github.com/goplus/spx/v2"
	_ "github.com/goplus/spx/v2/cmd/igox/embedpkg"
	_ "github.com/goplus/spx/v2/cmd/igox/pkg/github.com/goplus/spx/v2"
	_ "github.com/goplus/spx/v2/cmd/igox/pkg/github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

var defaultRunner *SpxRunner = NewSpxRunner()

// interpCacheEntry stores the build result.
type interpCacheEntry struct {
	interp *ixgo.Interp
	closer func() error
}

// SpxRunner encapsulates the build and run functionality for SPX code.
type SpxRunner struct {
	ctx   *ixgo.Context
	entry *interpCacheEntry
	debug bool
}

// NewSpxRunner creates a new SpxRunner instance.
func NewSpxRunner() *SpxRunner {
	// Initialize ixgo context
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		err := fmt.Errorf("failed to resolve package import %q", path)
		handleLookupError(err)
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

// RegisterExtFuns registers external functions for fmt package.
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

// Run executes the cached interpreter, automatically building if necessary.
//
// Behavior:
//  1. Executes the interpreter
//
// Returns: nil on success, error on build or execution failure.
//
// Note: This method is idempotent - won't rebuild unnecessarily.
func (r *SpxRunner) RunInterp(handleErr func(msg string)) any {
	if r.entry == nil || r.entry.interp == nil {
		return errors.New("Run: Build() must be called first")
	}

	plugin.GetPluginManager().Init()
	// Run interp in background goroutine (non-blocking)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				err := fmt.Errorf("panic in RunInterp: %v", rec)
				handleErr(err.Error())
			}
		}()

		interp := r.entry.interp
		code, runErr := r.ctx.RunInterp(interp, "main.go", nil)

		if runErr != nil {
			msg := fmt.Sprintf("failed to run XGo source (code %d): %v", code, runErr)
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
	if r.entry != nil {
		if r.entry.interp != nil {
			r.entry.interp.UnsafeRelease()
		}
		if r.entry.closer != nil {
			r.entry.closer()
		}
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

type Plugin struct {
	Name   string
	Plugin plugin.Plugin
}

//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
