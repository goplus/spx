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
	"strings"

	"github.com/goplus/spx/v4/internal/engine"
)

const (
	sayMsgSpliteWidth   = 25
	sayMsgLineHeight    = 26
	sayMsgDefaultHeight = 77
	thinkScale          = 0.8
)

const (
	StyleSay   = 1
	StyleThink = 2
)

// sayNodes holds the UI nodes for a specific style and direction
type sayNodes struct {
	vbox  *UiNode
	label *UiNode
}

// UiSay represents the UI component for displaying say and think bubbles
type UiSay struct {
	UiNode
	// Ordered by style, then direction: say left/right, think left/right.
	variants [4]sayNodes
}

// NewUiSay creates a new UiSay instance
func NewUiSay() *UiSay {
	return engine.NewUiNode[UiSay]()
}

// OnStart binds UI nodes in the engine callback context.
func (s *UiSay) OnStart() {
	for i, paths := range [4]struct{ vbox, label string }{
		{"VL", "VL/BG/Label"},
		{"VR", "VR/BG/Label"},
		{"VLThink", "VLThink/BG/MC/Label"},
		{"VRThink", "VRThink/BG/MC/Label"},
	} {
		s.variants[i] = sayNodes{
			vbox:  engine.BindUIForType[UiNode](s.GetId(), paths.vbox),
			label: engine.BindUIForType[UiNode](s.GetId(), paths.label),
		}
	}

	// Think nodes are visible by default in UiSay.tscn. Keep every variant
	// hidden until the first layout is applied so a newly-created bubble cannot
	// briefly render both clouds at the scene origin.
	for _, nodes := range s.variants {
		engine.Managers().UiMgr.SetVisible(nodes.vbox.GetId(), false)
	}
}

// SetTextLayout applies a layout resolved together with the other active bubbles.
func (s *UiSay) SetTextLayout(layout SayBubbleLayout) {
	selected := 0
	if !layout.isLeft {
		selected++
	}
	if layout.content.style == StyleThink {
		selected += 2
	}
	selectedNodes := s.variants[selected]
	for i, nodes := range s.variants {
		engine.Managers().UiMgr.SetVisible(nodes.vbox.GetId(), i == selected)
	}
	label := selectedNodes.label.GetId()
	engine.Managers().UiMgr.SetScale(s.GetId(), layout.renderScale)
	engine.Managers().UiMgr.SetPosition(s.GetId(), ViewToUI(layout.position))
	engine.Managers().UiMgr.SetText(label, layout.content.formattedMessage)
}

func formatSayMessage(msg string) string {
	if strings.ContainsRune(msg, '\n') {
		return msg
	}
	return splitTextLines(msg, sayMsgSpliteWidth)
}
