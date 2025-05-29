package ui

import (
	"github.com/realdream-ai/mathf"

	"github.com/goplus/spx/v2/internal/engine"
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
	uiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	pos = cameraMgr.GetLocalPosition(pos)
	uiMgr.SetGlobalPosition(pself.container.GetId(), WorldToUI(pos.Sub(mathf.NewVec2(size.X, -size.Y))))
	uiMgr.SetSize(pself.container.GetId(), size.Mulf(2))
	uiMgr.SetText(pself.labelMsg.GetId(), msg)
	uiMgr.SetText(pself.labelDes.GetId(), description)
}
