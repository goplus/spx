package ui

import (
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

type UiDebug struct {
	UiNode
	input *UiNode
}

func NewUiDebug() *UiDebug {
	panel := engine.NewUiNode[UiDebug]()
	return panel
}

// !!Warning: this method is called from the engine callback context
func (pself *UiDebug) OnStart() {
	pself.input = engine.BridgeBindUI[UiNode](pself.GetId(), "Label")
}

func (pself *UiDebug) Show(msg string) {
	mgr.UiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	mgr.UiMgr.SetVisible(pself.input.GetId(), msg != "")
	mgr.UiMgr.SetText(pself.input.GetId(), msg)
}
