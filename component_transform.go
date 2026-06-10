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

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	spxlog "github.com/goplus/spx/v2/internal/log"
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// ============================================================================
// Transform Component
// ============================================================================
// This component manages sprite position, rotation, and scale state.

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

// ============================================================================
// Transform Helpers
// ============================================================================

func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right":
		return LeftRight
	case "none":
		return None
	case "normal":
		return Normal
	default:
		spxlog.Warn("Unrecognized rotationStyle value '%s', using default 'Normal'.", style)
		return Normal
	}
}

// toRadian converts degrees to radians.
func toRadian(dir float64) float64 {
	return math.Pi * dir / 180
}

// normalizeDirection normalizes a direction angle to the range (-180, 180].
func normalizeDirection(dir float64) float64 {
	if dir <= -180 {
		dir += 360
	} else if dir > 180 {
		dir -= 360
	}
	return dir
}

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

// ============================================================================
// Lifecycle
// ============================================================================

// initialize initializes the transform component from configuration.
func (t *transformComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
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
	t.sprite.markProxyDirty()
}

// getPivot returns the current pivot point.
func (t *transformComponent) getPivot() mathf.Vec2 {
	return t.pivot
}

// ============================================================================
// Position and Movement
// ============================================================================

// MoveForward moves the sprite forward by the specified step size
// in the direction it's currently facing.
func (t *transformComponent) MoveForward(step float64) {
	sin, cos := math.Sincos(toRadian(t.direction))
	t.moveTo(t.x+step*sin, t.y+step*cos)
}

// Glide moves the sprite smoothly to the specified position over the given duration.
func (t *transformComponent) Glide(x, y float64, secs float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Glide: sprite=%s, x=%v, y=%v, secs=%v", t.sprite.name, x, y, secs)
	}

	x0, y0 := t.XY()
	from := mathf.NewVec2(x0, y0)
	to := mathf.NewVec2(x, y)

	aniCopy := coreproject.AniConfig{
		Duration: secs,
		From:     &from,
		To:       &to,
		AniType:  coreproject.AniTypeGlide,
		IsLoop:   true,
	}

	animName := t.sprite.getStateAnimName(StateGlide)
	t.sprite.animation().doTween(animName, &aniCopy)
}

// GlideTo moves the sprite smoothly to the specified object's position
// over the given duration.
func (t *transformComponent) GlideTo(obj any, secs float64) {
	x, y := t.sprite.g.objectPos(obj)
	t.Glide(x, y, secs)
}

// Step moves the sprite forward by the specified step size using a stepping animation.
func (t *transformComponent) Step(step, speed float64, animation SpriteAnimationName) {
	dirSin, dirCos := math.Sincos(toRadian(t.direction))
	diff := mathf.NewVec2(step*dirSin, step*dirCos)
	to := mathf.NewVec2(t.x, t.y).Add(diff)
	t.StepToPos(to.X, to.Y, speed, animation)
}

// StepToPos moves the sprite to the specified position using a stepping animation.
func (t *transformComponent) StepToPos(x, y, speed float64, animation SpriteAnimationName) {
	if animation == "" {
		animation = t.sprite.getStateAnimName(StateStep)
	}

	// If no animation exists, move to target immediately.
	if !t.sprite.hasAnim(animation) {
		t.SetXYpos(x, y)
		return
	}

	from := mathf.NewVec2(t.x, t.y)
	to := mathf.NewVec2(x, y)
	distance := from.DistanceTo(to)

	ani, ok := t.sprite.getAnimation(animation)
	if !ok {
		return
	}

	duration := math.Abs(distance) * ani.StepDuration / math.Max(speed, minSpeed)
	t.doAnimatedTween(animation, ani, &from, &to, coreproject.AniTypeMove, duration, speed)
}

// StepTo moves the sprite to the specified object's position using a stepping animation.
func (t *transformComponent) StepTo(obj any, speed float64, animation SpriteAnimationName) {
	x, y := t.sprite.g.objectPos(obj)
	t.StepToPos(x, y, speed, animation)
}

// SetSize sets the sprite's scale to the specified size.
func (t *transformComponent) SetSize(size float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetSize: sprite=%s, size=%v", t.sprite.name, size)
	}
	if size < 0 {
		spxlog.Warn("SetSize: sprite=%s, size=%v is negative, clamping to 0", t.sprite.name, size)
		size = 0
	}

	t.sprite.runtimeState.Scale = size
	t.sprite.runtimeState.IsCostumeDirty = true
	t.markDirty()
	t.sprite.updatePhysicsShapesScale()
}

// ChangeSize changes the sprite's scale by the specified delta.
func (t *transformComponent) ChangeSize(delta float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("ChangeSize: sprite=%s, delta=%v", t.sprite.name, delta)
	}
	t.SetSize(t.sprite.runtimeState.Scale + delta)
}

// XY returns the current position coordinates.
func (t *transformComponent) XY() (x, y float64) {
	return t.x, t.y
}

// Xpos returns the current x-coordinate.
func (t *transformComponent) Xpos() float64 {
	return t.x
}

// Ypos returns the current y-coordinate.
func (t *transformComponent) Ypos() float64 {
	return t.y
}

// SetXYpos sets both position coordinates to the specified values.
func (t *transformComponent) SetXYpos(x, y float64) {
	t.moveTo(x, y)
}

// SetXpos sets the x-coordinate to the specified value.
func (t *transformComponent) SetXpos(x float64) {
	t.moveTo(x, t.y)
}

// SetYpos sets the y-coordinate to the specified value.
func (t *transformComponent) SetYpos(y float64) {
	t.moveTo(t.x, y)
}

