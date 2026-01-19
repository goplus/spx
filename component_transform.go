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
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// ============================================================================
// Transform Component
// ============================================================================
// This component encapsulates all position, rotation, and scale functionality

type transformComponent struct {
	componentBase

	// Position
	x, y float64

	// Rotation
	direction     float64
	rotationStyle RotationStyle

	// Scale
	scale float64

	// Pivot point
	pivot mathf.Vec2

	// Dirty flag for transform updates
	isDirty bool
}

// initialize initializes the transform component from config
func (tc *transformComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	tc.componentBase.initialize(sprite, spriteCfg)
	// Always initialize from config
	tc.x = spriteCfg.X
	tc.y = spriteCfg.Y
	tc.direction = spriteCfg.Heading
	tc.rotationStyle = toRotationStyle(spriteCfg.RotationStyle)
	tc.scale = spriteCfg.Size
	tc.pivot = spriteCfg.Pivot
	tc.isDirty = false
}

// cloneFrom creates a new transform component by cloning from source
func (tc *transformComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcTransform := src.(*transformComponent)
	return &transformComponent{
		componentBase: componentBase{sprite: newSprite},
		x:             srcTransform.x,
		y:             srcTransform.y,
		direction:     srcTransform.direction,
		rotationStyle: srcTransform.rotationStyle,
		scale:         srcTransform.scale,
		pivot:         srcTransform.pivot,
		isDirty:       false, // Reset dirty flag for new sprite
	}
}

// onDestroy cleanup when component is destroyed
func (tc *transformComponent) onDestroy() {
	// Nothing to cleanup
}

// ============================================================================
// Position Methods
// ============================================================================

func (tc *transformComponent) GetXY() (x, y float64) {
	return tc.x, tc.y
}

func (tc *transformComponent) Xpos() float64 {
	return tc.x
}

func (tc *transformComponent) Ypos() float64 {
	return tc.y
}

func (tc *transformComponent) SetXpos(x float64) {
	tc.doMoveTo(x, tc.y)
}

func (tc *transformComponent) SetYpos(y float64) {
	tc.doMoveTo(tc.x, y)
}

func (tc *transformComponent) SetXYpos(x, y float64) {
	tc.doMoveTo(x, y)
}

func (tc *transformComponent) ChangeXpos(dx float64) {
	tc.doMoveTo(tc.x+dx, tc.y)
}

func (tc *transformComponent) ChangeYpos(dy float64) {
	tc.doMoveTo(tc.x, tc.y+dy)
}

func (tc *transformComponent) ChangeXYpos(dx, dy float64) {
	tc.doMoveTo(tc.x+dx, tc.y+dy)
}

func (tc *transformComponent) doMoveTo(x, y float64) {
	x, y = tc.fixWorldRange(x, y)
	if tc.sprite.isPenDown() {
		tc.sprite.movePen(x, y)
	}
	tc.x, tc.y = x, y
	tc.updateTransform()
}

func (tc *transformComponent) updateTransform() {
	tc.isDirty = true
	tc.sprite.isDirty = true
}

// fixWorldRange clamps sprite position within world boundaries
func (tc *transformComponent) fixWorldRange(x, y float64) (float64, float64) {
	rect := tc.sprite.bounds()
	if rect == nil {
		return x, y
	}
	worldW, worldH := tc.sprite.g.worldSize_()
	maxW := float64(worldW)/2.0 + float64(rect.Size.X)
	maxH := float64(worldH)/2.0 + float64(rect.Size.Y)
	if x < -maxW {
		x = -maxW
	}
	if x > maxW {
		x = maxW
	}
	if y < -maxH {
		y = -maxH
	}
	if y > maxH {
		y = maxH
	}
	return x, y
}

// ============================================================================
// Movement Methods
// ============================================================================

func (tc *transformComponent) MoveForward(step float64) {
	sin, cos := math.Sincos(toRadian(tc.direction))
	tc.doMoveTo(tc.x+step*sin, tc.y+step*cos)
}

func (tc *transformComponent) Glide(x, y float64, secs float64) {
	if debugInstr {
		spxlog.Debug("Glide: sprite=%s, x=%v, y=%v, secs=%v", tc.sprite.name, x, y, secs)
	}
	x0, y0 := tc.GetXY()
	from := mathf.NewVec2(x0, y0)
	to := mathf.NewVec2(x, y)
	anicopy := aniConfig{
		Duration: secs,
		From:     &from,
		To:       &to,
		AniType:  aniTypeGlide,
		IsLoop:   true,
	}
	animName := tc.sprite.getStateAnimName(StateGlide)
	tc.sprite.components.Animation().doTween(animName, &anicopy)
}

func (tc *transformComponent) GlideTo(obj any, secs float64) {
	x, y := tc.sprite.g.objectPos(obj)
	tc.Glide(x, y, secs)
}

func (tc *transformComponent) StepToPos(x, y, speed float64, animation SpriteAnimationName) {
	if animation == "" {
		animation = tc.sprite.getStateAnimName(StateStep)
	}
	// if no animation, goto target immediately
	if !tc.sprite.hasAnim(animation) {
		tc.SetXYpos(x, y)
	} else {
		speed = math.Max(speed, 0.001)
		from := mathf.NewVec2(tc.x, tc.y)
		to := mathf.NewVec2(x, y)
		distance := from.DistanceTo(to)
		if ani, ok := tc.sprite.animations[animation]; ok {
			anicopy := *ani
			anicopy.From = &from
			anicopy.To = &to
			anicopy.AniType = aniTypeMove
			anicopy.Duration = math.Abs(distance) * ani.StepDuration / speed
			anicopy.IsLoop = true
			anicopy.Speed = speed
			tc.sprite.doTween(animation, &anicopy)
			return
		}
	}
}

