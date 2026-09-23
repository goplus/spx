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
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	spxlog "github.com/goplus/spx/v3/internal/log"
	engine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

// ============================================================================
// Transform Component
// ============================================================================
// This component manages sprite position, rotation, and scale state.

const (
	// minSpeed is the minimum allowed speed value to prevent division by zero.
	minSpeed = 0.001

	// missingTargetDistance is Scratch's sentinel for an unresolved target.
	missingTargetDistance = 10_000

	// minBounceComponent prevents sprites from getting stuck at boundaries.
	minBounceComponent = 0.2

	// fullCircleDegrees represents a complete rotation in degrees.
	fullCircleDegrees = 360.0

	// halfCircleDegrees represents half a rotation in degrees.
	halfCircleDegrees = 180.0

	// fenceWidth matches Scratch's keepInFence margin.
	fenceWidth = 15.0

	// minVisibleSize matches Scratch's minimum rendered sprite size in pixels.
	minVisibleSize = 5.0

	// maxStageScale matches Scratch's 150% stage-coverage cap.
	maxStageScale = 1.5
)

// transformComponent encapsulates all position, rotation, and scale functionality
// for sprites in the game world.
type transformComponent struct {
	sprite *SpriteImpl

	// Position coordinates in world space.
	x, y float64

	// Rotation properties.
	direction     float64
	rotationStyle RotationStyle

	// Transform origin.
	pivot mathf.Vec2
}

// ============================================================================
// Lifecycle
// ============================================================================

// initialize initializes the transform component from configuration.
func (t *transformComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	t.sprite = sprite

	t.x = spriteCfg.X
	t.y = spriteCfg.Y
	t.direction = spriteCfg.Heading
	t.rotationStyle = toRotationStyle(spriteCfg.RotationStyle)
	t.pivot = spriteCfg.Pivot
}

// cloneFor creates a new transform component for newSprite.
func (t *transformComponent) cloneFor(newSprite *SpriteImpl) *transformComponent {
	return &transformComponent{
		sprite:        newSprite,
		x:             t.x,
		y:             t.y,
		direction:     t.direction,
		rotationStyle: t.rotationStyle,
		pivot:         t.pivot,
	}
}

// ============================================================================
// Transform State
// ============================================================================

// markDirty publishes a transform change to the sprite proxy.
func (t *transformComponent) markDirty() {
	t.sprite.markProxyDirty()
}

// getPivot returns the current pivot point.
func (t *transformComponent) getPivot() mathf.Vec2 {
	return t.pivot
}

// ============================================================================
// Position and Movement
// ============================================================================

func (t *transformComponent) moveForward(step float64) {
	sin, cos := math.Sincos(toRadian(t.direction))
	t.moveTo(t.x+step*sin, t.y+step*cos)
}

func (t *transformComponent) glide(x, y float64, secs float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Glide: sprite=%s, x=%v, y=%v, secs=%v", t.sprite.name, x, y, secs)
	}

	x0, y0 := t.getXY()
	animName := t.sprite.getStateAnimName(StateGlide)
	t.sprite.animation().doTween(animName, nil, tweenParams{
		aniType:  coreproject.AniTypeGlide,
		duration: secs,
		moveFrom: mathf.NewVec2(x0, y0),
		moveTo:   mathf.NewVec2(x, y),
	})
}

func (t *transformComponent) glideToTarget(target Target, secs float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Glide: target=%v, secs=%v", target, secs)
	}
	x, y, ok := t.sprite.g.resolveTargetPosition(target)
	if !ok {
		return
	}
	t.glide(x, y, secs)
}

func (t *transformComponent) step(step, speed float64, animation SpriteAnimationName) {
	dirSin, dirCos := math.Sincos(toRadian(t.direction))
	diff := mathf.NewVec2(step*dirSin, step*dirCos)
	to := mathf.NewVec2(t.x, t.y).Add(diff)
	t.stepToPos(to.X, to.Y, speed, animation)
}

func (t *transformComponent) stepToPos(x, y, speed float64, animation SpriteAnimationName) {
	if animation == "" {
		animation = t.sprite.getStateAnimName(StateStep)
	}

	from := mathf.NewVec2(t.x, t.y)
	to := mathf.NewVec2(x, y)
	distance := from.DistanceTo(to)

	ani, ok := t.sprite.getAnimation(animation)
	if !ok {
		t.setPosition(x, y)
		return
	}

	speed = math.Max(speed, minSpeed)
	duration := math.Abs(distance) * ani.StepDuration / speed
	t.sprite.animation().doTween(animation, ani, tweenParams{
		aniType:  coreproject.AniTypeMove,
		duration: duration,
		speed:    speed,
		moveFrom: from,
		moveTo:   to,
	})
}

