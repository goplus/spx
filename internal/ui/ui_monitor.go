package ui

import (
	"github.com/realdream-ai/mathf"
	. "github.com/realdream-ai/mathf"

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
	pself.bgAll = SyncBindUI[UiNode](pself.GetId(), "BG")
	pself.labelName = SyncBindUI[UiNode](pself.GetId(), "BG/H/LabelName")
	pself.labelBg = SyncBindUI[UiNode](pself.GetId(), "BG/H/C")
	pself.labelValue = SyncBindUI[UiNode](pself.GetId(), "BG/H/C/H/LabelValue")

	pself.valueOnly = SyncBindUI[UiNode](pself.GetId(), "ValueOnly")
	pself.labelValueOnly = SyncBindUI[UiNode](pself.GetId(), "ValueOnly/LabelValue")

}
func (pself *UiMonitor) ShowAll(isOn bool) {
	uiMgr.SetVisible(pself.bgAll.GetId(), isOn)
	uiMgr.SetVisible(pself.valueOnly.GetId(), !isOn)
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	uiMgr.SetVisible(pself.GetId(), isOn)
}

func (pself *UiMonitor) UpdateScale(x float64) {
	x *= windowScale
	uiMgr.SetScale(pself.GetId(), mathf.NewVec2(x, x))
}
func (pself *UiMonitor) UpdatePos(wpos Vec2) {
	pos := WorldToUI(wpos)
	uiMgr.SetGlobalPosition(pself.GetId(), pos)
}

func (pself *UiMonitor) UpdateText(name, value string) {
	uiMgr.SetText(pself.labelName.GetId(), name)
	uiMgr.SetText(pself.labelValue.GetId(), value)
	uiMgr.SetText(pself.labelValueOnly.GetId(), value)
}
func (pself *UiMonitor) UpdateColor(color Color) {
	uiMgr.SetColor(pself.labelBg.GetId(), color)
	uiMgr.SetColor(pself.valueOnly.GetId(), color)
}
