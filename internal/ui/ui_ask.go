package ui

import (
	"github.com/goplus/spx/v2/internal/engine"
)

type UiAsk struct {
	UiNode
	input    *UiNode
	checkBtn *UiNode
	OnCheck  func(string)
	askBody  *UiNode
	askLabel *UiNode
}

func NewUiAsk() *UiAsk {
	panel := engine.NewUiNode[UiAsk]()
	return panel
}

// !!Warning: this method was called in main thread
func (pself *UiAsk) OnStart() {
	pself.askBody = SyncBindUI[UiNode](pself.GetId(), "MF/Frame/AskBody")
	pself.askLabel = SyncBindUI[UiNode](pself.GetId(), "MF/Frame/AskBody/LabelAsk")

	pself.input = SyncBindUI[UiNode](pself.GetId(), "M/Input")
	pself.checkBtn = SyncBindUI[UiNode](pself.GetId(), "M/Input/Check")
	pself.checkBtn.OnUiClickEvent.Subscribe(func() {
		if pself.OnCheck != nil {
			pself.SetVisible(false)
			pself.OnCheck(pself.input.GetText())
		}
	})
}

func (pself *UiAsk) Show(isSprite bool, question string, onCheck func(string)) {
	// UiAsk prefab can auto scale to match window scale
	// uiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	pself.OnCheck = onCheck
	uiMgr.SetVisible(pself.askBody.GetId(), !isSprite)
	if !isSprite {
		uiMgr.SetText(pself.askLabel.GetId(), question)
	}
	uiMgr.SetText(pself.input.GetId(), "")
	uiMgr.SetVisible(pself.GetId(), true)
}
