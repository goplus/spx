package engine

import (
	"sync/atomic"

	"github.com/goplus/spx/v2/internal/enginewrap"
)

var (
	defaultManagers enginewrap.EngineManagers
	activeManagers  atomic.Pointer[enginewrap.EngineManagers]
)

func init() {
	activeManagers.Store(&defaultManagers)
}

// SetManagers injects the runtime-scoped manager set used by the Go bridge layer.
// Passing nil resets to the default zero-value manager set.
func SetManagers(managers *enginewrap.EngineManagers) {
	if managers == nil {
		activeManagers.Store(&defaultManagers)
		return
	}
	activeManagers.Store(managers)
}

// Managers returns the active runtime-scoped manager set for the current game.
func Managers() *enginewrap.EngineManagers {
	return activeManagers.Load()
}
