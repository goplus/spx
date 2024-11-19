package ui

import (
	. "github.com/realdream-ai/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
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
	panel := engine.SyncCreateEngineUiNode[UiQuote]("")
	return panel
}

func (pself *UiQuote) OnStart() {
	pself.container = BindUI[UiNode](pself.GetId(), "C")
	pself.imageL = BindUI[UiNode](pself.GetId(), "C/ImageL")
	pself.imageR = BindUI[UiNode](pself.GetId(), "C/ImageR")
	pself.labelDes = BindUI[UiNode](pself.GetId(), "C/LabelDes")
	pself.labelMsg = BindUI[UiNode](pself.GetId(), "C/LabelMsg")
}

func (pself *UiQuote) SetText(x, y float64, width, height float64, msg, description string) {
	x, y = engine.SyncGetCameraLocalPosition(x, y)
	engine.SyncUiSetGlobalPosition(pself.container.GetId(), WorldToScreen(x-width, y+height))
	engine.SyncUiSetSize(pself.container.GetId(), engine.NewVec2(width*2, height*2))
	engine.SyncUiSetText(pself.labelMsg.GetId(), msg)
	engine.SyncUiSetText(pself.labelDes.GetId(), description)
}
