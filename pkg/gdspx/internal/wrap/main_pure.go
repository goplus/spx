//go:build pure_engine

package wrap

import (
	. "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
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
	// Pure mode doesn't need FFI linking
	return true
}

func OnLinked() {
	// Pure mode doesn't need linking callbacks
}

func CreateMgrs() []IManager {
	return createMgrs()
}

func RegisterCallbacks(callbacks CallbackInfo) {
	// Pure mode doesn't need callback registration
	// Store callbacks for potential future use
	callbacks = callbacks
}
