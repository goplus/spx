package ui

import (
	"github.com/goplus/spbase/mathf"
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
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
	panel := engine.NewUiNode[UiMonitor]()
	return panel
}

// !!Warning: this method was called in main thread
func (pself *UiMonitor) OnStart() {
	pself.bgAll = engine.MainThreadBindUI[UiNode](pself.GetId(), "BG")
	pself.labelName = engine.MainThreadBindUI[UiNode](pself.GetId(), "BG/H/LabelName")
	pself.labelBg = engine.MainThreadBindUI[UiNode](pself.GetId(), "BG/H/C")
	pself.labelValue = engine.MainThreadBindUI[UiNode](pself.GetId(), "BG/H/C/H/LabelValue")

	pself.valueOnly = engine.MainThreadBindUI[UiNode](pself.GetId(), "ValueOnly")
	pself.labelValueOnly = engine.MainThreadBindUI[UiNode](pself.GetId(), "ValueOnly/LabelValue")

}
func (pself *UiMonitor) ShowAll(isOn bool) {
	engine.MainThreadUiSetVisible(pself.bgAll.GetId(), isOn)
	engine.MainThreadUiSetVisible(pself.valueOnly.GetId(), !isOn)
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	engine.MainThreadUiSetVisible(pself.GetId(), isOn)
}

func (pself *UiMonitor) UpdateScale(x float64) {
	x *= windowScale
	engine.MainThreadUiSetScale(pself.GetId(), mathf.NewVec2(x, x))
}
func (pself *UiMonitor) UpdatePos(wpos Vec2) {
	engine.MainThreadUiSetGlobalPosition(pself.GetId(), WorldToUI(wpos, true))
}

func (pself *UiMonitor) UpdateText(name, value string) {
	engine.MainThreadUiSetText(pself.labelName.GetId(), name)
	engine.MainThreadUiSetText(pself.labelValue.GetId(), value)
	engine.MainThreadUiSetText(pself.labelValueOnly.GetId(), value)
}
func (pself *UiMonitor) UpdateColor(color Color) {
	engine.MainThreadUiSetColor(pself.labelBg.GetId(), color)
	engine.MainThreadUiSetColor(pself.valueOnly.GetId(), color)
}
