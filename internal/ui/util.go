package ui

import (
	. "github.com/realdream-ai/mathf"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/enginewrap"
	gdx "github.com/realdream-ai/gdspx/pkg/engine"
)

// copy these variable to any namespace you want
var (
	audioMgr    enginewrap.AudioMgrImpl
	cameraMgr   enginewrap.CameraMgrImpl
	inputMgr    enginewrap.InputMgrImpl
	physicMgr   enginewrap.PhysicMgrImpl
	platformMgr enginewrap.PlatformMgrImpl
	resMgr      enginewrap.ResMgrImpl
	sceneMgr    enginewrap.SceneMgrImpl
	spriteMgr   enginewrap.SpriteMgrImpl
	uiMgr       enginewrap.UiMgrImpl
)

type UiNode struct {
	gdx.UiNode
}

func SyncBindUI[T any](parentNode gdx.Object, path string) *T {
	return engine.SyncBindUI[T](parentNode, path)
}

// convert world space position to screen space
func WorldToUI(pos Vec2) Vec2 {
	pos.Y *= -1
	viewport := cameraMgr.GetViewportRect()
	return pos.Add(viewport.Size.Mulf(0.5)).Sub(viewport.Position)
}
