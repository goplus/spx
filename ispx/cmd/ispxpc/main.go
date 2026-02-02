//go:build !js || !wasm

package main

import (
	"fmt"
	"os"
	"path/filepath"
	_ "unsafe"

	"github.com/goplus/spx/ispx"
)

func main() {
	spxEngineRegisterFFI()

	// The project directory is one level up from the current working directory
	// because this shared library runs from either:
	//   - .temp/ directory (spx run mode)
	//   - project/ directory (spx run/editor mode)
	//
	// In both cases, the spx source files (.spx) are in the parent directory.
	projDir, err := filepath.Abs("..")
	if err != nil {
		panic("failed to get project directory: " + err.Error())
	}

	if err := ispx.Init(nil); err != nil {
		panic("failed to initialize: " + err.Error())
	}

	if err := ispx.BuildFS(os.DirFS(projDir)); err != nil {
		panic("failed to build: " + err.Error())
	}

	if exitCode, err := ispx.Run(); err != nil {
		panic(fmt.Sprintf("interpreter exited with code %d: %v", exitCode, err))
	}
}

// spxEngineRegisterFFI registers engine callbacks for FFI use.
//
//go:linkname spxEngineRegisterFFI github.com/goplus/spx/v2/pkg/gdspx/internal/engine.RegisterFFI
func spxEngineRegisterFFI()
