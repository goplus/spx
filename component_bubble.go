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
	"sync"

	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/goplus/spx/v3/internal/ui"
)

// ============================================================================
// Bubble Component
// ============================================================================
// This component manages Say/Think and Quote bubbles for sprites.

type bubbleComponent struct {
	sprite *SpriteImpl
	mu     sync.Mutex

	textObj  *textBubble   // Text bubble object (Say/Think).
	quoteObj *quoterBubble // Quote bubble object.
}

type bubbleShape interface {
	destroyPanel()
	onUpdate(float64)
	flushPendingRemoval(int64)
}

// bubbleToken limits timed removal to one displayed generation.
type bubbleToken struct {
	bubble     *bubbleBase
	generation uint64
}

func (t bubbleToken) matches(bubble *bubbleBase) bool {
	return bubble != nil && t.bubble == bubble && t.generation == bubble.generation
}

// ============================================================================
// Lifecycle
// ============================================================================

// onDestroy cleans up when the component is destroyed.
func (b *bubbleComponent) onDestroy() {
	b.stopAll()
}

// ============================================================================
// Bubble Control
// ============================================================================

func (b *bubbleComponent) upsertText(msg string, style int) bubbleToken {
	b.sprite.requestRedrawIfVisible()
	b.mu.Lock()
	textObj := b.textObj
	created := false
	if textObj == nil {
		textObj = &textBubble{
			bubbleBase: b.sprite.newBubbleBase(),
			msg:        msg,
			style:      style,
			panel:      ui.NewUiSay(),
			content:    ui.NewSayBubbleContent(msg, style),
		}
		b.textObj = textObj
		created = true
	} else {
		if textObj.msg != msg || textObj.style != style {
			textObj.content = ui.NewSayBubbleContent(msg, style)
		}
		textObj.msg = msg
		textObj.style = style
		textObj.markDirty()
	}
	// Every call supersedes older timed removals, even if the content is unchanged.
	textObj.generation++
	if textObj.generation == 0 {
		// Zero represents an invalid token.
		textObj.generation++
	}
	textObj.timedRemovalPending = false
	token := bubbleToken{bubble: &textObj.bubbleBase, generation: textObj.generation}
	b.mu.Unlock()

	if created {
		b.sprite.g.shapeMgr.add(textObj)
		return token
	}
	b.sprite.g.shapeMgr.activateShape(textObj)
	return token
}

func (b *bubbleComponent) upsertQuote(message, description string) bubbleToken {
	b.sprite.requestRedrawIfVisible()
	b.mu.Lock()
	quoteObj := b.quoteObj
	created := false
	if quoteObj == nil {
		quoteObj = &quoterBubble{
			bubbleBase:  b.sprite.newBubbleBase(),
			message:     message,
			description: description,
			panel:       ui.NewUiQuote(),
		}
		b.quoteObj = quoteObj
		created = true
	} else {
		quoteObj.message = message
		quoteObj.description = description
		quoteObj.markDirty()
	}
	quoteObj.generation++
	if quoteObj.generation == 0 {
		quoteObj.generation++
	}
	quoteObj.timedRemovalPending = false
	token := bubbleToken{bubble: &quoteObj.bubbleBase, generation: quoteObj.generation}
	b.mu.Unlock()

	if created {
		b.sprite.g.shapeMgr.add(quoteObj)
		return token
	}
	b.sprite.g.shapeMgr.activateShape(quoteObj)
	return token
}

func (b *bubbleComponent) stopText() {
	b.mu.Lock()
	textObj := b.textObj
	b.textObj = nil
	if textObj != nil {
		textObj.timedRemovalPending = false
	}
	b.mu.Unlock()
	if textObj == nil {
		return
	}
	b.stopBubble(textObj)
}

// expireText defers removal so the next frame can renew the bubble.
func (b *bubbleComponent) expireText(token bubbleToken) {
	b.mu.Lock()
	if b.textObj == nil || !token.matches(&b.textObj.bubbleBase) {
		b.mu.Unlock()
		return
	}
	token.bubble.timedRemovalPending = true
	token.bubble.timedRemovalFrame = itime.Frame()
	b.mu.Unlock()

	engine.RequestRedraw()
}

// flushPendingTextRemoval commits expirations after their grace frame.
func (b *bubbleComponent) flushPendingTextRemoval(textObj *textBubble, frame int64) {
	b.mu.Lock()
	if b.textObj != textObj || !textObj.timedRemovalPending || textObj.timedRemovalFrame >= frame {
		b.mu.Unlock()
		return
	}
	b.textObj = nil
	textObj.timedRemovalPending = false
	b.mu.Unlock()

	b.stopBubble(textObj)
}

// expireQuote defers removal so the next frame can renew the bubble.
func (b *bubbleComponent) expireQuote(token bubbleToken) {
	b.mu.Lock()
	if b.quoteObj == nil || !token.matches(&b.quoteObj.bubbleBase) {
		b.mu.Unlock()
		return
	}
	token.bubble.timedRemovalPending = true
	token.bubble.timedRemovalFrame = itime.Frame()
	b.mu.Unlock()

	engine.RequestRedraw()
}

// flushPendingQuoteRemoval commits expirations after their grace frame.
func (b *bubbleComponent) flushPendingQuoteRemoval(quoteObj *quoterBubble, frame int64) {
	b.mu.Lock()
	if b.quoteObj != quoteObj || !quoteObj.timedRemovalPending || quoteObj.timedRemovalFrame >= frame {
		b.mu.Unlock()
		return
	}
	b.quoteObj = nil
	quoteObj.timedRemovalPending = false
	b.mu.Unlock()

	b.stopBubble(quoteObj)
}

func (b *bubbleComponent) stopQuote() {
	b.mu.Lock()
	quoteObj := b.quoteObj
	b.quoteObj = nil
	if quoteObj != nil {
		quoteObj.timedRemovalPending = false
	}
	b.mu.Unlock()
	if quoteObj == nil {
		return
	}
	b.stopBubble(quoteObj)
}

func (b *bubbleComponent) stopAll() {
	b.stopText()
	b.stopQuote()
}

// ============================================================================
// Internal Bubble Management
// ============================================================================

func (b *bubbleComponent) stopBubble(obj bubbleShape) {
	engine.RequestRedraw()
	obj.destroyPanel()
	b.sprite.g.shapeMgr.removeShape(obj)
}
