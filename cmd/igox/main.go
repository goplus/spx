package main

//go:generate go tool qexp -outdir pkg github.com/goplus/spx/v2
//go:generate go tool qexp -outdir pkg github.com/goplus/spx/v2/pkg/gdspx/pkg/engine
//go:generate go tool qexp -outdir pkg github.com/goplus/spx/v2/pkg/spx

// All packages available in the ispx Wasm runtime.
import (
	// Embedded third-party packages.
	_ "github.com/goplus/spx/v2/cmd/igox/embedpkg"
	_ "github.com/goplus/spx/v2/cmd/igox/pkg/github.com/goplus/spx/v2"
	_ "github.com/goplus/spx/v2/cmd/igox/pkg/github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	"github.com/goplus/spx/v2/pkg/ispx/runtime"
)

func main() {
	runtime.Run()
}
