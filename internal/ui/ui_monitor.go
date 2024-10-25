package ui

import (
	. "godot-ext/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
)

type UiMonitor struct {
	UiNode
	labelName      *UiNode
	labelValue     *UiNode
	UpdateCallBack UpdateFunc
}
type UpdateFunc func(float32)

func NewUiMonitor() *UiMonitor {
	panel := engine.SyncCreateEngineUiNode[UiMonitor]("")
	return panel
}
func (pself *UiMonitor) OnStart() {
	pself.labelName = BindUI[UiNode](pself.GetId(), "BG/H/LabelName")
	pself.labelValue = BindUI[UiNode](pself.GetId(), "BG/H/C/H/LabelValue")
}

func (pself *UiMonitor) OnUpdate(delta float32) {
	if pself.UpdateCallBack != nil {
		pself.UpdateCallBack(delta)
	}
}

func (pself *UiMonitor) UpdateScale(x float64) {
	engine.SyncUiSetScale(pself.GetId(), engine.NewVec2(x, x))
}
func (pself *UiMonitor) UpdatePos(x, y float64) {
	pos := WorldToScreen(x, y)
	engine.SyncUiSetGlobalPosition(pself.GetId(), pos)
}

func (pself *UiMonitor) UpdateText(name, value string) {
	pself.labelName.SetText(name)
	pself.labelValue.SetText(value)
	engine.SyncUiSetText(pself.labelName.GetId(), name)
	engine.SyncUiSetText(pself.labelValue.GetId(), value)
}
