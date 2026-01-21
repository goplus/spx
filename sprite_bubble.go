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

	spxlog "github.com/goplus/spx/v2/internal/log"
)

// ============================================================================
// Communication Methods (Say, Think, Ask, Quote)
// ============================================================================

func (p *SpriteImpl) Ask(msg any) {
	if debugInstr {
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
		p.doStopSay()
	})
}

func (p *SpriteImpl) Say__0(msg any) {
	p.Say__1(msg, 0)
}

func (p *SpriteImpl) Say__1(msg any, secs float64) {
	if debugInstr {
		spxlog.Debug("Say: sprite=%s, msg=%v, secs=%v", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleSay)
	if secs > 0 {
		p.waitStopSay(secs)
	}
}

func (p *SpriteImpl) Think__0(msg any) {
	p.Think__1(msg, 0)
}

func (p *SpriteImpl) Think__1(msg any, secs float64) {
	if debugInstr {
		spxlog.Debug("Think: sprite=%s, msg=%v, secs=%v", p.name, msg, secs)
	}
	p.sayOrThink(msg, styleThink)
	if secs > 0 {
		p.waitStopSay(secs)
	}
}

func (p *SpriteImpl) Quote__0(message string) {
	if message == "" {
		p.doStopQuote()
		return
	}
	p.Quote__2(message, "")
}

func (p *SpriteImpl) Quote__1(message string, secs float64) {
	p.Quote__3(message, "", secs)
}

func (p *SpriteImpl) Quote__2(message, description string) {
	p.Quote__3(message, description, 0)
}

func (p *SpriteImpl) Quote__3(message, description string, secs float64) {
	if debugInstr {
		spxlog.Debug("Quote: sprite=%s, message=%s, description=%s, secs=%v", p.name, message, description, secs)
	}
	p.quote(message, description)
	if secs > 0 {
		p.waitStopQuote(secs)
	}
}
