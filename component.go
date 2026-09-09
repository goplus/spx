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

import coreproject "github.com/goplus/spx/v3/internal/core/project"

// ============================================================================
// Component System
// ============================================================================
// This file defines the component-based architecture for sprites.
// Each component encapsulates a specific aspect of sprite functionality.

// component is the base interface for all sprite components.
type component interface {
	// initialize is called when the component is first created.
	// spriteCfg can be nil when cloning.
	initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig)

	// cloneFrom creates a new component instance by cloning from source.
	cloneFrom(src component, newSprite *SpriteImpl) component

	// onDestroy is called when the sprite is being destroyed.
	onDestroy()
}

// componentBase provides default implementations for the component interface.
type componentBase struct {
	sprite *SpriteImpl
}

// ============================================================================
// Component Registry
// ============================================================================

// spriteComponents holds all components attached to a sprite.
type spriteComponents struct {
	transform *transformComponent
	animation *animationComponent
	physics   *physicsComponent
	pen       *penComponent
	sound     *soundComponent
	bubble    *bubbleComponent // Optional: only allocated when Say/Think/Quote is used
}

func (c *componentBase) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	c.sprite = sprite
}

// initComponents initializes all sprite components.
func (sc *spriteComponents) initComponents(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	sc.transform = &transformComponent{}
	sc.transform.initialize(sprite, spriteCfg)

	sc.animation = &animationComponent{}
	sc.animation.initialize(sprite, spriteCfg)

	sc.physics = &physicsComponent{}
	sc.physics.initialize(sprite, spriteCfg)

	sc.pen = &penComponent{}
	sc.pen.initialize(sprite, spriteCfg)

	sc.sound = &soundComponent{}
	sc.sound.initialize(sprite, spriteCfg)
}

// cloneFrom creates independent component instances for a cloned sprite.
func (sc *spriteComponents) cloneFrom(src *spriteComponents, newSprite *SpriteImpl) {
	sc.transform = src.transform.cloneFrom(src.transform, newSprite).(*transformComponent)
	sc.animation = src.animation.cloneFrom(src.animation, newSprite).(*animationComponent)
	sc.physics = src.physics.cloneFrom(src.physics, newSprite).(*physicsComponent)
	sc.pen = src.pen.cloneFrom(src.pen, newSprite).(*penComponent)
	sc.sound = src.sound.cloneFrom(src.sound, newSprite).(*soundComponent)
	sc.bubble = nil
}

// destroyComponents destroys all sprite components.
func (sc *spriteComponents) destroyComponents() {
	if sc.transform != nil {
		sc.transform.onDestroy()
	}
	if sc.animation != nil {
		sc.animation.onDestroy()
	}
	if sc.physics != nil {
		sc.physics.onDestroy()
	}
	if sc.pen != nil {
		sc.pen.onDestroy()
	}
	if sc.sound != nil {
		sc.sound.onDestroy()
	}
	if sc.bubble != nil {
		sc.bubble.onDestroy()
	}
}

// ============================================================================
// Component Accessors
// ============================================================================

func (sc *spriteComponents) getTransform() *transformComponent {
	return sc.transform
}

func (sc *spriteComponents) getAnimation() *animationComponent {
	return sc.animation
}

func (sc *spriteComponents) getPhysics() *physicsComponent {
	return sc.physics
}

func (sc *spriteComponents) getPen() *penComponent {
	return sc.pen
}

func (sc *spriteComponents) getSound() *soundComponent {
	return sc.sound
}

func (sc *spriteComponents) getBubble() *bubbleComponent {
	if sc.bubble == nil {
		sc.bubble = &bubbleComponent{}
		sc.bubble.initialize(sc.transform.sprite, nil)
	}
	return sc.bubble
}
