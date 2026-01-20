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
	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// ======================== Transform Component ========================
// This file contains position, rotation, and scale related functionality
// for sprite transformations.

// -----------------------------------------------------------------------------
// Position Query and Distance Calculation
// -----------------------------------------------------------------------------

func (p *SpriteImpl) getXY() (x, y float64) {
	return p.components.Transform().GetXY()
}

// DistanceTo func:
//
//	DistanceTo(sprite)
//	DistanceTo(spx.Mouse)
//	DistanceTo(spx.Random)
func (p *SpriteImpl) DistanceTo__0(sprite Sprite) float64 {
	return p.components.Transform().DistanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__1(sprite SpriteName) float64 {
	return p.components.Transform().DistanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__2(obj specialObj) float64 {
	return p.components.Transform().DistanceTo(obj)
}

func (p *SpriteImpl) DistanceTo__3(pos Pos) float64 {
	return p.components.Transform().DistanceTo(pos)
}

// -----------------------------------------------------------------------------
// Core Movement Functions
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Move__0(step float64) {
	if debugInstr {
		spxlog.Debug("Move: sprite=%s, step=%v", p.name, step)
	}
	p.components.Transform().MoveForward(step)
}

func (p *SpriteImpl) Move__1(step int) {
	p.Move__0(float64(step))
}

func (p *SpriteImpl) Step__0(step float64) {
	p.components.Transform().Step(step, 1, "")
}

func (p *SpriteImpl) Step__1(step float64, speed float64) {
	p.components.Transform().Step(step, speed, "")
}

func (p *SpriteImpl) Step__2(step float64, speed float64, animation SpriteAnimationName) {
	p.components.Transform().Step(step, speed, animation)
}

func (p *SpriteImpl) doStepTo(obj any, speed float64, animation SpriteAnimationName) {
	if debugInstr {
		spxlog.Debug("Goto: sprite=%s, obj=%v", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.components.Transform().StepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) StepTo__0(sprite Sprite) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__1(sprite SpriteName) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__2(x, y float64) {
	p.components.Transform().StepToPos(x, y, 1, "")
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
	p.components.Transform().StepToPos(x, y, speed, "")
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
	p.components.Transform().StepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) StepTo__b(obj specialObj, speed float64, animation SpriteAnimationName) {
	p.doStepTo(obj, speed, animation)
}

func (p *SpriteImpl) doGlideTo(obj any, secs float64) {
	if debugInstr {
		spxlog.Debug("Glide: obj=%v, secs=%v", obj, secs)
	}
	x, y := p.g.objectPos(obj)
	p.components.Transform().Glide(x, y, secs)
}

func (p *SpriteImpl) Glide__0(x, y float64, secs float64) {
	p.components.Transform().Glide(x, y, secs)
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
	p.components.Transform().SetXYpos(x, y)
}

func (p *SpriteImpl) ChangeXYpos(dx, dy float64) {
	p.components.Transform().ChangeXYpos(dx, dy)
}

func (p *SpriteImpl) Xpos() float64 {
	return p.components.Transform().Xpos()
}

func (p *SpriteImpl) SetXpos(x float64) {
	p.components.Transform().SetXpos(x)
}

func (p *SpriteImpl) ChangeXpos(dx float64) {
	p.components.Transform().ChangeXpos(dx)
}

func (p *SpriteImpl) Ypos() float64 {
	return p.components.Transform().Ypos()
}

func (p *SpriteImpl) SetYpos(y float64) {
	p.components.Transform().SetYpos(y)
}

func (p *SpriteImpl) ChangeYpos(dy float64) {
	p.components.Transform().ChangeYpos(dy)
}

// -----------------------------------------------------------------------------
// Rotation and Direction Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) SetRotationStyle(style RotationStyle) {
	p.components.Transform().SetRotationStyle(style)
}

func (p *SpriteImpl) Heading() Direction {
	return p.components.Transform().Heading()
}

func (p *SpriteImpl) Turn__0(dir Direction) {
	p.components.Transform().Turn(dir, 1, "")
}

func (p *SpriteImpl) Turn__1(dir Direction, speed float64) {
	p.components.Transform().Turn(dir, speed, "")
}

func (p *SpriteImpl) Turn__2(dir Direction, speed float64, animation SpriteAnimationName) {
	p.components.Transform().Turn(dir, speed, animation)
}

func (p *SpriteImpl) TurnTo__0(target Sprite) {
	p.components.Transform().TurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__1(target SpriteName) {
	p.components.Transform().TurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__2(dir Direction) {
	p.components.Transform().TurnTo(dir, 1, "")
}

func (p *SpriteImpl) TurnTo__3(target specialObj) {
	p.components.Transform().TurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__4(target Sprite, speed float64) {
	p.components.Transform().TurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__5(target SpriteName, speed float64) {
	p.components.Transform().TurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__6(dir Direction, speed float64) {
	p.components.Transform().TurnTo(dir, speed, "")
}

func (p *SpriteImpl) TurnTo__7(target specialObj, speed float64) {
	p.components.Transform().TurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__8(target Sprite, speed float64, animation SpriteAnimationName) {
	p.components.Transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnTo__9(target SpriteName, speed float64, animation SpriteAnimationName) {
	p.components.Transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnTo__a(dir Direction, speed float64, animation SpriteAnimationName) {
	p.components.Transform().TurnTo(dir, speed, animation)
}

func (p *SpriteImpl) TurnTo__b(target specialObj, speed float64, animation SpriteAnimationName) {
	p.components.Transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) SetHeading(dir Direction) {
	p.components.Transform().SetHeading(dir)
}

func (p *SpriteImpl) ChangeHeading(dir Direction) {
	p.components.Transform().ChangeHeading(dir)
}

// -----------------------------------------------------------------------------
// Scale Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Size() float64 {
	return p.scale
}

func (p *SpriteImpl) SetSize(size float64) {
	p.components.Transform().SetSize(size)
}

func (p *SpriteImpl) ChangeSize(delta float64) {
	p.components.Transform().ChangeSize(delta)
}

// -----------------------------------------------------------------------------
// Pivot Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) getPivot() mathf.Vec2 {
	return p.components.Transform().getPivot()
}
