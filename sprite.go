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

import coreevent "github.com/goplus/spx/v2/internal/core/event"

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------
type RotationStyle int

type specialObj int

type switchAction int

type layerAction int

type dirAction int

// MotionOptions groups optional motion parameters so XGo callers can use kwargs.
type MotionOptions struct {
	// Speed scales motion playback. A zero value keeps the default speed (1),
	// and negative values are ignored.
	Speed Speed

	// Animation overrides the default state animation used for the motion.
	Animation SpriteAnimationName
}

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
const (
	Right Direction = 90
	Left  Direction = -90
	Up    Direction = 0
	Down  Direction = 180
)

const (
	touchingScreenLeft   = 1
	touchingScreenTop    = 2
	touchingScreenRight  = 4
	touchingScreenBottom = 8
	touchingAllEdges     = 15
)

const (
	Mouse      specialObj = -5
	Edge       specialObj = touchingAllEdges
	EdgeLeft   specialObj = touchingScreenLeft
	EdgeTop    specialObj = touchingScreenTop
	EdgeRight  specialObj = touchingScreenRight
	EdgeBottom specialObj = touchingScreenBottom
)

const (
	Prev switchAction = -1
	Next switchAction = 1
)

const (
	Front layerAction = -1
	Back  layerAction = 1
)

const (
	Forward  dirAction = -1
	Backward dirAction = 1
)

const (
	None RotationStyle = iota
	Normal
	LeftRight
)

const (
	StateDie   string = "die"
	StateTurn  string = "turn"
	StateGlide string = "glide"
	StateStep  string = "step"
)

const (
	AnimChannelFrame string = "@frame"
	AnimChannelTurn  string = "@turn"
	AnimChannelGlide string = "@glide"
	AnimChannelMove  string = "@move"
)

const (
	colorThreshold = 0.1
	alphaThreshold = 0.05
)

const (
	AllStop              StopKind = coreevent.AllStop
	AllOtherScripts      StopKind = coreevent.AllOtherScripts
	AllSprites           StopKind = coreevent.AllSprites
	ThisSprite           StopKind = coreevent.ThisSprite
	ThisScript           StopKind = coreevent.ThisScript
	OtherScriptsInSprite StopKind = coreevent.OtherScriptsInSprite
)

// VarName identifies a monitor variable by name.
type VarName = string

// Target can be a Sprite, SpriteName, specialObj (Mouse), or Pos (Random).
type Target = any

// Seconds is a time duration in seconds
type Seconds = float64

const (
	// literals with unit for Seconds type.
	// You can use 1s, 0.5s, 100ms, etc.
	XGou_Seconds = "s=1,ms=0.001"
)

// Speed is a motion speed multiplier, where 1 is the default speed
type Speed = float64

// XGo method overloads for Sprite interface
const (
	XGoo_Sprite_GlideWith    = ".GlideToTarget,.GlideToXYpos"
	XGoo_Sprite_StepToWith   = ".StepToTarget,.StepToXYpos"
	XGoo_Sprite_TurnToWith   = ".TurnToDir,.TurnToTarget,.TurnToXYpos"
	XGoo_Sprite_SetLayerWith = ".SetLayerTo,.ChangeLayer"
	XGoo_Sprite_QuoteWith    = ".QuoteMsg,.QuoteMsgEx"

	XGoo_SpriteImpl_GlideWith    = ".GlideToTarget,.GlideToXYpos"
	XGoo_SpriteImpl_StepToWith   = ".StepToTarget,.StepToXYpos"
	XGoo_SpriteImpl_TurnToWith   = ".TurnToDir,.TurnToTarget,.TurnToXYpos"
	XGoo_SpriteImpl_SetLayerWith = ".SetLayerTo,.ChangeLayer"
	XGoo_SpriteImpl_QuoteWith    = ".QuoteMsg,.QuoteMsgEx"
)

