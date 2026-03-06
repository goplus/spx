//go:build js

package facade

import (
	"github.com/goplus/spx/v2/internal/gdengine/binding/web"
	"github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

func LinkFFI() bool {
	return webffi.Link()
}

func OnLinked() {
	webffi.Linked()
}

func UnlinkFFI() {
	webffi.Unlink()
}

func RegisterCallbacks(callbacks engine.CallbackInfo) {
	webffi.BindCallback(callbacks)
}
