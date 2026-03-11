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

import coreproject "github.com/goplus/spx/v2/internal/core/project"

// ============================================================================
// Bubble Component
// ============================================================================
// This component manages Say/Think and Quote bubbles for sprites

type bubbleComponent struct {
	componentBase

	textObj  *textBubble   // Text bubble object (Say/Think)
	quoteObj *quoterBubble // Quote bubble object
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
	// Bubbles will be cleaned up by sprite manager
	b.textObj = nil
	b.quoteObj = nil
}

// ============================================================================
// Say/Think Methods
// ============================================================================

func (b *bubbleComponent) getTextObj() *textBubble {
	return b.textObj
}

func (b *bubbleComponent) setTextObj(obj *textBubble) {
	b.textObj = obj
}

func (b *bubbleComponent) hasText() bool {
	return b.textObj != nil
}

func (b *bubbleComponent) stopText() {
	if b.hasText() {
		textObj := b.getTextObj()
		textObj.panel.Destroy()
		textObj.panel = nil
		b.sprite.g.removeShape(textObj)
		b.setTextObj(nil)
	}
}

// ============================================================================
// Quote Methods
// ============================================================================

func (b *bubbleComponent) getQuoteObj() *quoterBubble {
	return b.quoteObj
}

func (b *bubbleComponent) setQuoteObj(obj *quoterBubble) {
	b.quoteObj = obj
}

func (b *bubbleComponent) hasQuote() bool {
	return b.quoteObj != nil
}

func (b *bubbleComponent) stopQuote() {
	if b.hasQuote() {
		quoteObj := b.getQuoteObj()
		quoteObj.panel.Destroy()
		quoteObj.panel = nil
		b.sprite.g.removeShape(quoteObj)
		b.setQuoteObj(nil)
	}
}
