//go:build js

package wrap

import (
	"github.com/goplus/spx/v2/pkg/gdspx/internal/webffi"
	"github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
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