// -----------------------------------------------------------------------------
// Sprite Interface
// -----------------------------------------------------------------------------
type Sprite interface {
	IEventSinks
	Shape

	// Core Methods
	Main()
	Name() string
	IsCloned() bool

	Clone__0()
	Clone__1(data any)

	CloneWith(__xgo_optional_data any)

	DeleteThisClone()
	Destroy()
	Die()

	// Visibility Methods
	Hide()
	Show()
	Visible() bool
	HideVar(name VarName)
	ShowVar(name VarName)

	// Position Methods
	Xpos() float64
	Ypos() float64
	SetXpos(x float64)
	SetYpos(y float64)
	SetXYpos(x, y float64)
	ChangeXpos(dx float64)
	ChangeYpos(dy float64)
	ChangeXYpos(dx, dy float64)

	// Movement Methods
	Step__0(step float64)
	Step__1(step float64, speed Speed)
	Step__2(step float64, speed Speed, animation SpriteAnimationName)

	StepWith(step float64, __xgo_optional_opts *MotionOptions)

	StepTo__0(sprite Sprite)
	StepTo__1(sprite SpriteName)
	StepTo__2(obj specialObj)
	StepTo__3(x, y float64)
	StepTo__4(sprite Sprite, speed Speed)
	StepTo__5(sprite SpriteName, speed Speed)
	StepTo__6(obj specialObj, speed Speed)
	StepTo__7(x, y, speed Speed)
	StepTo__8(sprite Sprite, speed Speed, animation SpriteAnimationName)
	StepTo__9(sprite SpriteName, speed Speed, animation SpriteAnimationName)
	StepTo__a(obj specialObj, speed Speed, animation SpriteAnimationName)
	StepTo__b(x, y, speed Speed, animation SpriteAnimationName)

	StepToTarget(target Target, __xgo_optional_opts *MotionOptions)
	StepToXYpos(x, y float64, __xgo_optional_opts *MotionOptions)

	Glide__0(sprite Sprite, secs Seconds)
	Glide__1(sprite SpriteName, secs Seconds)
	Glide__2(obj specialObj, secs Seconds)
	Glide__3(pos Pos, secs Seconds)
	Glide__4(x, y float64, secs Seconds)

	GlideToTarget(target Target, secs Seconds)
	GlideToXYpos(x, y float64, secs Seconds)

	// Heading and Rotation Methods
	Heading() Direction
	SetHeading(dir Direction)
	ChangeHeading(dir Direction)
	SetRotationStyle(style RotationStyle)

	Turn__0(dir Direction)
	Turn__1(dir Direction, speed Speed)
	Turn__2(dir Direction, speed Speed, animation SpriteAnimationName)

	TurnWith(dir Direction, __xgo_optional_opts *MotionOptions)

	TurnTo__0(target Sprite)
	TurnTo__1(target SpriteName)
	TurnTo__2(dir Direction)
	TurnTo__3(target specialObj)
	TurnTo__4(target Sprite, speed Speed)
	TurnTo__5(target SpriteName, speed Speed)
	TurnTo__6(dir Direction, speed Speed)
	TurnTo__7(target specialObj, speed Speed)
	TurnTo__8(target Sprite, speed Speed, animation SpriteAnimationName)
	TurnTo__9(target SpriteName, speed Speed, animation SpriteAnimationName)
	TurnTo__a(dir Direction, speed Speed, animation SpriteAnimationName)
	TurnTo__b(target specialObj, speed Speed, animation SpriteAnimationName)

	TurnToDir(dir Direction, __xgo_optional_opts *MotionOptions)
	TurnToTarget(target Target, __xgo_optional_opts *MotionOptions)
	TurnToXYpos(x, y float64, __xgo_optional_opts *MotionOptions)

	BounceOffEdge()

	// Size Methods
	Size() float64
	SetSize(size float64)
	ChangeSize(delta float64)

	// Layer Methods
	SetLayer__0(layer layerAction)
	SetLayer__1(dir dirAction, delta int)

	SetLayerTo(layer layerAction)
	ChangeLayer(dir dirAction, delta int)

	// Costume Methods
	CostumeName() SpriteCostumeName
	CostumeIndex() int

	SetCostume__0(costume SpriteCostumeName)
	SetCostume__1(index float64)
	SetCostume__2(index int)
	SetCostume__3(action switchAction)

	// Animation Methods
	Animate__0(name SpriteAnimationName)
	Animate__1(name SpriteAnimationName, loop bool)

	AnimateWith(name SpriteAnimationName, __xgo_optional_loop bool)
	AnimateAndWait(name SpriteAnimationName)
	StopAnimation(name SpriteAnimationName)

	// Graphic Effects Methods
	SetGraphicEffect(kind EffectKind, val float64)
	ChangeGraphicEffect(kind EffectKind, delta float64)
	ClearGraphicEffects()

	// Pen Methods
	PenDown()
	PenUp()

	SetPenColor__0(color Color)
	SetPenColor__1(kind PenColorParam, value float64)

	ChangePenColor(kind PenColorParam, delta float64)
	SetPenSize(size float64)
	ChangePenSize(delta float64)
	Stamp()

	// Distance and Detection Methods
	DistanceTo__0(sprite Sprite) float64
	DistanceTo__1(sprite SpriteName) float64
	DistanceTo__2(obj specialObj) float64
	DistanceTo__3(pos Pos) float64

	DistanceToWith(target Target) float64

	DirectionTo__0(sprite Sprite) Direction
	DirectionTo__1(sprite SpriteName) Direction
	DirectionTo__2(obj specialObj) Direction
	DirectionTo__3(pos Pos) Direction
	DirectionTo__4(x, y float64) Direction

	DirectionToWith(target Target) Direction

	Touching__0(sprite Sprite) bool
	Touching__1(sprite SpriteName) bool
	Touching__2(obj specialObj) bool

	TouchingWith(target Target) bool
	TouchingColor__0(color Color) bool
	TouchingColor__1(spriteColor, targetColor Color) bool

	// Communication Methods
	Say__0(msg any)
	Say__1(msg any, secs Seconds)

	SayWith(msg any, __xgo_optional_secs Seconds)

	Think__0(msg any)
	Think__1(msg any, secs Seconds)

	ThinkWith(msg any, __xgo_optional_secs Seconds)

	Ask(msg any)

	Quote__0(message string)
	Quote__1(message string, secs Seconds)
	Quote__2(message, description string)
	Quote__3(message, description string, secs Seconds)

	QuoteMsg(message string, __xgo_optional_secs Seconds)
	QuoteMsgEx(message, description string, __xgo_optional_secs Seconds)

	// Event Methods
	OnCloned__0(onCloned func())
	OnCloned__1(onCloned func(data any))

	OnTouchStart__0(sprite SpriteName, onTouchStart func())
	OnTouchStart__1(sprite SpriteName, onTouchStart func(Sprite))
	OnTouchStart__2(sprites []SpriteName, onTouchStart func())
	OnTouchStart__3(sprites []SpriteName, onTouchStart func(Sprite))

	// Sound Methods
	Volume() float64
	SetVolume(volume float64)
	ChangeVolume(delta float64)
	GetSoundEffect(kind SoundEffectKind) float64
	SetSoundEffect(kind SoundEffectKind, value float64)
	ChangeSoundEffect(kind SoundEffectKind, delta float64)

	Play__0(name SoundName)
	Play__1(name SoundName, loop bool)

	PlayWith(name SoundName, __xgo_optional_loop bool)
	PlayAndWait(name SoundName)
	PausePlaying(name SoundName)
	ResumePlaying(name SoundName)
	StopPlaying(name SoundName)

	// Physics Methods
	SetPhysicsMode(mode PhysicsMode)
	PhysicsMode() PhysicsMode
	SetVelocity(velocityX, velocityY float64)
	Velocity() (velocityX, velocityY float64)
	SetGravity(gravity float64)
	Gravity() float64
	AddImpulse(impulseX, impulseY float64)
	IsOnFloor() bool

	// Collider Methods
	SetColliderShape(isTrigger bool, ctype ColliderShapeType, params []float64) error
	ColliderShape(isTrigger bool) (ColliderShapeType, []float64)
	SetColliderPivot(isTrigger bool, offsetX, offsetY float64)
	ColliderPivot(isTrigger bool) (offsetX, offsetY float64)

	// Collision Methods
	SetCollisionLayer(layer int64)
	SetCollisionMask(mask int64)
	SetCollisionEnabled(enabled bool)
	CollisionLayer() int64
	CollisionMask() int64
	CollisionEnabled() bool

	// Trigger Methods
	SetTriggerEnabled(trigger bool)
	SetTriggerLayer(layer int64)
	SetTriggerMask(mask int64)
	TriggerLayer() int64
	TriggerMask() int64
	TriggerEnabled() bool
}
