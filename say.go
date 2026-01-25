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

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/ui"
)

func init() {
	const dpi = 72

}

// -------------------------------------------------------------------------------------

type sayOrThinker struct {
	sprite *SpriteImpl
	msg    string
	style  int // styleSay, styleThink
	panel  *ui.UiSay
}

func (p *sayOrThinker) onUpdate(delta float64) {
	p.refresh()
}

func (p *sayOrThinker) refresh() {
	if p.panel == nil || !p.sprite.Visible() {
		return
	}

	bound := p.sprite.bounds()
	center := bound.Center()
	size := bound.Size
	p.panel.SetText(p.sprite.g.getWindowSize(), center, size, p.msg, p.style)
}

// -------------------------------------------------------------------------------------

func (p *SpriteImpl) sayOrThink(msg any, style int) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		p.doStopSay()
		return
	}

	bubble := p.components.Bubble()
	old := bubble.getSayObj()
	if old == nil {
		newSay := &sayOrThinker{sprite: p, msg: msgStr, style: style}
		bubble.setSayObj(newSay)
		p.g.addShape(newSay)
		newSay.panel = ui.NewUiSay()
	} else {
		old.msg, old.style = msgStr, style
		p.g.activateShape(old)
	}
	bubble.getSayObj().refresh()
}

func (p *SpriteImpl) waitStopSay(secs float64) {
	engine.Wait(secs)
	p.doStopSay()
}

func (p *SpriteImpl) doStopSay() {
	bubble := p.components.bubble
	if bubble != nil && bubble.hasSay() {
		sayObj := bubble.getSayObj()
		sayObj.panel.Destroy()
		sayObj.panel = nil
		p.g.removeShape(sayObj)
		bubble.setSayObj(nil)
	}
}

// -------------------------------------------------------------------------------------
