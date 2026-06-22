/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ui

import (
	"github.com/goplus/spx/v2/internal/engine"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

type UiAsk struct {
	UiNode
	input          *UiNode
	checkBtn       *UiNode
	OnCheck        func(string)
	askBody        *UiNode
	askLabel       *UiNode
	lastEnterState bool
}

func NewUiAsk() *UiAsk {
	panel := engine.NewUiNode[UiAsk]()
	return panel
}

// !!Warning: this method is called from the engine callback context
func (pself *UiAsk) OnStart() {
	pself.askBody = engine.BridgeBindUI[UiNode](pself.GetId(), "MF/Frame/AskBody")
	pself.askLabel = engine.BridgeBindUI[UiNode](pself.GetId(), "MF/Frame/AskBody/LabelAsk")

	pself.input = engine.BridgeBindUI[UiNode](pself.GetId(), "M/Input")
	pself.checkBtn = engine.BridgeBindUI[UiNode](pself.GetId(), "M/Input/Check")

	// Handle check button click
	pself.checkBtn.OnUiClickEvent.Subscribe(func() {
		pself.handleCheck()
	})
}

// handleCheck executes the check callback and closes the dialog
func (pself *UiAsk) handleCheck() {
	if pself.OnCheck != nil {
		pself.SetVisible(false)
		pself.OnCheck(pself.input.GetText())
	}
}

// OnUpdate checks for Enter key press every frame
func (pself *UiAsk) Update() {
	enterPressed := mgr.InputMgr.GetKey(int64(gdx.KeyEnter)) || mgr.InputMgr.GetKey(int64(gdx.KeyKPEnter))
	// Trigger only on key press (not held down)
	if enterPressed && !pself.lastEnterState {
		pself.handleCheck()
	}

	pself.lastEnterState = enterPressed
}

func (pself *UiAsk) Show(isSprite bool, question string, onCheck func(string)) {
	// UiAsk prefab can auto scale to match window scale
	// mgr.UiMgr.SetScale(pself.GetId(), mathf.NewVec2(windowScale, windowScale))
	pself.OnCheck = onCheck
	showQuestion := !isSprite && question != ""
	mgr.UiMgr.SetVisible(pself.askBody.GetId(), showQuestion)
	if showQuestion {
		mgr.UiMgr.SetText(pself.askLabel.GetId(), question)
	}
	mgr.UiMgr.SetText(pself.input.GetId(), "")
	mgr.UiMgr.SetVisible(pself.GetId(), true)
	pself.lastEnterState = false
}
