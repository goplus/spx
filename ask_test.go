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

package spx

import (
	"testing"

	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

const (
	askScenePath = "res://engine/ui/UiAsk.tscn"
	sayScenePath = "res://engine/ui/UiSay.tscn"
)

type askTestBinding struct {
	parent pkgengine.Object
	path   string
}

type askTestUiMgr struct {
	pkgengine.IUiMgr
	nextID       pkgengine.Object
	ids          map[pkgengine.Object]struct{}
	created      map[string]int
	bound        map[askTestBinding]pkgengine.Object
	visible      map[pkgengine.Object]bool
	text         map[pkgengine.Object]string
	askRoot      pkgengine.Object
	onAskVisible func()
}

func newAskTestUiMgr(base pkgengine.IUiMgr) *askTestUiMgr {
	return &askTestUiMgr{
		IUiMgr:  base,
		nextID:  100,
		ids:     make(map[pkgengine.Object]struct{}),
		created: make(map[string]int),
		bound:   make(map[askTestBinding]pkgengine.Object),
		visible: make(map[pkgengine.Object]bool),
		text:    make(map[pkgengine.Object]string),
	}
}

func (m *askTestUiMgr) newID() pkgengine.Object {
	m.nextID++
	m.ids[m.nextID] = struct{}{}
	return m.nextID
}

func (m *askTestUiMgr) CreateNode(path string) pkgengine.Object {
	id := m.newID()
	m.created[path]++
	if path == askScenePath {
		m.askRoot = id
	}
	return id
}

func (m *askTestUiMgr) BindNode(parent pkgengine.Object, path string) pkgengine.Object {
	id := m.newID()
	m.bound[askTestBinding{parent: parent, path: path}] = id
	return id
}

func (*askTestUiMgr) DestroyNode(pkgengine.Object) bool { return true }

func (m *askTestUiMgr) SetVisible(id pkgengine.Object, visible bool) {
	m.visible[id] = visible
	if id == m.askRoot && visible && m.onAskVisible != nil {
		onAskVisible := m.onAskVisible
		m.onAskVisible = nil
		onAskVisible()
	}
}

func (m *askTestUiMgr) SetText(id pkgengine.Object, text string) {
	m.text[id] = text
}

func (m *askTestUiMgr) GetText(id pkgengine.Object) string {
	return m.text[id]
}

func installAskTestUiMgr(t *testing.T) *askTestUiMgr {
	t.Helper()
	enginewrap.Init(func(call func()) { call() })
	original := pkgengine.UiMgr
	mgr := newAskTestUiMgr(original)
	pkgengine.UiMgr = mgr
	t.Cleanup(func() {
		for id := range mgr.ids {
			pkgengine.DeleteUINode(id)
		}
		pkgengine.UiMgr = original
		enginewrap.Init(engine.WaitMainThread)
	})
	return mgr
}

func TestAskQuestionPlacement(t *testing.T) {
	tests := []struct {
		name          string
		useStage      bool
		spriteVisible bool
		wantPanel     bool
		wantBubble    bool
	}{
		{name: "stage", useStage: true, wantPanel: true},
		{name: "visible sprite", spriteVisible: true, wantBubble: true},
		{name: "hidden sprite", wantPanel: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mgr := installAskTestUiMgr(t)
			game := &Game{}
			game.shapeMgr.init()
			mgr.onAskVisible = func() {
				game.dialogState.AskPanel.OnCheck("answer")
			}

			const question = "How many?"
			if test.useStage {
				game.Ask(question)
			} else {
				sprite := &SpriteImpl{g: game}
				sprite.spriteState.IsVisible = test.spriteVisible
				sprite.Ask(question)
			}

			askBody := mgr.bound[askTestBinding{parent: mgr.askRoot, path: "MF/Frame/AskBody"}]
			if got := mgr.visible[askBody]; got != test.wantPanel {
				t.Errorf("question panel visible = %v, want %v", got, test.wantPanel)
			}
			askLabel := mgr.bound[askTestBinding{parent: mgr.askRoot, path: "MF/Frame/AskBody/LabelAsk"}]
			if test.wantPanel && mgr.text[askLabel] != question {
				t.Errorf("question panel text = %q, want %q", mgr.text[askLabel], question)
			}
			if got := mgr.created[sayScenePath] != 0; got != test.wantBubble {
				t.Errorf("question bubble created = %v, want %v", got, test.wantBubble)
			}
			if got := game.Answer(); got != "answer" {
				t.Errorf("answer = %q, want %q", got, "answer")
			}
		})
	}
}
