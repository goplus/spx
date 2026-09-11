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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/text"
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
	left       sayNodes
	right      sayNodes
	leftThink  sayNodes
	rightThink sayNodes
}

// OnStart binds UI nodes in the engine callback context.
func (s *UiSay) OnStart() {
	s.left = sayNodes{
		vbox:  engine.BridgeBindUI[UiNode](s.GetId(), "VL"),
		label: engine.BridgeBindUI[UiNode](s.GetId(), "VL/BG/Label"),
	}
	s.right = sayNodes{
		vbox:  engine.BridgeBindUI[UiNode](s.GetId(), "VR"),
		label: engine.BridgeBindUI[UiNode](s.GetId(), "VR/BG/Label"),
	}
	s.leftThink = sayNodes{
		vbox:  engine.BridgeBindUI[UiNode](s.GetId(), "VLThink"),
		label: engine.BridgeBindUI[UiNode](s.GetId(), "VLThink/BG/MC/Label"),
	}
	s.rightThink = sayNodes{
		vbox:  engine.BridgeBindUI[UiNode](s.GetId(), "VRThink"),
		label: engine.BridgeBindUI[UiNode](s.GetId(), "VRThink/BG/MC/Label"),
	}

	// Think nodes are visible by default in UiSay.tscn. Keep every variant
	// hidden until the first layout is applied so a newly-created bubble cannot
	// briefly render both clouds at the scene origin.
	mgr.UiMgr.SetVisible(s.left.vbox.GetId(), false)
	mgr.UiMgr.SetVisible(s.right.vbox.GetId(), false)
	mgr.UiMgr.SetVisible(s.leftThink.vbox.GetId(), false)
	mgr.UiMgr.SetVisible(s.rightThink.vbox.GetId(), false)
}

// SetText sets the text content and position for the say/think bubble
func (s *UiSay) SetText(winSize mathf.Vec2, pos mathf.Vec2, size mathf.Vec2, msg string, style int) {
	content := NewSayBubbleContent(msg, style)
	layout := NewSayBubbleLayoutContext(winSize).NewLayout(0, pos, size, content)
	s.SetTextLayout(layout)
}

// SetTextLayout applies a layout resolved together with the other active bubbles.
func (s *UiSay) SetTextLayout(layout SayBubbleLayout) {
	isThink := layout.content.style == StyleThink
	nodes := s.selectNodes(layout.isLeft, isThink)

	s.updateVisibility(layout.isLeft, isThink)
	s.updateUI(layout.position, layout.renderScale, nodes.label.GetId(), layout.content.formattedMessage)
}

// NewUiSay creates a new UiSay instance
func NewUiSay() *UiSay {
	return engine.NewUiNode[UiSay]()
}

func (s *UiSay) selectNodes(isLeft bool, isThink bool) sayNodes {
	switch {
	case isThink && isLeft:
		return s.leftThink
	case isThink:
		return s.rightThink
	case isLeft:
		return s.left
	default:
		return s.right
	}
}

func (s *UiSay) updateVisibility(isLeft bool, isThink bool) {
	mgr.UiMgr.SetVisible(s.left.vbox.GetId(), !isThink && isLeft)
	mgr.UiMgr.SetVisible(s.right.vbox.GetId(), !isThink && !isLeft)
	mgr.UiMgr.SetVisible(s.leftThink.vbox.GetId(), isThink && isLeft)
	mgr.UiMgr.SetVisible(s.rightThink.vbox.GetId(), isThink && !isLeft)
}

func (s *UiSay) updateUI(position mathf.Vec2, scale mathf.Vec2, label engine.Object, text string) {
	mgr.UiMgr.SetScale(s.GetId(), scale)
	mgr.UiMgr.SetPosition(s.GetId(), ViewToUI(position))
	mgr.UiMgr.SetText(label, text)
}

func formatSayMessage(msg string) string {
	if strings.ContainsRune(msg, '\n') {
		return msg
	}
	return text.SplitLines(msg, sayMsgSpliteWidth)
}
