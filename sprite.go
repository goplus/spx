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
	coreevent "github.com/goplus/spx/v4/internal/core/event"
	"github.com/goplus/spx/v4/internal/core/scratch"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------
type RotationStyle int

// DragMode controls whether a sprite can be dragged with the mouse.
type DragMode int

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
	NotDraggable DragMode = iota
	Draggable
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
	colorThreshold = 0.035
	alphaThreshold = 0.007
)

const (
	AllStop              StopKind = coreevent.AllStop
	AllOtherScripts      StopKind = coreevent.AllOtherScripts
	AllSprites           StopKind = coreevent.AllSprites
	ThisSprite           StopKind = coreevent.ThisSprite
	ThisScript           StopKind = coreevent.ThisScript
	OtherScriptsInSprite StopKind = coreevent.OtherScriptsInSprite
	OtherScriptsInGame   StopKind = coreevent.OtherScriptsInGame
)

// VarName identifies a monitor variable by name.
type VarName = string

// Pos identifies a list index or a special motion target.
type Pos = int

const (
	Invalid Pos = scratch.ListInvalid
	Last    Pos = scratch.ListLast
	All         = scratch.ListAll // Pos or StopKind
	Random  Pos = scratch.ListRandom
)

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

// XGo method overloads shared by Sprite and SpriteImpl.
const (
	XGoo_Sprite_GlideWith    = ".GlideToTarget,.GlideToXYpos"
	XGoo_Sprite_StepToWith   = ".StepToTarget,.StepToXYpos"
	XGoo_Sprite_TurnToWith   = ".TurnToDir,.TurnToTarget,.TurnToXYpos"
	XGoo_Sprite_SetLayerWith = ".SetLayerTo,.SetLayer__1"
	XGoo_Sprite_QuoteWith    = ".QuoteMsg,.QuoteMsgEx"

	XGoo_SpriteImpl_GlideWith    = XGoo_Sprite_GlideWith
	XGoo_SpriteImpl_StepToWith   = XGoo_Sprite_StepToWith
	XGoo_SpriteImpl_TurnToWith   = XGoo_Sprite_TurnToWith
	XGoo_SpriteImpl_SetLayerWith = XGoo_Sprite_SetLayerWith
	XGoo_SpriteImpl_QuoteWith    = XGoo_Sprite_QuoteWith
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
	ChangeXpos(dX float64)
	ChangeYpos(dY float64)
	ChangeXYpos(dX, dY float64)
	SetDragMode(mode DragMode)

	// Movement Methods
	Step__0(distance float64)
	Step__1(distance float64, speed Speed)
	Step__2(distance float64, speed Speed, animation SpriteAnimationName)

	StepWith(distance float64, __xgo_optional_opts *MotionOptions)

	StepTo__0(sprite Sprite)
	StepTo__1(name SpriteName)
	StepTo__2(object specialObj)
	StepTo__3(x, y float64)
	StepTo__4(sprite Sprite, speed Speed)
	StepTo__5(name SpriteName, speed Speed)
	StepTo__6(object specialObj, speed Speed)
	StepTo__7(x, y, speed Speed)
	StepTo__8(sprite Sprite, speed Speed, animation SpriteAnimationName)
	StepTo__9(name SpriteName, speed Speed, animation SpriteAnimationName)
	StepTo__a(object specialObj, speed Speed, animation SpriteAnimationName)
	StepTo__b(x, y, speed Speed, animation SpriteAnimationName)
	StepTo__c(pos Pos)
	StepTo__d(pos Pos, speed Speed)
	StepTo__e(pos Pos, speed Speed, animation SpriteAnimationName)

	StepToTarget(target Target, __xgo_optional_opts *MotionOptions)
	StepToXYpos(x, y float64, __xgo_optional_opts *MotionOptions)

	Glide__0(sprite Sprite, seconds Seconds)
	Glide__1(name SpriteName, seconds Seconds)
	Glide__2(object specialObj, seconds Seconds)
	Glide__3(pos Pos, seconds Seconds)
	Glide__4(x, y float64, seconds Seconds)

	GlideToTarget(target Target, secs Seconds)
	GlideToXYpos(x, y float64, secs Seconds)

	// Heading and Rotation Methods
	Heading() Direction
	SetHeading(direction Direction)
	ChangeHeading(degree Direction)
	SetRotationStyle(style RotationStyle)

	Turn__0(direction Direction)
	Turn__1(direction Direction, speed Speed)
	Turn__2(direction Direction, speed Speed, animation SpriteAnimationName)

	TurnWith(direction Direction, __xgo_optional_opts *MotionOptions)

	TurnTo__0(sprite Sprite)
	TurnTo__1(name SpriteName)
	TurnTo__2(direction Direction)
	TurnTo__3(object specialObj)
	TurnTo__4(sprite Sprite, speed Speed)
	TurnTo__5(name SpriteName, speed Speed)
	TurnTo__6(direction Direction, speed Speed)
	TurnTo__7(object specialObj, speed Speed)
	TurnTo__8(sprite Sprite, speed Speed, animation SpriteAnimationName)
	TurnTo__9(name SpriteName, speed Speed, animation SpriteAnimationName)
	TurnTo__a(direction Direction, speed Speed, animation SpriteAnimationName)
	TurnTo__b(object specialObj, speed Speed, animation SpriteAnimationName)

	TurnToDir(dir Direction, __xgo_optional_opts *MotionOptions)
	TurnToTarget(target Target, __xgo_optional_opts *MotionOptions)
	TurnToXYpos(x, y float64, __xgo_optional_opts *MotionOptions)

	BounceOffEdge()

	// Size Methods
	Size() float64
	SetSize(size float64)
	ChangeSize(dSize float64)

	// Layer Methods
	SetLayer__0(layer layerAction)
	SetLayer__1(direction dirAction, layers int)

	SetLayerTo(layer layerAction)

	// Costume Methods
	CostumeName() SpriteCostumeName
	CostumeIndex() int

	// ResolveCostumeIndex resolves a costume name or 1-based integer string to an index.
	//
	// Deprecated: Use [github.com/goplus/spx/v4/pkg/scratchcompat.ResolveCostumeIndex] instead.
	ResolveCostumeIndex(costume string) int

	SetCostume__0(name SpriteCostumeName)
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
	SetGraphicEffect(kind EffectKind, value float64)
	ChangeGraphicEffect(kind EffectKind, value float64)
	ClearGraphicEffects()

	// Pen Methods
	PenDown()
	PenUp()

	SetPenColor__0(color Color)
	SetPenColor__1(kind PenColorParam, value float64)

	ChangePenColor(kind PenColorParam, delta float64)
	SetPenHue(value float64)
	ChangePenHue(delta float64)
	SetPenShade(value float64)
	ChangePenShade(delta float64)
	SetPenSize(size float64)
	ChangePenSize(delta float64)
	Stamp()

	// Distance and Detection Methods
	DistanceTo__0(sprite Sprite) float64
	DistanceTo__1(name SpriteName) float64
	DistanceTo__2(object specialObj) float64
	DistanceTo__3(pos Pos) float64

	DistanceToWith(target Target) float64

	DirectionTo__0(sprite Sprite) Direction
	DirectionTo__1(sprite SpriteName) Direction
	DirectionTo__2(target specialObj) Direction
	DirectionTo__3(pos Pos) Direction
	DirectionTo__4(x, y float64) Direction

	DirectionToWith(target Target) Direction

	Touching__0(sprite Sprite) bool
	Touching__1(name SpriteName) bool
	Touching__2(object specialObj) bool

	TouchingWith(target Target) bool
	TouchingColor__0(color Color) bool
	TouchingColor__1(spriteColor, targetColor Color) bool

	// Communication Methods
	Say__0(word any)
	Say__1(word any, seconds Seconds)

	SayWith(word any, __xgo_optional_seconds Seconds)

	Think__0(word any)
	Think__1(word any, seconds Seconds)

	ThinkWith(word any, __xgo_optional_seconds Seconds)

	Ask(question any)

	Quote__0(message string)
	Quote__1(message string, secs Seconds)
	Quote__2(message, description string)
	Quote__3(message, description string, secs Seconds)

	QuoteMsg(message string, __xgo_optional_secs Seconds)
	QuoteMsgEx(message, description string, __xgo_optional_secs Seconds)

	// Event Methods
	OnCloned__0(onCloned func())
	OnCloned__1(onCloned func(data any))

	OnTouchStart__0(name SpriteName, onTouchStart func())
	OnTouchStart__1(name SpriteName, onTouchStart func(Sprite))
	OnTouchStart__2(names []SpriteName, onTouchStart func())
	OnTouchStart__3(names []SpriteName, onTouchStart func(Sprite))

	// Sound Methods
	Volume() float64
	SetVolume(volume float64)
	ChangeVolume(dVolume float64)
	GetSoundEffect(kind SoundEffectKind) float64
	SetSoundEffect(kind SoundEffectKind, value float64)
	ChangeSoundEffect(kind SoundEffectKind, delta float64)
	ClearSoundEffects()

	Play__0(sound SoundName)
	Play__1(sound SoundName, loop bool)

	PlayWith(sound SoundName, __xgo_optional_loop bool)
	PlayAndWait(sound SoundName)
	PausePlaying(sound SoundName)
	ResumePlaying(sound SoundName)
	StopPlaying(sound SoundName)

	// Physics Methods
	SetPhysicsMode(mode PhysicsMode)
	PhysicsMode() PhysicsMode
	SetVelocity(vx, vy float64)
	Velocity() (velocityX, velocityY float64)
	SetGravity(g float64)
	Gravity() float64
	// AddImpulse adds impulse / mass to velocity on the next dynamic physics tick.
	AddImpulse(ix, iy float64)
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
