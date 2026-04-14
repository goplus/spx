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

	spxlog "github.com/goplus/spx/v2/internal/log"
)

// ============================================================================
// Communication Methods (Say, Think, Ask, Quote)
// ============================================================================

func (p *SpriteImpl) Ask(msg any) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Ask: sprite=%s, msg=%v", p.name, msg)
	}
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		spxlog.Warn("ask: msg should not be empty")
		return
	}
	p.Say__0(msgStr)
	p.g.ask(true, msgStr, func(answer string) {
		p.doStopText()
	})
}

func (p *SpriteImpl) Say__0(msg any) {
	p.SayWith(msg, 0)
}

func (p *SpriteImpl) Say__1(msg any, secs Seconds) {
	p.SayWith(msg, secs)
}

func (p *SpriteImpl) SayWith(msg any, __xgo_optional_secs Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Say: sprite=%s, msg=%v, secs=%v", p.name, msg, __xgo_optional_secs)
	}
	p.sayOrThink(msg, ui.StyleSay)
	if __xgo_optional_secs > 0 {
		p.waitStopText(__xgo_optional_secs)
	}
}

func (p *SpriteImpl) Think__0(msg any) {
	p.ThinkWith(msg, 0)
}

func (p *SpriteImpl) Think__1(msg any, secs Seconds) {
	p.ThinkWith(msg, secs)
}

func (p *SpriteImpl) ThinkWith(msg any, __xgo_optional_secs Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Think: sprite=%s, msg=%v, secs=%v", p.name, msg, __xgo_optional_secs)
	}
	p.sayOrThink(msg, ui.StyleThink)
	if __xgo_optional_secs > 0 {
		p.waitStopText(__xgo_optional_secs)
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
		p.doStopQuote()
		return
	}
	p.QuoteMsgEx(message, "", __xgo_optional_secs)
}

func (p *SpriteImpl) QuoteMsgEx(message, description string, __xgo_optional_secs Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Quote: sprite=%s, message=%s, description=%s, secs=%v", p.name, message, description, __xgo_optional_secs)
	}
	p.quote(message, description)
	if __xgo_optional_secs > 0 {
		p.waitStopQuote(__xgo_optional_secs)
	}
}
