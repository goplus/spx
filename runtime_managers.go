package spx

import "github.com/goplus/spx/v2/internal/enginewrap"

// runtimeManagers groups all engine-facing manager wrappers for a Game instance.
// This keeps runtime dependencies scoped to the game instead of package-level globals.
type runtimeManagers struct {
	audioMgr         enginewrap.AudioMgrImpl
	cameraMgr        enginewrap.CameraMgrImpl
	inputMgr         enginewrap.InputMgrImpl
	physicsMgr       enginewrap.PhysicsMgrImpl
	platformMgr      enginewrap.PlatformMgrImpl
	resMgr           enginewrap.ResMgrImpl
	sceneMgr         enginewrap.SceneMgrImpl
	spriteMgr        enginewrap.SpriteMgrImpl
	uiMgr            enginewrap.UiMgrImpl
	extMgr           enginewrap.ExtMgrImpl
	penMgr           enginewrap.PenMgrImpl
	debugMgr         enginewrap.DebugMgrImpl
	navigationMgr    enginewrap.NavigationMgrImpl
	tilemapMgr       enginewrap.TilemapMgrImpl
	tilemapparserMgr enginewrap.TilemapparserMgrImpl
}

func (p *Game) rt() *runtimeManagers {
	return &p.runtime
}

func (p *SpriteImpl) rt() *runtimeManagers {
	return p.g.rt()
}

func (c *componentBase) rt() *runtimeManagers {
	return c.sprite.rt()
}
