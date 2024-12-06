package ui

import (
	. "github.com/realdream-ai/gdspx/pkg/engine"
	"github.com/realdream-ai/mathf"
	. "github.com/realdream-ai/mathf"

	"github.com/goplus/spx/internal/engine"
)

type UiMonitor struct {
	UiNode
	bgAll          *UiNode
	valueOnly      *UiNode
	labelName      *UiNode
	labelBg        *UiNode
	labelValue     *UiNode
	labelValueOnly *UiNode
}
type UpdateFunc func(float64)

func NewUiMonitor() *UiMonitor {
	panel := engine.SyncCreateEngineUiNode[UiMonitor]("")
	return panel
}
func (pself *UiMonitor) OnStart() {
	pself.bgAll = BindUI[UiNode](pself.GetId(), "BG")
	pself.labelName = BindUI[UiNode](pself.GetId(), "BG/H/LabelName")
	pself.labelBg = BindUI[UiNode](pself.GetId(), "BG/H/C")
	pself.labelValue = BindUI[UiNode](pself.GetId(), "BG/H/C/H/LabelValue")

	pself.valueOnly = BindUI[UiNode](pself.GetId(), "ValueOnly")
	pself.labelValueOnly = BindUI[UiNode](pself.GetId(), "ValueOnly/LabelValue")

}
func (pself *UiMonitor) ShowAll(isOn bool) {
	engine.SyncUiSetVisible(pself.bgAll.GetId(), isOn)
	engine.SyncUiSetVisible(pself.valueOnly.GetId(), !isOn)
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	engine.SyncUiSetVisible(pself.GetId(), isOn)
}

func (pself *UiMonitor) UpdateScale(x float64) {
	engine.SyncUiSetScale(pself.GetId(), mathf.NewVec2(x, x))
}
func (pself *UiMonitor) UpdatePos(x, y float64) {
	pos := WorldToScreen(x, y)
	engine.SyncUiSetGlobalPosition(pself.GetId(), pos)
}

func (pself *UiMonitor) UpdateText(name, value string) {
	engine.SyncUiSetText(pself.labelName.GetId(), name)
	engine.SyncUiSetText(pself.labelValue.GetId(), value)
	engine.SyncUiSetText(pself.labelValueOnly.GetId(), value)
}
func (pself *UiMonitor) UpdateColor(color Color) {
	engine.SyncUiSetColor(pself.labelBg.GetId(), color)
	engine.SyncUiSetColor(pself.valueOnly.GetId(), color)
}
