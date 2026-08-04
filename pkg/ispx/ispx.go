/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ispx

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/transform"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/mod/modfile"
	_ "github.com/goplus/reflectx/icall/icall2048"
	_ "github.com/goplus/spx/v3"
	spxfs "github.com/goplus/spx/v3/fs"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/pkg/ispx/internal/memfs"
)

func init() {
	// NOTE: Keep in sync with the config in spx's gox.mod.
	xgobuild.RegisterProject(&modfile.Project{
		Ext:      ".spx",
		Class:    "Game",
		Works:    []*modfile.Class{{Ext: ".spx", Class: "SpriteImpl", Embedded: true}},
		PkgPaths: []string{"github.com/goplus/spx/v3", "math"},
	})
}

var (
	mu          sync.Mutex
	ixgoCtx     *ixgo.Context
	ixgoInterp  *ixgo.Interp
	runDone     chan struct{}
	optimizeSSA bool
)

// initOptions controls optional interpreter initialization behavior.
type initOptions struct {
	optimizeSSA bool
}

// InitOption configures interpreter initialization.
type InitOption func(*initOptions)

// WithSSAOptimization enables ixgo's default SSA transformation pipeline before
// creating the interpreter.
func WithSSAOptimization() InitOption {
	return func(options *initOptions) {
		options.optimizeSSA = true
	}
}

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
	"github.com/goplus/spx/v3",
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
func Init(ctx *ixgo.Context, options ...InitOption) error {
	mu.Lock()
	defer mu.Unlock()

	if ixgoCtx != nil {
		panic("ispx: already initialized")
	}

	var opts initOptions
	for _, option := range options {
		option(&opts)
	}

	if ctx == nil {
		ctx = ixgo.NewContext(ixgo.SupportMultipleInterp | xgobuild.StaticLoad)
	}
	if ctx.Lookup == nil {
		ctx.Lookup = defaultIXGoContextLookup
	}
	ctx.SetPanic(logRuntimePanic)

	for _, pkg := range defaultPackagesToImport {
		ctx.Loader.Import(pkg)
	}

	// Register patch for spx to support functions with generic type like [spx.XGot_Game_XGox_GetWidget].
	//
	// See https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
	if err := ctx.RegisterPatch("github.com/goplus/spx/v3", `
package spx

import . "github.com/goplus/spx/v3"

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
	optimizeSSA = opts.optimizeSSA
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
	if optimizeSSA {
		if err := transform.Transform(pkg); err != nil {
			return fmt.Errorf("failed to transform SSA: %w", err)
		}
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
