package spx

import "github.com/goplus/spx/v2/internal/enginewrap"

// engineManagers groups all engine-facing manager wrappers for a Game instance.
// This keeps runtime dependencies scoped to the game instead of package-level globals.
type engineManagers struct {
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

func (p *Game) engine() *engineManagers {
	return &p.engineMgr
}

func (p *SpriteImpl) engine() *engineManagers {
	return p.g.engine()
}

func (c *componentBase) engine() *engineManagers {
	return c.sprite.engine()
}
