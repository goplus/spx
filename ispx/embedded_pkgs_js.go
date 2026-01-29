//go:build js && wasm

package ispx

// Embedded packages for the WebAssembly ispx runtime.
import (
	// Stdlib packages.
	_ "github.com/goplus/ixgo/pkg/syscall/js"
)
