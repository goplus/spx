package ui

import (
	"github.com/goplus/spbase/mathf"

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

// !!Warning: this method is called from the engine callback context
func (pself *UiQuote) OnStart() {
	pself.container = engine.BridgeBindUI[UiNode](pself.GetId(), "C")
	pself.imageL = engine.BridgeBindUI[UiNode](pself.GetId(), "C/ImageL")
	pself.imageR = engine.BridgeBindUI[UiNode](pself.GetId(), "C/ImageR")
	pself.labelDes = engine.BridgeBindUI[UiNode](pself.GetId(), "C/LabelDes")
	pself.labelMsg = engine.BridgeBindUI[UiNode](pself.GetId(), "C/LabelMsg")
}

func (pself *UiQuote) SetText(pos mathf.Vec2, size mathf.Vec2, msg, description string) {
	mgr.UiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	pos = engine.BridgeWorldToView(pos)
	targetPos := pos.Sub(mathf.NewVec2(size.X, -size.Y))
	mgr.UiMgr.SetGlobalPosition(pself.container.GetId(), ViewToUI(targetPos))
	mgr.UiMgr.SetSize(pself.container.GetId(), size.Mulf(2))
	mgr.UiMgr.SetText(pself.labelMsg.GetId(), msg)
	mgr.UiMgr.SetText(pself.labelDes.GetId(), description)
}
