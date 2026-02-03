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

import "github.com/goplus/spx/v2/internal/ui"

type quoterBubble struct {
	bubbleBase  // Embedded common bubble functionality
	message     string
	description string
	panel       *ui.UiQuote
}

func (pself *quoterBubble) onUpdate(delta float64) {
	if pself.checkNeedsUpdate() {
		pself.refresh()
		pself.markClean()
	}
}

func (pself *quoterBubble) refresh() {
	if pself.panel == nil {
		return
	}

	center, size := pself.getBounds()
	extSize := 10.0
	pself.panel.SetText(center, size.Divf(2).Addf(extSize), pself.message, pself.description)
}

// -------------------------------------------------------------------------------------

func (p *SpriteImpl) quote(message, description string) {
	bubble := p.components.Bubble()
	old := bubble.getQuoteObj()
	if old == nil {
		newQuote := &quoterBubble{
			bubbleBase:  bubbleBase{sprite: p, camera: p.g.camera, isDirty: true},
			message:     message,
			description: description,
		}
		bubble.setQuoteObj(newQuote)
		p.g.addShape(newQuote)
		newQuote.panel = ui.NewUiQuote()
	} else {
		old.message, old.description = message, description
		p.g.activateShape(old)
	}
	bubble.getQuoteObj().markDirty()
}

func (p *SpriteImpl) waitStopQuote(secs float64) {
	waitAndStop(secs, p.doStopQuote)
}

func (p *SpriteImpl) doStopQuote() {
	bubble := p.components.bubble
	if bubble != nil {
		bubble.stopQuote()
	}
}
