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

	sayObj   *sayOrThinker // Say/Think bubble object
	quoteObj *quoter       // Quote bubble object
}

// initialize initializes the bubble component
func (bc *bubbleComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	bc.componentBase.initialize(sprite, spriteCfg)
	// Bubbles are created on-demand (lazy initialization)
	bc.sayObj = nil
	bc.quoteObj = nil
}

// cloneFrom creates a new bubble component by cloning from source
func (bc *bubbleComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	// Bubbles are NOT cloned - each sprite starts with clean bubbles
	newBubble := &bubbleComponent{
		componentBase: componentBase{sprite: newSprite},
		sayObj:        nil,
		quoteObj:      nil,
	}
	return newBubble
}

// onDestroy cleanup when component is destroyed
func (bc *bubbleComponent) onDestroy() {
	// Bubbles will be cleaned up by sprite manager
	bc.sayObj = nil
	bc.quoteObj = nil
}

// ============================================================================
// Refresh Methods
// ============================================================================

// refresh updates bubble positions (called every frame for visible sprites)
func (bc *bubbleComponent) refresh() {
	if bc.quoteObj != nil {
		bc.quoteObj.refresh()
	}
	if bc.sayObj != nil {
		bc.sayObj.refresh()
	}
}

// ============================================================================
// Say/Think Methods
// ============================================================================

func (bc *bubbleComponent) getSayObj() *sayOrThinker {
	return bc.sayObj
}

func (bc *bubbleComponent) setSayObj(obj *sayOrThinker) {
	bc.sayObj = obj
}

func (bc *bubbleComponent) hasSay() bool {
	return bc.sayObj != nil
}

// ============================================================================
// Quote Methods
// ============================================================================

func (bc *bubbleComponent) getQuoteObj() *quoter {
	return bc.quoteObj
}

func (bc *bubbleComponent) setQuoteObj(obj *quoter) {
	bc.quoteObj = obj
}

func (bc *bubbleComponent) hasQuote() bool {
	return bc.quoteObj != nil
}