func (tc *transformComponent) StepTo(obj any, speed float64, animation SpriteAnimationName) {
	x, y := tc.sprite.g.objectPos(obj)
	tc.StepToPos(x, y, speed, animation)
}

func (tc *transformComponent) Step(step float64, speed float64, animation SpriteAnimationName) {
	dirSin, dirCos := math.Sincos(toRadian(tc.direction))
	diff := mathf.NewVec2(step*dirSin, step*dirCos)
	to := mathf.NewVec2(tc.x, tc.y).Add(diff)
	tc.StepToPos(to.X, to.Y, speed, animation)
}

// ============================================================================
// Rotation Methods
// ============================================================================

func (tc *transformComponent) Heading() Direction {
	return tc.direction
}

func (tc *transformComponent) SetHeading(dir Direction) {
	tc.setDirection(dir, false)
}

func (tc *transformComponent) ChangeHeading(dir Direction) {
	tc.setDirection(dir, true)
}

func (tc *transformComponent) SetRotationStyle(style RotationStyle) {
	if debugInstr {
		spxlog.Debug("SetRotationStyle: sprite=%s, style=%v", tc.sprite.name, style)
	}
	tc.rotationStyle = style
}

func (tc *transformComponent) setDirection(dir float64, change bool) bool {
	if change {
		dir += tc.direction
	}
	dir = normalizeDirection(dir)
	if tc.direction == dir {
		return false
	}
	tc.direction = dir
	tc.updateTransform()
	return true
}

func (tc *transformComponent) Turn(val Direction, speed float64, animation SpriteAnimationName) {
	delta := val
	if animation == "" {
		animation = tc.sprite.getStateAnimName(StateTurn)
	}
	if ani, ok := tc.sprite.animations[animation]; ok {
		anicopy := *ani
		anicopy.From = tc.direction
		anicopy.To = tc.direction + delta
		anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta) / speed
		anicopy.AniType = aniTypeTurn
		anicopy.IsLoop = true
		anicopy.Speed = speed
		tc.sprite.doTween(animation, &anicopy)
		return
	}
	tc.setDirection(delta, true)
	if debugInstr {
		spxlog.Debug("Turn: sprite=%s, val=%v", tc.sprite.name, val)
	}
}

func (tc *transformComponent) TurnTo(obj any, speed float64, animation SpriteAnimationName) {
	var angle float64
	switch v := obj.(type) {
	case Direction:
		angle = v
	default:
		x, y := tc.sprite.g.objectPos(obj)
		dx := x - tc.x
		dy := y - tc.y
		angle = 90 - engine.RadToDeg(math.Atan2(dy, dx))
	}

	if animation == "" {
		animation = tc.sprite.getStateAnimName(StateTurn)
	}
	if ani, ok := tc.sprite.animations[animation]; ok {
		fromangle := math.Mod(tc.direction+360.0, 360.0)
		toangle := math.Mod(angle+360.0, 360.0)
		if toangle-fromangle > 180.0 {
			fromangle = fromangle + 360.0
		}
		if fromangle-toangle > 180.0 {
			toangle = toangle + 360.0
		}
		delta := math.Abs(fromangle - toangle)
		anicopy := *ani
		anicopy.From = fromangle
		anicopy.To = toangle
		anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta) / speed
		anicopy.AniType = aniTypeTurn
		anicopy.IsLoop = true
		anicopy.Speed = speed
		tc.sprite.components.Animation().doTween(animation, &anicopy)
		return
	}
	if tc.setDirection(angle, false) && debugInstr {
		spxlog.Debug("TurnTo: sprite=%s, obj=%v", tc.sprite.name, obj)
	}
}

func (tc *transformComponent) BounceOffEdge() {
	if debugInstr {
		spxlog.Debug("BounceOffEdge: %s", tc.sprite.name)
	}

	nearestEdge := tc.sprite.checkNearestTouchedBoundary()

	if nearestEdge == 0 {
		return
	}

	// prevents sprites from getting stuck at boundaries
	const minBounceComponent = 0.2
	radians := toRadian(90 - tc.direction)
	dx := math.Cos(radians)
	dy := -math.Sin(radians)

	switch nearestEdge {
	case touchingScreenLeft:
		dx = math.Max(minBounceComponent, math.Abs(dx))
	case touchingScreenTop:
		dy = math.Max(minBounceComponent, math.Abs(dy))
	case touchingScreenRight:
		dx = -math.Max(minBounceComponent, math.Abs(dx))
	case touchingScreenBottom:
		dy = -math.Max(minBounceComponent, math.Abs(dy))
	}

	newDirection := engine.RadToDeg(math.Atan2(dy, dx)) + 90
	tc.direction = normalizeDirection(newDirection)
}

// ============================================================================
// Scale Methods
// ============================================================================

func (tc *transformComponent) Size() float64 {
	return tc.scale
}

func (tc *transformComponent) SetSize(size float64) {
	if debugInstr {
		spxlog.Debug("SetSize: sprite=%s, size=%v", tc.sprite.name, size)
	}
	tc.scale = size
	tc.updateTransform()
	tc.sprite.isCostumeDirty = true
	tc.sprite.updateScale()
}

func (tc *transformComponent) ChangeSize(delta float64) {
	if debugInstr {
		spxlog.Debug("ChangeSize: sprite=%s, delta=%v", tc.sprite.name, delta)
	}
	tc.SetSize(tc.scale + delta)
}

// ============================================================================
// Distance Calculation
// ============================================================================

func (tc *transformComponent) DistanceTo(obj any) float64 {
	x, y := tc.x, tc.y
	x2, y2 := tc.sprite.g.objectPos(obj)
	x -= x2
	y -= y2
	return math.Sqrt(x*x + y*y)
}
