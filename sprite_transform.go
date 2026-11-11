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
	spxlog "github.com/goplus/spx/v4/internal/log"
)

// -----------------------------------------------------------------------------
// Position
// -----------------------------------------------------------------------------
func (p *SpriteImpl) DistanceTo__0(sprite Sprite) float64 {
	return p.transform().distanceToTarget(sprite)
}

func (p *SpriteImpl) DistanceTo__1(name SpriteName) float64 {
	return p.transform().distanceToTarget(name)
}

func (p *SpriteImpl) DistanceTo__2(object specialObj) float64 {
	return p.transform().distanceToTarget(object)
}

func (p *SpriteImpl) DistanceTo__3(pos Pos) float64 {
	return p.transform().distanceToTarget(pos)
}

func (p *SpriteImpl) DistanceToWith(target Target) float64 {
	return p.transform().distanceToTarget(target)
}

func (p *SpriteImpl) DirectionTo__0(sprite Sprite) Direction {
	return p.transform().directionToTarget(sprite)
}

func (p *SpriteImpl) DirectionTo__1(sprite SpriteName) Direction {
	return p.transform().directionToTarget(sprite)
}

func (p *SpriteImpl) DirectionTo__2(target specialObj) Direction {
	return p.transform().directionToTarget(target)
}

func (p *SpriteImpl) DirectionTo__3(pos Pos) Direction {
	return p.transform().directionToTarget(pos)
}

func (p *SpriteImpl) DirectionTo__4(x, y float64) Direction {
	return p.transform().directionToPos(x, y)
}

func (p *SpriteImpl) DirectionToWith(target Target) Direction {
	return p.transform().directionToTarget(target)
}

// -----------------------------------------------------------------------------
// Movement
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Move__0(step float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Move: sprite=%s, step=%v", p.name, step)
	}
	p.transform().moveForward(step)
}

func (p *SpriteImpl) Move__1(step int) {
	p.Move__0(float64(step))
}

func (p *SpriteImpl) Step__0(distance float64) {
	p.transform().step(distance, 1, "")
}

func (p *SpriteImpl) Step__1(distance float64, speed float64) {
	p.transform().step(distance, speed, "")
}

func (p *SpriteImpl) Step__2(distance float64, speed float64, animation SpriteAnimationName) {
	p.transform().step(distance, speed, animation)
}

func (p *SpriteImpl) StepWith(distance float64, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().step(distance, speed, animation)
}

func (p *SpriteImpl) StepTo__0(sprite Sprite) {
	p.transform().stepToTarget(sprite, 1, "")
}

func (p *SpriteImpl) StepTo__1(name SpriteName) {
	p.transform().stepToTarget(name, 1, "")
}

func (p *SpriteImpl) StepTo__2(object specialObj) {
	p.transform().stepToTarget(object, 1, "")
}

func (p *SpriteImpl) StepTo__3(x, y float64) {
	p.transform().stepToPos(x, y, 1, "")
}

func (p *SpriteImpl) StepTo__4(sprite Sprite, speed float64) {
	p.transform().stepToTarget(sprite, speed, "")
}

func (p *SpriteImpl) StepTo__5(name SpriteName, speed float64) {
	p.transform().stepToTarget(name, speed, "")
}

func (p *SpriteImpl) StepTo__6(object specialObj, speed float64) {
	p.transform().stepToTarget(object, speed, "")
}

func (p *SpriteImpl) StepTo__7(x, y, speed float64) {
	p.transform().stepToPos(x, y, speed, "")
}

func (p *SpriteImpl) StepTo__8(sprite Sprite, speed float64, animation SpriteAnimationName) {
	p.transform().stepToTarget(sprite, speed, animation)
}

func (p *SpriteImpl) StepTo__9(name SpriteName, speed float64, animation SpriteAnimationName) {
	p.transform().stepToTarget(name, speed, animation)
}

func (p *SpriteImpl) StepTo__a(object specialObj, speed float64, animation SpriteAnimationName) {
	p.transform().stepToTarget(object, speed, animation)
}

func (p *SpriteImpl) StepTo__b(x, y, speed float64, animation SpriteAnimationName) {
	p.transform().stepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) StepTo__c(pos Pos) {
	p.transform().stepToTarget(pos, 1, "")
}

func (p *SpriteImpl) StepTo__d(pos Pos, speed float64) {
	p.transform().stepToTarget(pos, speed, "")
}

func (p *SpriteImpl) StepTo__e(pos Pos, speed float64, animation SpriteAnimationName) {
	p.transform().stepToTarget(pos, speed, animation)
}

func (p *SpriteImpl) StepToTarget(target Target, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().stepToTarget(target, speed, animation)
}

func (p *SpriteImpl) StepToXYpos(x, y float64, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	// Coordinate overloads already have their target position, so they mirror
	// the positional StepTo variants and skip target resolution.
	p.transform().stepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) Glide__0(sprite Sprite, seconds Seconds) {
	p.transform().glideToTarget(sprite, seconds)
}

func (p *SpriteImpl) Glide__1(name SpriteName, seconds Seconds) {
	p.transform().glideToTarget(name, seconds)
}

func (p *SpriteImpl) Glide__2(object specialObj, seconds Seconds) {
	p.transform().glideToTarget(object, seconds)
}

