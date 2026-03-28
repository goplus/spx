package ui

import (
	"github.com/goplus/spbase/mathf"
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
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

// !!Warning: this method is called from the engine callback context
func (pself *UiMeasure) OnStart() {
	pself.container = engine.BridgeBindUI[UiNode](pself.GetId(), "C")
	pself.imageLine = engine.BridgeBindUI[UiNode](pself.GetId(), "C/Line")
	pself.labelContainer = engine.BridgeBindUI[UiNode](pself.GetId(), "LC")
	pself.labelValue = engine.BridgeBindUI[UiNode](pself.GetId(), "LC/Label")
}

func (pself *UiMeasure) UpdateInfo(wpos Vec2, length, heading float64, name string, color Color) {
	mgr.UiMgr.SetScale(pself.GetId(), engine.UniformVec2(engine.WindowScale()))
	extraLen := 4.0 //hack for engine picture size
	length += extraLen

	rad := engine.DegToRad(heading - 90)
	sc := engine.Sincos(rad).Mulf(length / 2)
	pos := ViewToUI(wpos)
	labelPos := pos
	pos = pos.Sub(NewVec2(sc.Y, sc.X))

	mgr.UiMgr.SetGlobalPosition(pself.container.GetId(), pos)
	mgr.UiMgr.SetColor(pself.container.GetId(), color)
	mgr.UiMgr.SetSize(pself.container.GetId(), mathf.NewVec2(length+extraLen, 26))
	mgr.UiMgr.SetRotation(pself.container.GetId(), rad)

	mgr.UiMgr.SetGlobalPosition(pself.labelContainer.GetId(), labelPos)
	mgr.UiMgr.SetColor(pself.labelContainer.GetId(), color)
	mgr.UiMgr.SetText(pself.labelValue.GetId(), name)
}
