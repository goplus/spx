package ui

import (
	"github.com/goplus/spx/internal/engine"
	"github.com/realdream-ai/mathf"
)

type UiDebug struct {
	UiNode
	input *UiNode
}

func NewUiDebug() *UiDebug {
	panel := engine.NewUiNode[UiDebug]()
	return panel
}

// !!Warning: this method was called in main thread
func (pself *UiDebug) OnStart() {
	pself.input = SyncBindUI[UiNode](pself.GetId(), "Label")
}

func (pself *UiDebug) Show(msg string) {
	uiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	uiMgr.SetVisible(pself.input.GetId(), msg != "")
	uiMgr.SetText(pself.input.GetId(), msg)
}
