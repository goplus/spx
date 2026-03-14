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
// Component System - Base Interfaces and Structures
// ============================================================================
// This file defines the component-based architecture for sprites.
// Each component encapsulates a specific aspect of sprite functionality.

// component is the base interface for all sprite components
type component interface {
	// initialize is called when the component is first created
	// spriteCfg can be nil when cloning
	initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig)

	// cloneFrom creates a new component instance by cloning from source
	cloneFrom(src component, newSprite *SpriteImpl) component

	// onDestroy is called when the sprite is being destroyed
	onDestroy()
}

// componentBase provides default implementations for component interface
type componentBase struct {
	sprite *SpriteImpl
}

func (c *componentBase) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	c.sprite = sprite
}

func (c *componentBase) onDestroy() {
	// Default: do nothing
}

// ============================================================================
// Component Registry
// ============================================================================

// spriteComponents holds all components attached to a sprite
type spriteComponents struct {
	transform *transformComponent
	animation *animationComponent
	physics   *physicsComponent
	pen       *penComponent
	sound     *soundComponent
	bubble    *bubbleComponent // Optional: only allocated when Say/Think/Quote is used
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

// cloneFrom creates new component instances by cloning from source components
// This is used when cloning a sprite to ensure each sprite has independent components
// Each component's cloneFrom method is responsible for its own cloning logic
func (sc *spriteComponents) cloneFrom(src *spriteComponents, newSprite *SpriteImpl) {
	// Delegate cloning to each component
	sc.transform = src.transform.cloneFrom(src.transform, newSprite).(*transformComponent)
	sc.animation = src.animation.cloneFrom(src.animation, newSprite).(*animationComponent)
	sc.physics = src.physics.cloneFrom(src.physics, newSprite).(*physicsComponent)
	sc.pen = src.pen.cloneFrom(src.pen, newSprite).(*penComponent)
	sc.sound = src.sound.cloneFrom(src.sound, newSprite).(*soundComponent)
	// Bubble component is optional and NOT cloned - each sprite starts fresh
	sc.bubble = nil
}

// destroyComponents destroys all sprite components
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

// Transform returns the transform component
func (sc *spriteComponents) Transform() *transformComponent {
	return sc.transform
}

// Animation returns the animation component
func (sc *spriteComponents) Animation() *animationComponent {
	return sc.animation
}

// Physics returns the physics component
func (sc *spriteComponents) Physics() *physicsComponent {
	return sc.physics
}

// Pen returns the pen component
func (sc *spriteComponents) Pen() *penComponent {
	return sc.pen
}

// Sound returns the sound component
func (sc *spriteComponents) Sound() *soundComponent {
	return sc.sound
}

// Bubble returns the bubble component (lazy initialization)
func (sc *spriteComponents) Bubble() *bubbleComponent {
	if sc.bubble == nil {
		sc.bubble = &bubbleComponent{}
		sc.bubble.initialize(sc.transform.sprite, nil)
	}
	return sc.bubble
}
