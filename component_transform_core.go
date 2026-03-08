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

import "github.com/goplus/spbase/mathf"

const (
	// minSpeed is the minimum allowed speed value to prevent division by zero.
	minSpeed = 0.001

	// minBounceComponent prevents sprites from getting stuck at boundaries.
	minBounceComponent = 0.2

	// fullCircleDegrees represents a complete rotation in degrees.
	fullCircleDegrees = 360.0

	// halfCircleDegrees represents half a rotation in degrees.
	halfCircleDegrees = 180.0
)

// transformComponent encapsulates all position, rotation, and scale functionality
// for sprites in the game world.
type transformComponent struct {
	componentBase

	// Position coordinates in world space
	x, y float64

	// Rotation properties
	direction     float64
	rotationStyle RotationStyle

	// Transform origin
	pivot mathf.Vec2

	// State
	isDirty bool
}

// initialize initializes the transform component from configuration.
func (t *transformComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	t.componentBase.initialize(sprite, spriteCfg)

	t.x = spriteCfg.X
	t.y = spriteCfg.Y
	t.direction = spriteCfg.Heading
	t.rotationStyle = toRotationStyle(spriteCfg.RotationStyle)
	t.pivot = spriteCfg.Pivot
	t.isDirty = false
}

// cloneFrom creates a new transform component by cloning from source.
func (t *transformComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcTransform := src.(*transformComponent)
	return &transformComponent{
		componentBase: componentBase{sprite: newSprite},
		x:             srcTransform.x,
		y:             srcTransform.y,
		direction:     srcTransform.direction,
		rotationStyle: srcTransform.rotationStyle,
		pivot:         srcTransform.pivot,
		isDirty:       false,
	}
}

// onDestroy performs cleanup when the component is destroyed.
func (t *transformComponent) onDestroy() {
	// No resources to cleanup
}

// markDirty marks the transform as dirty, triggering an update.
func (t *transformComponent) markDirty() {
	t.isDirty = true
	t.sprite.IsDirty = true
}

// getPivot returns the current pivot point.
func (t *transformComponent) getPivot() mathf.Vec2 {
	return t.pivot
}