// ChangeXYpos changes both position coordinates by the specified deltas.
func (t *transformComponent) ChangeXYpos(dx, dy float64) {
	t.moveTo(t.x+dx, t.y+dy)
}

// ChangeXpos changes the x-coordinate by the specified delta.
func (t *transformComponent) ChangeXpos(dx float64) {
	t.moveTo(t.x+dx, t.y)
}

// ChangeYpos changes the y-coordinate by the specified delta.
func (t *transformComponent) ChangeYpos(dy float64) {
	t.moveTo(t.x, t.y+dy)
}

// DistanceTo calculates the Euclidean distance from this sprite to the specified object.
func (t *transformComponent) DistanceTo(obj any) float64 {
	x, y := t.x, t.y
	x2, y2 := t.sprite.g.objectPos(obj)
	dx := x - x2
	dy := y - y2
	return math.Sqrt(dx*dx + dy*dy)
}

// setXY sets position directly without triggering side effects.
// This is used for internal operations that need to bypass normal movement logic.
func (t *transformComponent) setXY(x, y float64) {
	t.x, t.y = x, y
}

// moveTo moves the sprite to the specified position, handling pen movement
// and transform updates.
func (t *transformComponent) moveTo(x, y float64) {
	x, y = t.fixWorldRange(x, y)
	t.sprite.movePen(x, y)
	t.x, t.y = x, y
	t.markDirty()
}

// fixWorldRange clamps sprite position within world boundaries.
func (t *transformComponent) fixWorldRange(x, y float64) (float64, float64) {
	rect := t.sprite.bounds()
	if rect == nil {
		return x, y
	}

	worldLeft, worldTop, worldRight, worldBottom := t.sprite.g.worldBounds()

	leftOffset := t.x - rect.Position.X
	rightOffset := rect.Position.X + rect.Size.X - t.x
	bottomOffset := t.y - rect.Position.Y
	topOffset := rect.Position.Y + rect.Size.Y - t.y

	x = mathf.Clamp(x, float64(worldLeft)-rightOffset, float64(worldRight)+leftOffset)
	y = mathf.Clamp(y, float64(worldBottom)-topOffset, float64(worldTop)+bottomOffset)

	return x, y
}

// ============================================================================
// Rotation
// ============================================================================

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

// TurnToPos turns the sprite to face the specified position using an animation.
func (t *transformComponent) TurnToPos(x, y, speed float64, animation SpriteAnimationName) {
	targetAngle := t.calculateTargetAngleToPos(x, y)
	fromAngle, toAngle := t.normalizeAngleRange(t.direction, targetAngle)

	t.doTurnAnimation(fromAngle, toAngle, speed, animation, func() {
		if t.applyDirection(targetAngle) && isDebugInstrEnabled() {
			spxlog.Debug("TurnToPos: sprite=%s, x=%v, y=%v", t.sprite.name, x, y)
		}
	})
}

// BounceOffEdge bounces the sprite off the selected edge area by reflecting
// its direction vector based on the edge that was touched.
func (t *transformComponent) BounceOffEdge(area string) {
	if isDebugInstrEnabled() {
		spxlog.Debug("BounceOffEdge: %s", t.sprite.name)
	}

	nearestEdge := t.sprite.checkNearestTouchedBoundary(area)
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

// ============================================================================
// Rotation Helpers
// ============================================================================

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

// DirectionTo returns the normalized heading needed to face the specified object.
func (t *transformComponent) DirectionTo(obj any) Direction {
	return normalizeDirection(t.calculateTargetAngle(obj))
}

// DirectionToPos returns the normalized heading needed to face the specified position.
func (t *transformComponent) DirectionToPos(x, y float64) Direction {
	return normalizeDirection(t.calculateTargetAngleToPos(x, y))
}

// calculateTargetAngle calculates the angle to turn toward the specified object.
func (t *transformComponent) calculateTargetAngle(obj any) float64 {
	switch v := obj.(type) {
	case Direction:
		return v
	default:
		x, y := t.sprite.g.objectPos(obj)
		return t.calculateTargetAngleToPos(x, y)
	}
}

func (t *transformComponent) calculateTargetAngleToPos(x, y float64) float64 {
	if t.x == x && t.y == y {
		return t.direction
	}
	return engine.HeadingToPoint(mathf.NewVec2(t.x, t.y), mathf.NewVec2(x, y))
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

// ============================================================================
// Tween Helpers
// ============================================================================

// doTurnAnimation handles turn animation with fallback to direct heading change.
// This helper eliminates code duplication between Turn and TurnTo methods.
func (t *transformComponent) doTurnAnimation(
	from, to float64,
	speed float64,
	animation SpriteAnimationName,
	fallback func(),
) {
	if animation == "" {
		animation = t.sprite.getStateAnimName(StateTurn)
	}

	ani, ok := t.sprite.getAnimation(animation)
	if !ok {
		fallback()
		return
	}

	absDelta := math.Abs(from - to)
	duration := ani.TurnToDuration / fullCircleDegrees * absDelta / math.Max(speed, minSpeed)
	t.doAnimatedTween(animation, ani, from, to, coreproject.AniTypeTurn, duration, speed)
}

// doAnimatedTween creates and executes a tween animation with the specified parameters.
// This helper eliminates code duplication in animation methods.
func (t *transformComponent) doAnimatedTween(
	name SpriteAnimationName,
	base *coreproject.AniConfig,
	from, to any,
	aniType coreproject.AniType,
	duration float64,
	speed float64,
) {
	speed = math.Max(speed, minSpeed)

	aniCopy := *base
	aniCopy.From = from
	aniCopy.To = to
	aniCopy.AniType = aniType
	aniCopy.Duration = duration
	aniCopy.IsLoop = true
	aniCopy.Speed = speed

	t.sprite.doTween(name, &aniCopy)
}
