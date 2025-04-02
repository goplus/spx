//go:build js

package wrap

import (
	"github.com/goplus/spx/pkg/gdspx/internal/webffi"
	. "github.com/goplus/spx/pkg/gdspx/pkg/engine"
)

type EngineStartFunc func()
type EngineUpdateFunc func(delta float64)
type EngineDestroyFunc func()

var (
	mgrs      []IManager
	callbacks CallbackInfo
)

func addManager[T IManager](mgr T) T {
	mgrs = append(mgrs, mgr)
	return mgr
}
func LinkFFI() bool {
	return webffi.Link()
}

func OnLinked() {
	webffi.Linked()
}

func CreateMgrs() []IManager {
	return createMgrs()
}

func RegisterCallbacks(callbacks CallbackInfo) {
	webffi.BindCallback(callbacks)
}
