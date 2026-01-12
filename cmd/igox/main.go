package main

//go:generate go tool qexp -outdir ../../pkg/ispx/pkg github.com/goplus/spx/v2
//go:generate go tool qexp -outdir ../../pkg/ispx/pkg github.com/goplus/spx/v2/pkg/gdspx/pkg/engine
//go:generate go tool qexp -outdir ../../pkg/ispx/pkg github.com/goplus/spx/v2/pkg/spx

// All packages available in the ispx Wasm runtime.
import (
	"github.com/goplus/spx/v2/pkg/ispx"
)

func main() {
	ispx.Launch(nil) // Use default config
}
