package ui

import (
	"math"

	. "github.com/realdream-ai/gdspx/pkg/engine"

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
	panel := engine.SyncCreateEngineUiNode[UiMeasure]("")
	return panel
}

func (pself *UiMeasure) OnStart() {
	pself.container = BindUI[UiNode](pself.GetId(), "C")
	pself.imageLine = BindUI[UiNode](pself.GetId(), "C/Line")
	pself.labelContainer = BindUI[UiNode](pself.GetId(), "LC")
	pself.labelValue = BindUI[UiNode](pself.GetId(), "LC/Label")
}

func (pself *UiMeasure) UpdateInfo(x, y float64, length, heading float64, name string, color Color) {
	extraLen := 4.0 //hack for engine picture size
	length += extraLen
	rad := engine.HeadingToRad(heading - 90)
	s, c := math.Sincos(float64(rad))
	halfX, halfY := (c * length / 2), (s * length / 2)
	pos := WorldToScreen(x, y)
	labelPos := pos
	pos.X -= float32(halfX)
	pos.Y -= float32(halfY)
	engine.SyncUiSetGlobalPosition(pself.container.GetId(), pos)
	engine.SyncUiSetColor(pself.container.GetId(), color)
	engine.SyncUiSetSize(pself.container.GetId(), engine.NewVec2(length+extraLen, 26))
	engine.SyncUiSetRotation(pself.container.GetId(), rad)

	engine.SyncUiSetGlobalPosition(pself.labelContainer.GetId(), labelPos)
	engine.SyncUiSetColor(pself.labelContainer.GetId(), color)
	engine.SyncUiSetText(pself.labelValue.GetId(), name)
}
