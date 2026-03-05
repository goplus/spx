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

// !!Warning: this method was called in main thread
func (pself *UiQuote) OnStart() {
	pself.container = engine.MainThreadBindUI[UiNode](pself.GetId(), "C")
	pself.imageL = engine.MainThreadBindUI[UiNode](pself.GetId(), "C/ImageL")
	pself.imageR = engine.MainThreadBindUI[UiNode](pself.GetId(), "C/ImageR")
	pself.labelDes = engine.MainThreadBindUI[UiNode](pself.GetId(), "C/LabelDes")
	pself.labelMsg = engine.MainThreadBindUI[UiNode](pself.GetId(), "C/LabelMsg")
}

func (pself *UiQuote) SetText(pos mathf.Vec2, size mathf.Vec2, msg, description string) {
	engine.MainThreadUiSetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	camPos := engine.MainThreadGetCameraPosition()
	pos = pos.Sub(camPos)
	zoom := engine.MainThreadGetCameraZoom()
	pos = pos.Mul(zoom.Divf(windowScale))
	targetPos := pos.Sub(mathf.NewVec2(size.X, -size.Y))
	engine.MainThreadUiSetGlobalPosition(pself.container.GetId(), WorldToUI(targetPos, true))
	engine.MainThreadUiSetSize(pself.container.GetId(), size.Mulf(2))
	engine.MainThreadUiSetText(pself.labelMsg.GetId(), msg)
	engine.MainThreadUiSetText(pself.labelDes.GetId(), description)
}
