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
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// -----------------------------------------------------------------------------
// Position
// -----------------------------------------------------------------------------
func (p *SpriteImpl) getXY() (x, y float64) {
	return p.transform().XY()
}

func (p *SpriteImpl) DistanceTo__0(sprite Sprite) float64 {
	return p.transform().DistanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__1(sprite SpriteName) float64 {
	return p.transform().DistanceTo(sprite)
}

func (p *SpriteImpl) DistanceTo__2(obj specialObj) float64 {
	return p.transform().DistanceTo(obj)
}

func (p *SpriteImpl) DistanceTo__3(pos Pos) float64 {
	return p.transform().DistanceTo(pos)
}

func (p *SpriteImpl) DistanceToWith(target Target) float64 {
	return p.transform().DistanceTo(target)
}

func (p *SpriteImpl) DirectionTo__0(sprite Sprite) Direction {
	return p.transform().DirectionTo(sprite)
}

func (p *SpriteImpl) DirectionTo__1(sprite SpriteName) Direction {
	return p.transform().DirectionTo(sprite)
}

func (p *SpriteImpl) DirectionTo__2(obj specialObj) Direction {
	return p.transform().DirectionTo(obj)
}

func (p *SpriteImpl) DirectionTo__3(pos Pos) Direction {
	return p.transform().DirectionTo(pos)
}

func (p *SpriteImpl) DirectionTo__4(x, y float64) Direction {
	return p.transform().DirectionToPos(x, y)
}

func (p *SpriteImpl) DirectionToWith(target Target) Direction {
	return p.transform().DirectionTo(target)
}

// -----------------------------------------------------------------------------
// Movement
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Move__0(step float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Move: sprite=%s, step=%v", p.name, step)
	}
	p.transform().MoveForward(step)
}

func (p *SpriteImpl) Move__1(step int) {
	p.Move__0(float64(step))
}

func (p *SpriteImpl) Step__0(step float64) {
	p.transform().Step(step, 1, "")
}

func (p *SpriteImpl) Step__1(step float64, speed float64) {
	p.transform().Step(step, speed, "")
}

func (p *SpriteImpl) Step__2(step float64, speed float64, animation SpriteAnimationName) {
	p.transform().Step(step, speed, animation)
}

func (p *SpriteImpl) StepWith(step float64, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().Step(step, speed, animation)
}

func (p *SpriteImpl) doStepTo(obj any, speed float64, animation SpriteAnimationName) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Goto: sprite=%s, obj=%v", p.name, obj)
	}
	x, y := p.g.objectPos(obj)
	p.transform().StepToPos(x, y, speed, animation)
}

func motionOptions(opts *MotionOptions) (speed Speed, animation SpriteAnimationName) {
	speed = 1
	if opts == nil {
		return
	}
	if opts.Speed > 0 {
		speed = opts.Speed
	} else if opts.Speed < 0 {
		spxlog.Warn("MotionOptions.Speed=%v is negative, defaulting to 1", opts.Speed)
	}
	animation = opts.Animation
	return
}

