package ui

import (
	"github.com/realdream-ai/mathf"
	. "github.com/realdream-ai/mathf"

	"github.com/goplus/spx/internal/engine"
)

type UiMeasure struct {
	UiNode
	container      *UiNode
	imageLine      *UiNode
	labelValue     *UiNode
	labelContainer *UiNode
}

func NewUiMeasure() *UiMeasure {
	panel := engine.NewUiNode[UiMeasure]()
	return panel
}

// !!Warning: this method was called in main thread
func (pself *UiMeasure) OnStart() {
	pself.container = SyncBindUI[UiNode](pself.GetId(), "C")
	pself.imageLine = SyncBindUI[UiNode](pself.GetId(), "C/Line")
	pself.labelContainer = SyncBindUI[UiNode](pself.GetId(), "LC")
	pself.labelValue = SyncBindUI[UiNode](pself.GetId(), "LC/Label")
}

func (pself *UiMeasure) UpdateInfo(wpos Vec2, length, heading float64, name string, color Color) {
	uiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	extraLen := 4.0 //hack for engine picture size
	length += extraLen

	rad := engine.DegToRad(heading - 90)
	sc := engine.Sincos(rad).Mulf(length / 2)
	pos := WorldToUI(wpos)
	labelPos := pos
	pos = pos.Sub(NewVec2(sc.Y, sc.X))

	uiMgr.SetGlobalPosition(pself.container.GetId(), pos)
	uiMgr.SetColor(pself.container.GetId(), color)
	uiMgr.SetSize(pself.container.GetId(), mathf.NewVec2(length+extraLen, 26))
	uiMgr.SetRotation(pself.container.GetId(), rad)

	uiMgr.SetGlobalPosition(pself.labelContainer.GetId(), labelPos)
	uiMgr.SetColor(pself.labelContainer.GetId(), color)
	uiMgr.SetText(pself.labelValue.GetId(), name)
}
