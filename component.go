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

// initComponents initializes all sprite components.
func (sc *spriteComponents) initComponents(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	sc.transform = &transformComponent{}
	sc.transform.initialize(sprite, spriteCfg)

	sc.animation = &animationComponent{}
	sc.animation.initialize(sprite, spriteCfg)

	sc.physics = &physicsComponent{}
	sc.physics.initialize(sprite, spriteCfg)

	sc.pen = &penComponent{}
	sc.pen.initialize(sprite)

	sc.sound = &soundComponent{sprite: sprite}
}

// cloneComponents creates independent eager components for a cloned sprite.
// Bubble state is intentionally omitted and remains lazy.
func cloneComponents(source *SpriteImpl, newSprite *SpriteImpl) spriteComponents {
	// Pen and sound inherit the original instance's current state across generations.
	original := source.originalSprite()
	return spriteComponents{
		transform: source.components.transform.cloneFor(newSprite),
		animation: source.components.animation.cloneFor(newSprite),
		physics:   source.components.physics.cloneFor(newSprite),
		pen:       original.components.pen.cloneFor(newSprite),
		sound:     original.components.sound.cloneFor(newSprite),
	}
}

// destroyComponents destroys all sprite components.
func (sc *spriteComponents) destroyComponents() {
	if sc.animation != nil {
		sc.animation.onDestroy()
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
