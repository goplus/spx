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

// ============================================================================
// Bubble Component
// ============================================================================
// This component manages Say/Think and Quote bubbles for sprites

type bubbleComponent struct {
	componentBase

	textObj  *textBubble   // Text bubble object (Say/Think)
	quoteObj *quoterBubble // Quote bubble object
}

// initialize initializes the bubble component
func (bc *bubbleComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	bc.componentBase.initialize(sprite, spriteCfg)
	// Bubbles are created on-demand (lazy initialization)
	bc.textObj = nil
	bc.quoteObj = nil
}

// cloneFrom creates a new bubble component by cloning from source
func (bc *bubbleComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	// Bubbles are NOT cloned - each sprite starts with clean bubbles
	newBubble := &bubbleComponent{
		componentBase: componentBase{sprite: newSprite},
		textObj:       nil,
		quoteObj:      nil,
	}
	return newBubble
}

// onDestroy cleanup when component is destroyed
func (bc *bubbleComponent) onDestroy() {
	// Bubbles will be cleaned up by sprite manager
	bc.textObj = nil
	bc.quoteObj = nil
}

// ============================================================================
// Say/Think Methods
// ============================================================================

func (bc *bubbleComponent) getTextObj() *textBubble {
	return bc.textObj
}

func (bc *bubbleComponent) setTextObj(obj *textBubble) {
	bc.textObj = obj
}

func (bc *bubbleComponent) hasText() bool {
	return bc.textObj != nil
}

func (bc *bubbleComponent) stopText() {
	if bc.hasText() {
		textObj := bc.getTextObj()
		textObj.panel.Destroy()
		textObj.panel = nil
		bc.sprite.g.removeShape(textObj)
		bc.setTextObj(nil)
	}
}

// ============================================================================
// Quote Methods
// ============================================================================

func (bc *bubbleComponent) getQuoteObj() *quoterBubble {
	return bc.quoteObj
}

func (bc *bubbleComponent) setQuoteObj(obj *quoterBubble) {
	bc.quoteObj = obj
}

func (bc *bubbleComponent) hasQuote() bool {
	return bc.quoteObj != nil
}

func (bc *bubbleComponent) stopQuote() {
	if bc.hasQuote() {
		quoteObj := bc.getQuoteObj()
		quoteObj.panel.Destroy()
		quoteObj.panel = nil
		bc.sprite.g.removeShape(quoteObj)
		bc.setQuoteObj(nil)
	}
}
