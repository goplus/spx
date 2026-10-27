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
	"github.com/goplus/spx/v4/internal/engine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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
	return engine.NewUiNode[UiAsk]()
}

// OnStart binds UI nodes and subscribes to clicks in the engine callback context.
func (p *UiAsk) OnStart() {
	p.askBody = engine.BridgeBindUI[UiNode](p.GetId(), "MF/Frame/AskBody")
	p.askLabel = engine.BridgeBindUI[UiNode](p.GetId(), "MF/Frame/AskBody/LabelAsk")

	p.input = engine.BridgeBindUI[UiNode](p.GetId(), "M/Input")
	p.checkBtn = engine.BridgeBindUI[UiNode](p.GetId(), "M/Input/Check")

	p.checkBtn.OnUiClickEvent.Subscribe(func() {
		p.handleCheck()
	})
}

// Update handles Enter key presses.
func (p *UiAsk) Update() {
	enterPressed := engine.Managers().InputMgr.GetKey(int64(gdx.KeyEnter)) || engine.Managers().InputMgr.GetKey(int64(gdx.KeyKPEnter))
	// Trigger only on key press (not held down)
	if enterPressed && !p.lastEnterState {
		p.handleCheck()
	}

	p.lastEnterState = enterPressed
}

func (p *UiAsk) Show(showQuestionInPanel bool, question string, onCheck func(string)) {
	p.OnCheck = onCheck
	showQuestion := showQuestionInPanel && question != ""
	engine.Managers().UiMgr.SetVisible(p.askBody.GetId(), showQuestion)
	if showQuestion {
		engine.Managers().UiMgr.SetText(p.askLabel.GetId(), question)
	}
	engine.Managers().UiMgr.SetText(p.input.GetId(), "")
	engine.Managers().UiMgr.SetVisible(p.GetId(), true)
	p.lastEnterState = false
}

// handleCheck hides the dialog before invoking the callback, if present.
func (p *UiAsk) handleCheck() {
	if p.OnCheck != nil {
		p.SetVisible(false)
		p.OnCheck(p.input.GetText())
	}
}
