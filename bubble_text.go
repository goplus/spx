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
	"fmt"

	"github.com/goplus/spx/v2/internal/ui"
)

type textBubble struct {
	bubbleBase // Embedded common bubble functionality
	msg        string
	style      int // styleSay, styleThink
	panel      *ui.UiSay
}

func (pself *textBubble) onUpdate(delta float64) {
	if pself.checkNeedsUpdate() {
		pself.refresh()
		pself.markClean()
	}
}

func (pself *textBubble) refresh() {
	if pself.panel == nil {
		return
	}
	center, size := pself.getBounds()
	pself.panel.SetText(pself.sprite.g.getWindowSize(), center, size, pself.msg, pself.style)
}

// -------------------------------------------------------------------------------------

func (p *SpriteImpl) sayOrThink(msg any, style int) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		p.doStopText()
		return
	}

	bubble := p.components.Bubble()
	old := bubble.getTextObj()
	if old == nil {
		newSay := &textBubble{
			bubbleBase: bubbleBase{sprite: p, camera: p.g.camera, isDirty: true},
			msg:        msgStr,
			style:      style,
		}
		bubble.setTextObj(newSay)
		p.g.addShape(newSay)
		newSay.panel = ui.NewUiSay()
	} else {
		old.msg, old.style = msgStr, style
		p.g.activateShape(old)
	}
	bubble.getTextObj().markDirty()
}

func (p *SpriteImpl) waitStopText(secs float64) {
	waitAndStop(secs, p.doStopText)
}

func (p *SpriteImpl) doStopText() {
	bubble := p.components.bubble
	if bubble != nil {
		bubble.stopText()
	}
}
