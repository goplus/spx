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
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/ui"
)

const (
	quotePadding     = 5.0
	quoteLineWidth   = 8.0
	quoteHeadLen     = 16.0
	quoteTextPadding = 3.0
	quoteBorderRadis = 10.0
)

func init() {
	const dpi = 72
}

type quoter struct {
	sprite      *SpriteImpl
	message     string
	description string
	panel       *ui.UiQuote
}

func (p *quoter) onUpdate(delta float64) {
	p.refresh()
}

func (p *quoter) refresh() {
	if p.panel == nil || !p.sprite.Visible() {
		return
	}
	bound := p.sprite.bounds()
	center := bound.Center()
	size := bound.Size
	extSize := 10.0
	p.panel.SetText(center, size.Divf(2).Addf(extSize), p.message, p.description)
}

func (p *SpriteImpl) quote(message, description string) {
	bubble := p.components.Bubble()
	old := bubble.getQuoteObj()
	if old == nil {
		newQuote := &quoter{sprite: p, message: message, description: description}
		bubble.setQuoteObj(newQuote)
		p.g.addShape(newQuote)
		newQuote.panel = ui.NewUiQuote()
	} else {
		old.message, old.description = message, description
		p.g.activateShape(old)
	}
	bubble.getQuoteObj().refresh()
}

func (p *SpriteImpl) waitStopQuote(secs float64) {
	engine.Wait(secs)
	p.doStopQuote()
}

func (p *SpriteImpl) doStopQuote() {
	bubble := p.components.bubble
	if bubble != nil && bubble.hasQuote() {
		quoteObj := bubble.getQuoteObj()
		quoteObj.panel.Destroy()
		quoteObj.panel = nil
		p.g.removeShape(quoteObj)
		bubble.setQuoteObj(nil)
	}
}
