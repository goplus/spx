package ui

import (
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
)

var (
	mgr *enginewrap.EngineManagers

	windowScale             float64
	baseScreenWidth         int
	baseScreenHeight        int
	clampUIPositionInScreen bool
)

type UiNode struct {
	engine.UiNode
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

// WorldToUI converts world space position to screen space.
func WorldToUI(pos Vec2) Vec2 {
	pos = pos.Mulf(windowScale)
	pos = NewVec2(pos.X, -pos.Y)
	viewport := mgr.CameraMgr.GetViewportRect()
	return pos.Add(viewport.Size.Mulf(0.5)).Sub(viewport.Position)
}
