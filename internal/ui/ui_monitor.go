package ui

import (
	"github.com/goplus/spbase/mathf"
	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/internal/engine"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
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
	gdx.UiMgr.SetVisible(pself.bgAll.GetId(), isOn)
	gdx.UiMgr.SetVisible(pself.valueOnly.GetId(), !isOn)
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	gdx.UiMgr.SetVisible(pself.GetId(), isOn)
}

func (pself *UiMonitor) UpdateScale(x float64) {
	x *= windowScale
	gdx.UiMgr.SetScale(pself.GetId(), mathf.NewVec2(x, x))
}
func (pself *UiMonitor) UpdatePos(wpos Vec2) {
	// Use WorldToUI with useDirect=true to avoid cameraMgr deadlock when called from main thread
	gdx.UiMgr.SetGlobalPosition(pself.GetId(), WorldToUI(wpos, true))
}

func (pself *UiMonitor) UpdateText(name, value string) {
	gdx.UiMgr.SetText(pself.labelName.GetId(), name)
	gdx.UiMgr.SetText(pself.labelValue.GetId(), value)
	gdx.UiMgr.SetText(pself.labelValueOnly.GetId(), value)
}
func (pself *UiMonitor) UpdateColor(color Color) {
	gdx.UiMgr.SetColor(pself.labelBg.GetId(), color)
	gdx.UiMgr.SetColor(pself.valueOnly.GetId(), color)
}
