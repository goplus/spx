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
	"testing"

	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type askShowUiMgr struct {
	gdx.IUiMgr
	visible map[gdx.Object]bool
	text    map[gdx.Object]string
}

func (m *askShowUiMgr) SetVisible(id gdx.Object, visible bool) {
	m.visible[id] = visible
}

func (m *askShowUiMgr) SetText(id gdx.Object, text string) {
	m.text[id] = text
}

func TestUiAskShow(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	original := gdx.UiMgr
	mgr := &askShowUiMgr{IUiMgr: original}
	gdx.UiMgr = mgr
	t.Cleanup(func() {
		gdx.UiMgr = original
		enginewrap.Init(engine.WaitMainThread)
	})

	const (
		rootID     gdx.Object = 1
		askBodyID  gdx.Object = 2
		askLabelID gdx.Object = 3
		inputID    gdx.Object = 4
	)
	newNode := func(id gdx.Object) *UiNode {
		node := &UiNode{}
		node.SetId(id)
		return node
	}
	panel := &UiAsk{
		input:    newNode(inputID),
		askBody:  newNode(askBodyID),
		askLabel: newNode(askLabelID),
	}
	panel.SetId(rootID)

	tests := []struct {
		name                string
		showQuestionInPanel bool
		question            string
		wantQuestionVisible bool
		wantQuestionText    bool
	}{
		{
			name:                "show question",
			showQuestionInPanel: true,
			question:            "How many?",
			wantQuestionVisible: true,
			wantQuestionText:    true,
		},
		{
			name:     "hide panel question",
			question: "How many?",
		},
		{
			name:                "empty question",
			showQuestionInPanel: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mgr.visible = make(map[gdx.Object]bool)
			mgr.text = make(map[gdx.Object]string)
			panel.lastEnterState = true
			callbackCalled := false

			panel.Show(test.showQuestionInPanel, test.question, func(string) {
				callbackCalled = true
			})

			if got := mgr.visible[askBodyID]; got != test.wantQuestionVisible {
				t.Errorf("question visible = %v, want %v", got, test.wantQuestionVisible)
			}
			questionText, questionTextSet := mgr.text[askLabelID]
			if questionTextSet != test.wantQuestionText {
				t.Errorf("question text set = %v, want %v", questionTextSet, test.wantQuestionText)
			}
			if test.wantQuestionText && questionText != test.question {
				t.Errorf("question text = %q, want %q", questionText, test.question)
			}
			if got := mgr.text[inputID]; got != "" {
				t.Errorf("input text = %q, want empty", got)
			}
			if !mgr.visible[rootID] {
				t.Error("Ask root was not shown")
			}
			if panel.lastEnterState {
				t.Error("last Enter state was not reset")
			}
			panel.OnCheck("answer")
			if !callbackCalled {
				t.Error("Ask callback was not retained")
			}
		})
	}
}