func (p *SpriteImpl) StepTo__0(sprite Sprite) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__1(sprite SpriteName) {
	p.doStepTo(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__2(obj specialObj) {
	p.doStepTo(obj, 1, "")
}

func (p *SpriteImpl) StepTo__3(x, y float64) {
	p.transform().StepToPos(x, y, 1, "")
}

func (p *SpriteImpl) StepTo__4(sprite Sprite, speed float64) {
	p.doStepTo(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__5(sprite SpriteName, speed float64) {
	p.doStepTo(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__6(obj specialObj, speed float64) {
	p.doStepTo(obj, speed, "")
}

func (p *SpriteImpl) StepTo__7(x, y, speed float64) {
	p.transform().StepToPos(x, y, speed, "")
}

func (p *SpriteImpl) StepTo__8(sprite Sprite, speed float64, animation SpriteAnimationName) {
	p.doStepTo(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__9(sprite SpriteName, speed float64, animation SpriteAnimationName) {
	p.doStepTo(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__a(obj specialObj, speed float64, animation SpriteAnimationName) {
	p.doStepTo(obj, speed, animation)
}

func (p *SpriteImpl) StepTo__b(x, y, speed float64, animation SpriteAnimationName) {
	p.transform().StepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) StepTo__c(pos Pos) {
	p.doStepTo(pos, 1, "")
}

func (p *SpriteImpl) StepTo__d(pos Pos, speed float64) {
	p.doStepTo(pos, speed, "")
}

func (p *SpriteImpl) StepTo__e(pos Pos, speed float64, animation SpriteAnimationName) {
	p.doStepTo(pos, speed, animation)
}

func (p *SpriteImpl) StepToTarget(target Target, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.doStepTo(target, speed, animation)
}

func (p *SpriteImpl) StepToXYpos(x, y float64, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	// Coordinate overloads already have their target position, so they mirror
	// the positional StepTo variants and intentionally skip doStepTo's logging.
	p.transform().StepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) doGlideTo(obj Target, secs Seconds) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Glide: obj=%v, secs=%v", obj, secs)
	}
	x, y := p.g.objectPos(obj)
	p.transform().Glide(x, y, secs)
}

func (p *SpriteImpl) Glide__0(sprite Sprite, secs Seconds) {
	p.doGlideTo(sprite, secs)
}

func (p *SpriteImpl) Glide__1(sprite SpriteName, secs Seconds) {
	p.doGlideTo(sprite, secs)
}

func (p *SpriteImpl) Glide__2(obj specialObj, secs Seconds) {
	p.doGlideTo(obj, secs)
}

func (p *SpriteImpl) Glide__3(pos Pos, secs Seconds) {
	p.doGlideTo(pos, secs)
}

func (p *SpriteImpl) Glide__4(x, y float64, secs Seconds) {
	p.transform().Glide(x, y, secs)
}

func (p *SpriteImpl) GlideToTarget(target Target, secs Seconds) {
	p.doGlideTo(target, secs)
}

func (p *SpriteImpl) GlideToXYpos(x, y float64, secs Seconds) {
	p.transform().Glide(x, y, secs)
}

func (p *SpriteImpl) SetXYpos(x, y float64) {
	p.transform().SetXYpos(x, y)
}

func (p *SpriteImpl) ChangeXYpos(dx, dy float64) {
	p.transform().ChangeXYpos(dx, dy)
}

func (p *SpriteImpl) Xpos() float64 {
	return p.transform().Xpos()
}

func (p *SpriteImpl) SetXpos(x float64) {
	p.transform().SetXpos(x)
}

func (p *SpriteImpl) ChangeXpos(dx float64) {
	p.transform().ChangeXpos(dx)
}

func (p *SpriteImpl) Ypos() float64 {
	return p.transform().Ypos()
}

func (p *SpriteImpl) SetYpos(y float64) {
	p.transform().SetYpos(y)
}

func (p *SpriteImpl) ChangeYpos(dy float64) {
	p.transform().ChangeYpos(dy)
}

// -----------------------------------------------------------------------------
// Rotation
// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetRotationStyle(style RotationStyle) {
	p.transform().SetRotationStyle(style)
}

func (p *SpriteImpl) Heading() Direction {
	return p.transform().Heading()
}

func (p *SpriteImpl) Turn__0(dir Direction) {
	p.transform().Turn(dir, 1, "")
}

func (p *SpriteImpl) Turn__1(dir Direction, speed float64) {
	p.transform().Turn(dir, speed, "")
}

func (p *SpriteImpl) Turn__2(dir Direction, speed float64, animation SpriteAnimationName) {
	p.transform().Turn(dir, speed, animation)
}

func (p *SpriteImpl) TurnWith(dir Direction, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().Turn(dir, speed, animation)
}

func (p *SpriteImpl) TurnTo__0(target Sprite) {
	p.transform().TurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__1(target SpriteName) {
	p.transform().TurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__2(dir Direction) {
	p.transform().TurnTo(dir, 1, "")
}

func (p *SpriteImpl) TurnTo__3(target specialObj) {
	p.transform().TurnTo(target, 1, "")
}

func (p *SpriteImpl) TurnTo__4(target Sprite, speed float64) {
	p.transform().TurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__5(target SpriteName, speed float64) {
	p.transform().TurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__6(dir Direction, speed float64) {
	p.transform().TurnTo(dir, speed, "")
}

func (p *SpriteImpl) TurnTo__7(target specialObj, speed float64) {
	p.transform().TurnTo(target, speed, "")
}

func (p *SpriteImpl) TurnTo__8(target Sprite, speed float64, animation SpriteAnimationName) {
	p.transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnTo__9(target SpriteName, speed float64, animation SpriteAnimationName) {
	p.transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnTo__a(dir Direction, speed float64, animation SpriteAnimationName) {
	p.transform().TurnTo(dir, speed, animation)
}

func (p *SpriteImpl) TurnTo__b(target specialObj, speed float64, animation SpriteAnimationName) {
	p.transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnToDir(dir Direction, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().TurnTo(dir, speed, animation)
}

func (p *SpriteImpl) TurnToTarget(target Target, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnToXYpos(x, y float64, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().TurnToPos(x, y, speed, animation)
}

func (p *SpriteImpl) SetHeading(dir Direction) {
	p.transform().SetHeading(dir)
}

func (p *SpriteImpl) ChangeHeading(dir Direction) {
	p.transform().ChangeHeading(dir)
}

func (p *SpriteImpl) BounceOffEdge() {
	p.transform().BounceOffEdge(edgeAreaStage)
}

// -----------------------------------------------------------------------------
// Scale Control
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Size() float64 {
	return p.runtimeState.Scale
}

func (p *SpriteImpl) SetSize(size float64) {
	p.transform().SetSize(size)
}

func (p *SpriteImpl) ChangeSize(delta float64) {
	p.transform().ChangeSize(delta)
}

// -----------------------------------------------------------------------------
// Pivot Control
// -----------------------------------------------------------------------------
func (p *SpriteImpl) getPivot() mathf.Vec2 {
	return p.transform().getPivot()
}
