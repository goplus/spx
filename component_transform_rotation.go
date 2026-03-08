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
	"math"

	spxlog "github.com/goplus/spx/v2/internal/log"
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// Heading returns the current direction the sprite is facing.
func (t *transformComponent) Heading() Direction {
	return t.direction
}

// SetHeading sets the sprite's direction to the specified value.
func (t *transformComponent) SetHeading(dir Direction) bool {
	return t.applyDirection(dir)
}

// ChangeHeading changes the sprite's direction by the specified delta.
func (t *transformComponent) ChangeHeading(delta Direction) bool {
	return t.applyDirection(t.direction + delta)
}

// SetRotationStyle sets the rotation style for the sprite.
func (t *transformComponent) SetRotationStyle(style RotationStyle) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetRotationStyle: sprite=%s, style=%v", t.sprite.name, style)
	}
	t.rotationStyle = style
}

// Turn rotates the sprite by the specified angle using an animation.
func (t *transformComponent) Turn(delta Direction, speed float64, animation SpriteAnimationName) {
	from := t.direction
	to := t.direction + delta
	t.doTurnAnimation(from, to, speed, animation, func() {
		if t.ChangeHeading(delta) && isDebugInstrEnabled() {
			spxlog.Debug("Turn: sprite=%s, delta=%v", t.sprite.name, delta)
		}
	})
}

// TurnTo turns the sprite to face the specified object or direction using an animation.
func (t *transformComponent) TurnTo(obj any, speed float64, animation SpriteAnimationName) {
	targetAngle := t.calculateTargetAngle(obj)
	fromAngle, toAngle := t.normalizeAngleRange(t.direction, targetAngle)

	t.doTurnAnimation(fromAngle, toAngle, speed, animation, func() {
		if t.applyDirection(targetAngle) && isDebugInstrEnabled() {
			spxlog.Debug("TurnTo: sprite=%s, obj=%v", t.sprite.name, obj)
		}
	})
}

// BounceOffEdge bounces the sprite off the edge of the stage by reflecting
// its direction vector based on the edge that was touched.
func (t *transformComponent) BounceOffEdge() {
	if isDebugInstrEnabled() {
		spxlog.Debug("BounceOffEdge: %s", t.sprite.name)
	}

	nearestEdge := t.sprite.checkNearestTouchedBoundary()
	if nearestEdge == 0 {
		return
	}

	radians := toRadian(90 - t.direction)
	dx := math.Cos(radians)
	dy := -math.Sin(radians)

	dx, dy = t.calculateBounceDirection(nearestEdge, dx, dy)

	newDirection := engine.RadToDeg(math.Atan2(dy, dx)) + 90
	t.direction = normalizeDirection(newDirection)
}

// applyDirection sets the sprite's direction to the normalized value.
// Returns true if the direction was actually changed.
func (t *transformComponent) applyDirection(dir float64) bool {
	dir = normalizeDirection(dir)
	if t.direction == dir {
		return false
	}

	t.direction = dir
	t.markDirty()
	return true
}

// calculateTargetAngle calculates the angle to turn toward the specified object.
func (t *transformComponent) calculateTargetAngle(obj any) float64 {
	switch v := obj.(type) {
	case Direction:
		return v
	default:
		x, y := t.sprite.g.objectPos(obj)
		dx := x - t.x
		dy := y - t.y
		return 90 - engine.RadToDeg(math.Atan2(dy, dx))
	}
}

// normalizeAngleRange normalizes two angles to minimize the rotation distance.
// This ensures the sprite takes the shortest path when rotating.
func (t *transformComponent) normalizeAngleRange(from, to float64) (float64, float64) {
	fromNorm := math.Mod(from+fullCircleDegrees, fullCircleDegrees)
	toNorm := math.Mod(to+fullCircleDegrees, fullCircleDegrees)

	if toNorm-fromNorm > halfCircleDegrees {
		fromNorm += fullCircleDegrees
	} else if fromNorm-toNorm > halfCircleDegrees {
		toNorm += fullCircleDegrees
	}

	return fromNorm, toNorm
}

// calculateBounceDirection calculates the new direction vector after bouncing
// off the specified edge.
func (t *transformComponent) calculateBounceDirection(edge int, dx, dy float64) (float64, float64) {
	switch edge {
	case touchingScreenLeft:
		dx = math.Max(minBounceComponent, math.Abs(dx))
	case touchingScreenTop:
		dy = math.Max(minBounceComponent, math.Abs(dy))
	case touchingScreenRight:
		dx = -math.Max(minBounceComponent, math.Abs(dx))
	case touchingScreenBottom:
		dy = -math.Max(minBounceComponent, math.Abs(dy))
	}
	return dx, dy
}
