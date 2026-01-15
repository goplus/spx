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

// ======================== Transform Component ========================
// This file contains position, rotation, and scale related functionality
// for sprite transformations.

// -----------------------------------------------------------------------------
// Position Query and Distance Calculation
// -----------------------------------------------------------------------------

func (p *SpriteImpl) getXY() (x, y float64) {
	return p.x, p.y
}

// DistanceTo func:
//
//	DistanceTo(sprite)
//	DistanceTo(spx.Mouse)
//	DistanceTo(spx.Random)
func (p *SpriteImpl) distanceTo(obj any) float64 {
	x, y := p.x, p.y
	x2, y2 := p.g.objectPos(obj)
	x -= x2
	y -= y2
	return math.Sqrt(x*x + y*y)
}

func (p *SpriteImpl) DistanceTo__0(sprite Sprite) float64 {
	return p.distanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__1(sprite SpriteName) float64 {
	return p.distanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__2(obj specialObj) float64 {
	return p.distanceTo(obj)
}

func (p *SpriteImpl) DistanceTo__3(pos Pos) float64 {
	return p.distanceTo(pos)
}

// -----------------------------------------------------------------------------
// Core Movement Functions
// -----------------------------------------------------------------------------

func (p *SpriteImpl) doMoveTo(x, y float64) {
	p.doMoveToForAnim(x, y)
}

func (p *SpriteImpl) doMoveToForAnim(x, y float64) {
	x, y = p.fixWorldRange(x, y)
	if p.isPenDown {
		p.movePen(x, y)
	}
	p.x, p.y = x, y
	p.updateTransform()
}

func (p *SpriteImpl) updateTransform() {
	p.isDirty = true
}

func (p *SpriteImpl) updateScale() {
	p.triggerInfo.applyShape(p.syncSprite, true, p.scale)
	p.collisionInfo.applyShape(p.syncSprite, false, p.scale)
}

func (p *SpriteImpl) goMoveForward(step float64) {
	sin, cos := math.Sincos(toRadian(p.direction))
	p.doMoveTo(p.x+step*sin, p.y+step*cos)
}

func (p *SpriteImpl) Move__0(step float64) {
	if debugInstr {
		spxlog.Debug("Move: sprite=%s, step=%v", p.name, step)
	}
	p.goMoveForward(step)
}

func (p *SpriteImpl) Move__1(step int) {
	p.Move__0(float64(step))
}

func (p *SpriteImpl) Step__0(step float64) {
	p.doStep(step, 1, "")
}

func (p *SpriteImpl) Step__1(step float64, speed float64) {
	p.doStep(step, speed, "")
}

func (p *SpriteImpl) Step__2(step float64, speed float64, animation SpriteAnimationName) {
	p.doStep(step, speed, animation)
}

func (p *SpriteImpl) doStepToPos(x, y, speed float64, animation SpriteAnimationName) {
	if animation == "" {
		animation = p.getStateAnimName(StateStep)
	}
	// if no animation, goto target immediately (unified check for Spine + frame animations)
	if !p.hasAnimation(animation) {
		p.SetXYpos(x, y)
		return
	}

	speed = math.Max(speed, 0.001)
	from := mathf.NewVec2(p.x, p.y)
	to := mathf.NewVec2(x, y)
	distance := from.DistanceTo(to)

	// Get animation config, use default for Spine mode if not found
	ani := p.animations[animation]
	if ani == nil {
		ani = p.getDefaultTweenConfig()
	}

	anicopy := *ani
	anicopy.From = &from
	anicopy.To = &to
	anicopy.AniType = aniTypeMove
	anicopy.Duration = math.Abs(distance) * ani.StepDuration / speed
	anicopy.IsLoop = true
	anicopy.Speed = speed
	p.doTween(animation, &anicopy)
}

func (p *SpriteImpl) doStepTo(obj any, speed float64, animation SpriteAnimationName) {
	if debugInstr {
		spxlog.Debug("Goto: sprite=%s, obj=%v", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.doStepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) doStep(step float64, speed float64, animation SpriteAnimationName) {
	dirSin, dirCos := math.Sincos(toRadian(p.direction))
	diff := mathf.NewVec2(step*dirSin, step*dirCos)
	to := mathf.NewVec2(p.x, p.y).Add(diff)
	p.doStepToPos(to.X, to.Y, speed, animation)
}

func (p *SpriteImpl) StepTo__0(sprite Sprite) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__1(sprite SpriteName) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__2(x, y float64) {
	p.doStepToPos(x, y, 1, "")
}

func (p *SpriteImpl) StepTo__3(obj specialObj) {
	p.doStepTo(obj, 1, "")
}

func (p *SpriteImpl) StepTo__4(sprite Sprite, speed float64) {
	p.doStepTo(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__5(sprite SpriteName, speed float64) {
	p.doStepTo(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__6(x, y, speed float64) {
	p.doStepToPos(x, y, speed, "")
}

func (p *SpriteImpl) StepTo__7(obj specialObj, speed float64) {
	p.doStepTo(obj, speed, "")
}

func (p *SpriteImpl) StepTo__8(sprite Sprite, speed float64, animation SpriteAnimationName) {
	p.doStepTo(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__9(sprite SpriteName, speed float64, animation SpriteAnimationName) {
	p.doStepTo(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__a(x, y, speed float64, animation SpriteAnimationName) {
	p.doStepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) StepTo__b(obj specialObj, speed float64, animation SpriteAnimationName) {
	p.doStepTo(obj, speed, animation)
}

func (p *SpriteImpl) doGlideTo(obj any, secs float64) {
	if debugInstr {
		spxlog.Debug("Glide: obj=%v, secs=%v", obj, secs)
	}
	x, y := p.g.objectPos(obj)
	p.doGlide(x, y, secs)
}

func (p *SpriteImpl) doGlide(x, y float64, secs float64) {
	if debugInstr {
		spxlog.Debug("Glide: sprite=%s, x=%v, y=%v, secs=%v", p.name, x, y, secs)
	}
	x0, y0 := p.getXY()
	from := mathf.NewVec2(x0, y0)
	to := mathf.NewVec2(x, y)
	anicopy := aniConfig{
		Duration: secs,
		From:     &from,
		To:       &to,
		AniType:  aniTypeGlide,
	}
	anicopy.IsLoop = true
	animName := p.getStateAnimName(StateGlide)
	p.doTween(animName, &anicopy)
}

func (p *SpriteImpl) Glide__0(x, y float64, secs float64) {
	p.doGlide(x, y, secs)
}

func (p *SpriteImpl) Glide__1(sprite Sprite, secs float64) {
	p.doGlideTo(sprite, secs)
}

func (p *SpriteImpl) Glide__2(sprite SpriteName, secs float64) {
	p.doGlideTo(sprite, secs)
}

func (p *SpriteImpl) Glide__3(obj specialObj, secs float64) {
	p.doGlideTo(obj, secs)
}

func (p *SpriteImpl) Glide__4(pos Pos, secs float64) {
	p.doGlideTo(pos, secs)
}

func (p *SpriteImpl) SetXYpos(x, y float64) {
	p.doMoveTo(x, y)
}

func (p *SpriteImpl) ChangeXYpos(dx, dy float64) {
	p.doMoveTo(p.x+dx, p.y+dy)
}

func (p *SpriteImpl) Xpos() float64 {
	return p.x
}

func (p *SpriteImpl) SetXpos(x float64) {
	p.doMoveTo(x, p.y)
}

func (p *SpriteImpl) ChangeXpos(dx float64) {
	p.doMoveTo(p.x+dx, p.y)
}

func (p *SpriteImpl) Ypos() float64 {
	return p.y
}

func (p *SpriteImpl) SetYpos(y float64) {
	p.doMoveTo(p.x, y)
}

func (p *SpriteImpl) ChangeYpos(dy float64) {
	p.doMoveTo(p.x, p.y+dy)
}

// -----------------------------------------------------------------------------
// Rotation and Direction Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) SetRotationStyle(style RotationStyle) {
	if debugInstr {
		spxlog.Debug("SetRotationStyle: sprite=%s, style=%v", p.name, style)
	}
	p.rotationStyle = style
}

func (p *SpriteImpl) Heading() Direction {
	return p.direction
}

func (p *SpriteImpl) Turn__0(dir Direction) {
	p.doTurn(dir, 1, "")
}

func (p *SpriteImpl) Turn__1(dir Direction, speed float64) {
	p.doTurn(dir, speed, "")
}

func (p *SpriteImpl) Turn__2(dir Direction, speed float64, animation SpriteAnimationName) {
	p.doTurn(dir, speed, animation)
}

func (p *SpriteImpl) TurnTo__0(target Sprite) {
	p.doTurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__1(target SpriteName) {
	p.doTurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__2(dir Direction) {
	p.doTurnTo(dir, 1, "")
}

func (p *SpriteImpl) TurnTo__3(target specialObj) {
	p.doTurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__4(target Sprite, speed float64) {
	p.doTurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__5(target SpriteName, speed float64) {
	p.doTurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__6(dir Direction, speed float64) {
	p.doTurnTo(dir, speed, "")
}

func (p *SpriteImpl) TurnTo__7(target specialObj, speed float64) {
	p.doTurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__8(target Sprite, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnTo__9(target SpriteName, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnTo__a(dir Direction, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(dir, speed, animation)
}

func (p *SpriteImpl) TurnTo__b(target specialObj, speed float64, animation SpriteAnimationName) {
	p.doTurnTo(target, speed, animation)
}

func (p *SpriteImpl) doTurn(val Direction, speed float64, animation SpriteAnimationName) {
	delta := val
	if animation == "" {
		animation = p.getStateAnimName(StateTurn)
	}

	// Unified check for Spine + frame animations
	if !p.hasAnimation(animation) {
		p.setDirection(delta, true)
		if debugInstr {
			spxlog.Debug("Turn: sprite=%s, val=%v", p.name, val)
		}
		return
	}

	// Get animation config, use default for Spine mode if not found
	ani := p.animations[animation]
	if ani == nil {
		ani = p.getDefaultTweenConfig()
	}

	anicopy := *ani
	anicopy.From = p.direction
	anicopy.To = p.direction + delta
	anicopy.Duration = ani.TurnToDuration / 360.0 * math.Abs(delta) / speed
	anicopy.AniType = aniTypeTurn
	anicopy.IsLoop = true
	anicopy.Speed = speed
	p.doTween(animation, &anicopy)
}

func (p *SpriteImpl) doTurnTo(obj any, speed float64, animation SpriteAnimationName) {
	var angle float64
	switch v := obj.(type) {
	case Direction:
		angle = v
	default:
		x, y := p.g.objectPos(obj)
		dx := x - p.x
		dy := y - p.y
		angle = 90 - engine.RadToDeg(math.Atan2(dy, dx))
	}

	if animation == "" {
		animation = p.getStateAnimName(StateTurn)
	}

	// Unified check for Spine + frame animations
	if !p.hasAnimation(animation) {
		if p.setDirection(angle, false) && debugInstr {
			spxlog.Debug("TurnTo: sprite=%s, obj=%v", p.name, obj)
		}
		return
	}

	// Get animation config, use default for Spine mode if not found
	ani := p.animations[animation]
	if ani == nil {
		ani = p.getDefaultTweenConfig()
	}

	fromangle := math.Mod(p.direction+360.0, 360.0)
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
	p.doTween(animation, &anicopy)
}

func (p *SpriteImpl) SetHeading(dir Direction) {
	p.setDirection(dir, false)
}

func (p *SpriteImpl) ChangeHeading(dir Direction) {
	p.setDirection(dir, true)
}

func (p *SpriteImpl) setDirection(dir float64, change bool) bool {
	if change {
		dir += p.direction
	}
	dir = normalizeDirection(dir)
	if p.direction == dir {
		return false
	}
	p.direction = dir
	p.updateTransform()
	return true
}

// -----------------------------------------------------------------------------
// Scale Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Size() float64 {
	v := p.scale
	return v
}

func (p *SpriteImpl) SetSize(size float64) {
	if debugInstr {
		spxlog.Debug("SetSize: sprite=%s, size=%v", p.name, size)
	}
	p.scale = size
	p.updateTransform()
	p.isCostumeDirty = true
	p.updateScale()
}

func (p *SpriteImpl) ChangeSize(delta float64) {
	if debugInstr {
		spxlog.Debug("ChangeSize: sprite=%s, delta=%v", p.name, delta)
	}
	p.SetSize(p.scale + delta)
}

// -----------------------------------------------------------------------------
// Utility Functions
// -----------------------------------------------------------------------------

// fixWorldRange clamps sprite position within world boundaries
func (p *SpriteImpl) fixWorldRange(x, y float64) (float64, float64) {
	rect := p.bounds()
	if rect == nil {
		return x, y
	}
	worldW, worldH := p.g.worldSize_()
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
