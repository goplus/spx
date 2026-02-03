package ui

import (
	"github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

type UiQuote struct {
	UiNode
	container *UiNode
	imageL    *UiNode
	imageR    *UiNode
	labelDes  *UiNode
	labelMsg  *UiNode
}

func NewUiQuote() *UiQuote {
	panel := engine.NewUiNode[UiQuote]()
	return panel
}

// !!Warning: this method was called in main thread
func (pself *UiQuote) OnStart() {
	pself.container = SyncBindUI[UiNode](pself.GetId(), "C")
	pself.imageL = SyncBindUI[UiNode](pself.GetId(), "C/ImageL")
	pself.imageR = SyncBindUI[UiNode](pself.GetId(), "C/ImageR")
	pself.labelDes = SyncBindUI[UiNode](pself.GetId(), "C/LabelDes")
	pself.labelMsg = SyncBindUI[UiNode](pself.GetId(), "C/LabelMsg")
}

func (pself *UiQuote) SetText(pos mathf.Vec2, size mathf.Vec2, msg, description string) {
	gdx.UiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	camPos := gdx.CameraMgr.GetCameraPosition()
	camPos.Y = -camPos.Y
	pos = pos.Sub(camPos)
	zoom := gdx.CameraMgr.GetCameraZoom()
	pos = pos.Mul(zoom.Divf(windowScale))
	targetPos := pos.Sub(mathf.NewVec2(size.X, -size.Y))
	gdx.UiMgr.SetGlobalPosition(pself.container.GetId(), WorldToUI(targetPos, true))
	gdx.UiMgr.SetSize(pself.container.GetId(), size.Mulf(2))
	gdx.UiMgr.SetText(pself.labelMsg.GetId(), msg)
	gdx.UiMgr.SetText(pself.labelDes.GetId(), description)
}
