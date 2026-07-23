//go:build tools

package main

// Keep the Builder AI version available to the SPX project generator without
// linking its SPX v2 runtime into the v3 ispx WASM binary. The local AI test
// harness is preserved on the wip/v3-builder-ai branch until Builder AI moves
// to SPX v3.
import _ "github.com/goplus/builder/tools/ai"
