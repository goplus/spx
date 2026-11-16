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
	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/ui"
)

func (p *SpriteImpl) Ask(question any) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Ask: sprite=%s, msg=%v", p.name, question)
	}
	msgStr := dialogText(question)

	isVisible := p.Visible()
	if isVisible {
		p.sayOrThink(msgStr, ui.StyleSay)
	}
	showQuestionInPanel := !isVisible
	p.g.ask(showQuestionInPanel, msgStr, func(answer string) {
		p.stopTextBubble()
	})
}

func (p *SpriteImpl) Say__0(word any) {
	p.SayWith(word, 0)
}

func (p *SpriteImpl) Say__1(word any, seconds Seconds) {
	p.SayWith(word, seconds)
}

func (p *SpriteImpl) SayWith(word any, __xgo_optional_seconds Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Say: sprite=%s, msg=%v, secs=%v", p.name, word, __xgo_optional_seconds)
	}
	token := p.sayOrThink(word, ui.StyleSay)
	if __xgo_optional_seconds > 0 {
		p.waitAndExpireText(__xgo_optional_seconds, token)
	}
}

func (p *SpriteImpl) Think__0(word any) {
	p.ThinkWith(word, 0)
}

func (p *SpriteImpl) Think__1(word any, seconds Seconds) {
	p.ThinkWith(word, seconds)
}

func (p *SpriteImpl) ThinkWith(word any, __xgo_optional_seconds Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Think: sprite=%s, msg=%v, secs=%v", p.name, word, __xgo_optional_seconds)
	}
	token := p.sayOrThink(word, ui.StyleThink)
	if __xgo_optional_seconds > 0 {
		p.waitAndExpireText(__xgo_optional_seconds, token)
	}
}

func (p *SpriteImpl) Quote__0(message string) {
	p.QuoteMsg(message, 0)
}

func (p *SpriteImpl) Quote__1(message string, secs Seconds) {
	p.QuoteMsg(message, secs)
}

func (p *SpriteImpl) Quote__2(message, description string) {
	p.QuoteMsgEx(message, description, 0)
}

func (p *SpriteImpl) Quote__3(message, description string, secs Seconds) {
	p.QuoteMsgEx(message, description, secs)
}

func (p *SpriteImpl) QuoteMsg(message string, __xgo_optional_secs Seconds) {
	if message == "" {
		p.stopQuoteBubble()
		return
	}
	p.QuoteMsgEx(message, "", __xgo_optional_secs)
}

func (p *SpriteImpl) QuoteMsgEx(message, description string, __xgo_optional_secs Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Quote: sprite=%s, message=%s, description=%s, secs=%v", p.name, message, description, __xgo_optional_secs)
	}
	token := p.bubble().upsertQuote(message, description)
	if __xgo_optional_secs > 0 {
		p.waitAndExpireQuote(__xgo_optional_secs, token)
	}
}

func (p *SpriteImpl) sayOrThink(msg any, style int) bubbleToken {
	msgStr := dialogText(msg)
	if msgStr == "" {
		p.stopTextBubble()
		return bubbleToken{}
	}

	bubble := p.bubble()
	return bubble.upsertText(msgStr, style)
}

func (p *SpriteImpl) waitAndExpireText(secs float64, token bubbleToken) {
	engine.Wait(secs)
	bubble := p.components.bubble
	if bubble != nil {
		// Expire only the generation displayed by this call.
		bubble.expireText(token)
	}
}

func (p *SpriteImpl) stopTextBubble() {
	bubble := p.components.bubble
	if bubble != nil {
		bubble.stopText()
	}
}

func (p *SpriteImpl) waitAndExpireQuote(secs float64, token bubbleToken) {
	engine.Wait(secs)
	bubble := p.components.bubble
	if bubble != nil {
		bubble.expireQuote(token)
	}
}

func (p *SpriteImpl) stopQuoteBubble() {
	bubble := p.components.bubble
	if bubble != nil {
		bubble.stopQuote()
	}
}

func (p *SpriteImpl) newBubbleBase() bubbleBase {
	return bubbleBase{sprite: p, camera: p.g.camera, isDirty: true}
}

func (p *SpriteImpl) bubbleBounds() *mathf.Rect2 {
	if !p.hasCostume() {
		return nil
	}

	x, y := p.getXY()
	w, h := p.adjustPositionAndGetDimensions(&x, &y)

	return &mathf.Rect2{
		Position: mathf.Vec2{X: x - w*0.5, Y: y - h*0.5},
		Size:     mathf.Vec2{X: w, Y: h},
	}
}
