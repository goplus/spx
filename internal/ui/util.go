package ui

import (
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var (
	mgr *enginewrap.EngineManagers

	windowScale             float64
	baseScreenWidth         int
	baseScreenHeight        int
	clampUIPositionInScreen bool
)

type UiNode struct {
	gdx.UiNode
}

func Init(managers *enginewrap.EngineManagers) {
	mgr = managers
}

func SetWindowScale(scale float64) {
	windowScale = scale
}

func SetBaseScreenSize(width, height int) {
	baseScreenWidth = width
	baseScreenHeight = height
}

func ClampUIPositionInScreen(isClamp bool) {
	clampUIPositionInScreen = isClamp
}

func SyncBindUI[T any](parentNode gdx.Object, path string) *T {
	return engine.SyncBindUI[T](parentNode, path)
}

// WorldToUI converts world space position to screen space
// If useDirect is true, uses direct gdx calls (safe for main thread, e.g., onUpdate methods)
// If useDirect is false, uses cameraMgr (may deadlock if called from main thread)
func WorldToUI(pos Vec2, useDirect bool) Vec2 {
	pos = pos.Mulf(windowScale)
	pos = NewVec2(pos.X, -pos.Y)

	var camMgr gdx.ICameraMgr
	if useDirect {
		camMgr = gdx.CameraMgr
	} else {
		camMgr = &mgr.CameraMgr
	}

	viewport := camMgr.GetViewportRect()
	return pos.Add(viewport.Size.Mulf(0.5)).Sub(viewport.Position)
}