func (t *transformComponent) stepToTarget(target Target, speed float64, animation SpriteAnimationName) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Goto: sprite=%s, target=%v", t.sprite.name, target)
	}
	x, y, ok := t.sprite.g.resolveTargetPosition(target)
	if !ok {
		return
	}
	t.stepToPos(x, y, speed, animation)
}

func (t *transformComponent) setSize(size float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetSize: sprite=%s, size=%v", t.sprite.name, size)
	}
	if size < 0 {
		spxlog.Warn("SetSize: sprite=%s, size=%v is negative, clamping to 0", t.sprite.name, size)
		size = 0
	}
	size = t.clampSpriteScale(size)

	t.sprite.runtimeState.Scale = size
	t.sprite.runtimeState.IsCostumeDirty = true
	t.markDirty()
	t.sprite.updatePhysicsShapesScale()
}

func (t *transformComponent) changeSize(delta float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("ChangeSize: sprite=%s, delta=%v", t.sprite.name, delta)
	}
	t.setSize(t.sprite.runtimeState.Scale + delta)
}

func (t *transformComponent) getXY() (x, y float64) {
	return t.x, t.y
}

func (t *transformComponent) getX() float64 {
	return t.x
}

func (t *transformComponent) getY() float64 {
	return t.y
}

func (t *transformComponent) setPosition(x, y float64) {
	t.moveTo(x, y)
}

func (t *transformComponent) setX(x float64) {
	t.moveTo(x, t.y)
}

func (t *transformComponent) setY(y float64) {
	t.moveTo(t.x, y)
}

func (t *transformComponent) changePosition(dx, dy float64) {
	t.moveTo(t.x+dx, t.y+dy)
}

func (t *transformComponent) changeX(dx float64) {
	t.moveTo(t.x+dx, t.y)
}

func (t *transformComponent) changeY(dy float64) {
	t.moveTo(t.x, t.y+dy)
}

func (t *transformComponent) distanceToTarget(target Target) float64 {
	x, y := t.x, t.y
	x2, y2, ok := t.sprite.g.resolveTargetPosition(target)
	if !ok {
		return missingTargetDistance
	}
	dx := x - x2
	dy := y - y2
	return math.Sqrt(dx*dx + dy*dy)
}

// ============================================================================
// Position Helpers
// ============================================================================

// clampSpriteScale follows Scratch's sprite size limits.
func (t *transformComponent) clampSpriteScale(size float64) float64 {
	if len(t.sprite.costumes) == 0 || t.sprite.costumeIndex < 0 || t.sprite.costumeIndex >= len(t.sprite.costumes) {
		return size
	}

	costumeWidth, costumeHeight := t.sprite.currentCostume().getSizeF()
	if costumeWidth <= 0 || costumeHeight <= 0 {
		return size
	}

	worldWidth, worldHeight := t.sprite.g.displayState.WorldWidth, t.sprite.g.displayState.WorldHeight
	if worldWidth <= 0 || worldHeight <= 0 {
		worldWidth, worldHeight = baseScreenWidth, baseScreenHeight
	}

	minScale := math.Min(
		1.0,
		math.Max(
			minVisibleSize/costumeWidth,
			minVisibleSize/costumeHeight,
		),
	)
	maxScale := math.Min(
		(maxStageScale*float64(worldWidth))/costumeWidth,
		(maxStageScale*float64(worldHeight))/costumeHeight,
	)
	if maxScale < minScale {
		maxScale = minScale
	}

	return mathf.Clamp(size, minScale, maxScale)
}

// setPositionRaw updates position without movement side effects and reports whether it changed.
func (t *transformComponent) setPositionRaw(x, y float64) bool {
	if t.x == x && t.y == y {
		return false
	}
	t.x, t.y = x, y
	return true
}

// moveTo moves the sprite to the specified position, handling pen movement
// and transform updates.
func (t *transformComponent) moveTo(x, y float64) {
	x, y = t.fixWorldRange(x, y)
	t.sprite.movePen(x, y)
	t.x, t.y = x, y
	t.markDirty()
}

// fixWorldRange mirrors Scratch's keepInFence behavior: a sprite may extend
// beyond the stage as long as at least a small fenced slice remains visible.
func (t *transformComponent) fixWorldRange(x, y float64) (float64, float64) {
	rect := t.sprite.fenceBounds()
	if rect == nil {
		return x, y
	}
	if t.sprite.g.displayState.WorldWidth <= 0 || t.sprite.g.displayState.WorldHeight <= 0 {
		return x, y
	}

	worldLeft, worldTop, worldRight, worldBottom := t.sprite.g.worldBounds()
	dx := x - t.x
	dy := y - t.y
	left := rect.Position.X
	right := rect.Position.X + rect.Size.X
	bottom := rect.Position.Y
	top := rect.Position.Y + rect.Size.Y

	fenceInset := math.Min(fenceWidth, math.Floor(math.Min(rect.Size.X, rect.Size.Y)/2))

	minX := float64(worldLeft) + fenceInset
	maxX := float64(worldRight) - fenceInset
	if right+dx < minX {
		// Scratch snaps a fenced coordinate inward to an integer.
		x = math.Ceil(t.x + (minX - right))
	} else if left+dx > maxX {
		x = math.Floor(t.x + (maxX - left))
	}

	minY := float64(worldBottom) + fenceInset
	maxY := float64(worldTop) - fenceInset
	if top+dy < minY {
		y = math.Ceil(t.y + (minY - top))
	} else if bottom+dy > maxY {
		y = math.Floor(t.y + (maxY - bottom))
	}

	return x, y
}

