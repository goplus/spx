//go:build !js && !pure_engine

package facade

import (
	"github.com/goplus/spx/v2/internal/gdengine/binding/native"
	"github.com/goplus/spx/v2/pkg/spx/pkg/engine"
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