func (p *SpriteImpl) Glide__3(pos Pos, seconds Seconds) {
	p.transform().glideToTarget(pos, seconds)
}

func (p *SpriteImpl) Glide__4(x, y float64, seconds Seconds) {
	p.transform().glide(x, y, seconds)
}

func (p *SpriteImpl) GlideToTarget(target Target, secs Seconds) {
	p.transform().glideToTarget(target, secs)
}

func (p *SpriteImpl) GlideToXYpos(x, y float64, secs Seconds) {
	p.transform().glide(x, y, secs)
}

func (p *SpriteImpl) SetXYpos(x, y float64) {
	p.transform().setPosition(x, y)
}

func (p *SpriteImpl) ChangeXYpos(dX, dY float64) {
	p.transform().changePosition(dX, dY)
}

func (p *SpriteImpl) Xpos() float64 {
	return p.transform().x
}

func (p *SpriteImpl) SetXpos(x float64) {
	p.transform().setX(x)
}

func (p *SpriteImpl) ChangeXpos(dX float64) {
	p.transform().changeX(dX)
}

func (p *SpriteImpl) Ypos() float64 {
	return p.transform().y
}

func (p *SpriteImpl) SetYpos(y float64) {
	p.transform().setY(y)
}

func (p *SpriteImpl) ChangeYpos(dY float64) {
	p.transform().changeY(dY)
}

// -----------------------------------------------------------------------------
// Rotation
// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetRotationStyle(style RotationStyle) {
	p.transform().setRotationStyle(style)
}

func (p *SpriteImpl) Heading() Direction {
	return p.transform().direction
}

func (p *SpriteImpl) Turn__0(direction Direction) {
	p.transform().turn(direction, 1, "")
}

func (p *SpriteImpl) Turn__1(direction Direction, speed float64) {
	p.transform().turn(direction, speed, "")
}

func (p *SpriteImpl) Turn__2(direction Direction, speed float64, animation SpriteAnimationName) {
	p.transform().turn(direction, speed, animation)
}

func (p *SpriteImpl) TurnWith(direction Direction, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().turn(direction, speed, animation)
}

func (p *SpriteImpl) TurnTo__0(sprite Sprite) {
	p.transform().turnTo(sprite, 1, "")
}

func (p *SpriteImpl) TurnTo__1(name SpriteName) {
	p.transform().turnTo(name, 1, "")
}

func (p *SpriteImpl) TurnTo__2(direction Direction) {
	p.transform().turnTo(direction, 1, "")
}

func (p *SpriteImpl) TurnTo__3(object specialObj) {
	p.transform().turnTo(object, 1, "")
}

func (p *SpriteImpl) TurnTo__4(sprite Sprite, speed float64) {
	p.transform().turnTo(sprite, speed, "")
}

func (p *SpriteImpl) TurnTo__5(name SpriteName, speed float64) {
	p.transform().turnTo(name, speed, "")
}

func (p *SpriteImpl) TurnTo__6(direction Direction, speed float64) {
	p.transform().turnTo(direction, speed, "")
}

func (p *SpriteImpl) TurnTo__7(object specialObj, speed float64) {
	p.transform().turnTo(object, speed, "")
}

func (p *SpriteImpl) TurnTo__8(sprite Sprite, speed float64, animation SpriteAnimationName) {
	p.transform().turnTo(sprite, speed, animation)
}

func (p *SpriteImpl) TurnTo__9(name SpriteName, speed float64, animation SpriteAnimationName) {
	p.transform().turnTo(name, speed, animation)
}

func (p *SpriteImpl) TurnTo__a(direction Direction, speed float64, animation SpriteAnimationName) {
	p.transform().turnTo(direction, speed, animation)
}

func (p *SpriteImpl) TurnTo__b(object specialObj, speed float64, animation SpriteAnimationName) {
	p.transform().turnTo(object, speed, animation)
}

func (p *SpriteImpl) TurnToDir(dir Direction, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().turnTo(dir, speed, animation)
}

func (p *SpriteImpl) TurnToTarget(target Target, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().turnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnToXYpos(x, y float64, __xgo_optional_opts *MotionOptions) {
	speed, animation := motionOptions(__xgo_optional_opts)
	p.transform().turnToPos(x, y, speed, animation)
}

func (p *SpriteImpl) SetHeading(direction Direction) {
	p.transform().setHeading(direction)
}

func (p *SpriteImpl) ChangeHeading(degree Direction) {
	p.transform().changeHeading(degree)
}

func (p *SpriteImpl) BounceOffEdge() {
	p.transform().bounceOffEdge(edgeAreaStage)
}

// -----------------------------------------------------------------------------
// Scale Control
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Size() float64 {
	return p.runtimeState.scale
}

func (p *SpriteImpl) SetSize(size float64) {
	p.transform().setSize(size)
}

func (p *SpriteImpl) ChangeSize(dSize float64) {
	p.transform().changeSize(dSize)
}

// -----------------------------------------------------------------------------
// Internal Helpers
// -----------------------------------------------------------------------------
func (p *SpriteImpl) getXY() (x, y float64) {
	return p.transform().getXY()
}

func (p *SpriteImpl) getPivot() mathf.Vec2 {
	return p.transform().pivot
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
