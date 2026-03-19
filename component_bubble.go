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

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/ui"
)

// ============================================================================
// Bubble Component
// ============================================================================
// This component manages Say/Think and Quote bubbles for sprites

type bubbleComponent struct {
	componentBase
	mu sync.Mutex

	textObj  *textBubble   // Text bubble object (Say/Think)
	quoteObj *quoterBubble // Quote bubble object
}

type bubbleShape interface {
	destroyPanel()
}

// initialize initializes the bubble component.
func (b *bubbleComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	b.componentBase.initialize(sprite, spriteCfg)
	// Bubbles are created on-demand (lazy initialization)
	b.textObj = nil
	b.quoteObj = nil
}

// cloneFrom creates a new bubble component by cloning from source.
func (b *bubbleComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	// Bubbles are NOT cloned - each sprite starts with clean bubbles
	newBubble := &bubbleComponent{
		componentBase: componentBase{sprite: newSprite},
		textObj:       nil,
		quoteObj:      nil,
	}
	return newBubble
}

// onDestroy cleanup when component is destroyed.
func (b *bubbleComponent) onDestroy() {
	b.stopAll()
}

func (b *bubbleComponent) upsertText(msg string, style int) {
	b.mu.Lock()
	textObj := b.textObj
	created := false
	if textObj == nil {
		textObj = &textBubble{
			bubbleBase: b.sprite.newBubbleBase(),
			msg:        msg,
			style:      style,
			panel:      ui.NewUiSay(),
		}
		b.textObj = textObj
		created = true
	} else {
		textObj.msg = msg
		textObj.style = style
		textObj.markDirty()
	}
	b.mu.Unlock()

	if created {
		b.sprite.g.addShape(textObj)
		return
	}
	b.sprite.g.activateShape(textObj)
}

func (b *bubbleComponent) upsertQuote(message, description string) {
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
	b.mu.Unlock()

	if created {
		b.sprite.g.addShape(quoteObj)
		return
	}
	b.sprite.g.activateShape(quoteObj)
}

func (b *bubbleComponent) stopText() {
	b.mu.Lock()
	textObj := b.textObj
	b.textObj = nil
	b.mu.Unlock()
	if textObj == nil {
		return
	}
	b.stopBubble(textObj)
}

func (b *bubbleComponent) stopQuote() {
	b.mu.Lock()
	quoteObj := b.quoteObj
	b.quoteObj = nil
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

func (b *bubbleComponent) stopBubble(obj bubbleShape) {
	obj.destroyPanel()
	b.sprite.g.removeShape(obj)
}