// ============================================================================
// Rotation
// ============================================================================

func (t *transformComponent) heading() Direction {
	return t.direction
}

func (t *transformComponent) setHeading(dir Direction) bool {
	return t.applyDirection(dir)
}

func (t *transformComponent) changeHeading(delta Direction) bool {
	return t.applyDirection(t.direction + delta)
}

func (t *transformComponent) setRotationStyle(style RotationStyle) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetRotationStyle: sprite=%s, style=%v", t.sprite.name, style)
	}
	t.rotationStyle = style
	t.markDirty()
}

func (t *transformComponent) turn(delta Direction, speed float64, animation SpriteAnimationName) {
	from := t.direction
	to := t.direction + delta
	t.doTurnAnimation(from, to, speed, animation, func() {
		if t.changeHeading(delta) && isDebugInstrEnabled() {
			spxlog.Debug("Turn: sprite=%s, delta=%v", t.sprite.name, delta)
		}
	})
}

func (t *transformComponent) turnTo(target any, speed float64, animation SpriteAnimationName) {
	targetDirection, ok := t.resolveTargetDirection(target)
	if !ok {
		return
	}
	fromAngle, toAngle := t.normalizeAngleRange(t.direction, targetDirection)

	t.doTurnAnimation(fromAngle, toAngle, speed, animation, func() {
		if t.applyDirection(targetDirection) && isDebugInstrEnabled() {
			spxlog.Debug("TurnTo: sprite=%s, target=%v", t.sprite.name, target)
		}
	})
}

func (t *transformComponent) turnToPos(x, y, speed float64, animation SpriteAnimationName) {
	targetAngle := t.calculateTargetAngleToPos(x, y)
	fromAngle, toAngle := t.normalizeAngleRange(t.direction, targetAngle)

	t.doTurnAnimation(fromAngle, toAngle, speed, animation, func() {
		if t.applyDirection(targetAngle) && isDebugInstrEnabled() {
			spxlog.Debug("TurnToPos: sprite=%s, x=%v, y=%v", t.sprite.name, x, y)
		}
	})
}

func (t *transformComponent) bounceOffEdge(area string) {
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

	t.moveTo(t.x, t.y)
}

// ============================================================================
// Rotation Helpers
// ============================================================================

// applyDirection normalizes the direction and reports whether it changed.
func (t *transformComponent) applyDirection(dir float64) bool {
	dir = normalizeDirection(dir)
	if t.direction == dir {
		return false
	}

	t.direction = dir
	t.markDirty()
	return true
}

func (t *transformComponent) directionToTarget(target Target) Direction {
	direction, ok := t.resolveTargetDirection(target)
	if !ok {
		return t.direction
	}
	return normalizeDirection(direction)
}

func (t *transformComponent) directionToPos(x, y float64) Direction {
	return normalizeDirection(t.calculateTargetAngleToPos(x, y))
}

func (t *transformComponent) resolveTargetDirection(target any) (Direction, bool) {
	switch target := target.(type) {
	case Direction:
		return target, true
	default:
		x, y, ok := t.sprite.g.resolveTargetPosition(target)
		if !ok {
			return 0, false
		}
		return t.calculateTargetAngleToPos(x, y), true
	}
}

func (t *transformComponent) calculateTargetAngleToPos(x, y float64) float64 {
	if t.x == x && t.y == y {
		return t.direction
	}
	return engine.HeadingToPoint(mathf.NewVec2(t.x, t.y), mathf.NewVec2(x, y))
}

// normalizeAngleRange chooses equivalent angles with the shortest rotation path.
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

// doTurnAnimation runs a turn tween and falls back to a direct heading change.
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
	speed = math.Max(speed, minSpeed)
	duration := ani.TurnToDuration / fullCircleDegrees * absDelta / speed
	t.sprite.animation().doTween(animation, ani, tweenParams{
		aniType:  coreproject.AniTypeTurn,
		duration: duration,
		speed:    speed,
		turnFrom: from,
		turnTo:   to,
	})
}

// ============================================================================
// Transform Helpers
// ============================================================================

func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right", "leftRight":
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
