package runtime

import (
	"errors"
	"fmt"
	_ "unsafe"

	"github.com/goplus/spx/v2/pkg/ispx/plugin"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	_ "github.com/goplus/spx/v2"
)

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

// newSpxRunnerWithConfig creates a new SpxRunner with custom configuration
func newSpxRunnerWithConfig(cfg *Config) *SpxRunner {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Use defaults for nil fields and update package-level defaults
	if cfg.Logger != nil {
		defaultLogger = cfg.Logger
	} else {
		cfg.Logger = defaultLogger
	}
	if cfg.Platform != nil {
		defaultPlatform = cfg.Platform
	} else {
		cfg.Platform = defaultPlatform
	}

	return newSpxRunnerInternal(cfg)
}

// newSpxRunnerInternal creates a new SpxRunner with the given configuration.
func newSpxRunnerInternal(cfg *Config) *SpxRunner {
	// Initialize ixgo context
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp)
	ctx.Lookup = func(root, path string) (dir string, found bool) {
		err := fmt.Errorf("failed to resolve package import %q", path)
		handleLookupError(err)
		return
	}
	ctx.SetPanic(logPanicInfo)

	registerExtFuns(ctx)

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

	// Register custom plugins from config
	for _, p := range cfg.Plugins {
		if err := p.Plugin.RegisterPatch(ctx); err != nil {
			return nil
		}
	}

	return &SpxRunner{
		ctx:   ctx,
		debug: cfg.Debug,
	}
}

// registerExtFuns registers external functions for fmt package.
func registerExtFuns(ctx *ixgo.Context) {
	ctx.RegisterExternal("fmt.Print", func(frame *ixgo.Frame, a ...any) (n int, err error) {
		msg := fmt.Sprint(a...)
		logWithCaller(msg, frame)
		return len(msg), nil
	})
	ctx.RegisterExternal("fmt.Printf", func(frame *ixgo.Frame, format string, a ...any) (n int, err error) {
		msg := fmt.Sprintf(format, a...)
		logWithCaller(msg, frame)
		return len(msg), nil
	})
	ctx.RegisterExternal("fmt.Println", func(frame *ixgo.Frame, a ...any) (n int, err error) {
		msg := fmt.Sprintln(a...)
		logWithCaller(msg, frame)
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

type Plugin struct {
	Name   string
	Plugin plugin.Plugin
}

//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
