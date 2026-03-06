package impl

import (
	. "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

var (
	mgrs []IManager
)

func addManager[T IManager](mgr T) T {
	mgrs = append(mgrs, mgr)
	return mgr
}

func CreateMgrs() []IManager {
	// Make repeated calls deterministic.
	mgrs = mgrs[:0]
	return createMgrs()
}

type baseMgr struct{}

func (pself *baseMgr) OnStart() {}

func (pself *baseMgr) OnUpdate(delta float64) {}

func (pself *baseMgr) OnFixedUpdate(delta float64) {}

func (pself *baseMgr) OnDestroy() {}

func (pself *baseMgr) OnPause(isPaused bool) {}
