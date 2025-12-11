package main

//go:generate go tool qexp -outdir pkg github.com/goplus/spx/v2
//go:generate go tool qexp -outdir pkg github.com/goplus/spx/v2/pkg/gdspx/pkg/engine
//go:generate go tool qexp -outdir pkg github.com/goplus/spx/v2/pkg/spx

// All packages available in the ispx Wasm runtime.
import (
	// Embedded third-party packages.
	"github.com/goplus/spx/v2/cmd/igox/launcher"
)

func main() {
	launcher.Run()
}
