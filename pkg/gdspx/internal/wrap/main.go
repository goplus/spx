//go:build !js && !pure_engine

package wrap

import (
	"github.com/goplus/spx/v2/pkg/gdspx/internal/ffi"
	"github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

func LinkFFI() bool {
	return ffi.Link()
}

func OnLinked() {
	ffi.Linked()
}

func UnlinkFFI() {
	ffi.Unlink()
}

func RegisterCallbacks(callbacks engine.CallbackInfo) {
	ffi.BindCallback(callbacks)
}
