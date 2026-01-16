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

// ============================================================================
// Type Definitions
// ============================================================================

// Direction represents the heading direction in degrees
type Direction = float64

// RotationStyle defines how a sprite rotates
type RotationStyle int

// specialObj represents special target objects for sprite operations
type specialObj int

// Type aliases for sprite-related identifiers
type (
	SpriteName          = string
	SpriteCostumeName   = string
	SpriteAnimationName = string
)

// ============================================================================
// Direction Constants
// ============================================================================

const (
	Right Direction = 90
	Left  Direction = -90
	Up    Direction = 0
	Down  Direction = 180
)

// ============================================================================
// Special Object Constants
// ============================================================================

const (
	Mouse      specialObj = -5
	Edge       specialObj = touchingAllEdges
	EdgeLeft   specialObj = touchingScreenLeft
	EdgeTop    specialObj = touchingScreenTop
	EdgeRight  specialObj = touchingScreenRight
	EdgeBottom specialObj = touchingScreenBottom
)

// ============================================================================
// Rotation Style Constants
// ============================================================================

const (
	None RotationStyle = iota
	Normal
	LeftRight
)

// ============================================================================
// State Constants
// ============================================================================

const (
	StateDie   string = "die"
	StateTurn  string = "turn"
	StateGlide string = "glide"
	StateStep  string = "step"
)

// ============================================================================
// Animation Channel Constants
// ============================================================================

const (
	AnimChannelFrame string = "@frame"
	AnimChannelTurn  string = "@turn"
	AnimChannelGlide string = "@glide"
	AnimChannelMove  string = "@move"
)

// ============================================================================
// Internal Constants
// ============================================================================

const (
	colorThreshold = 0.1
	alphaThreshold = 0.05
)

const (
	touchingScreenLeft   = 1
	touchingScreenTop    = 2
	touchingScreenRight  = 4
	touchingScreenBottom = 8
	touchingAllEdges     = 15
)

// ============================================================================
// Sprite Interface
// ============================================================================

// Sprite defines the interface for all sprite objects in the game
type Sprite interface {
	IEventSinks
	Shape

	// Core Methods
	Main()
	Name() string
	IsCloned() bool
	DeleteThisClone()
	Destroy()
	Die()
	DeltaTime() float64
	TimeSinceLevelLoad() float64

	// Visibility Methods
	Hide()
	Show()
	Visible() bool
	HideVar(name string)
	ShowVar(name string)

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
	Step__1(step float64, speed float64)
	Step__2(step float64, speed float64, animation SpriteAnimationName)
	StepTo__0(sprite Sprite)
	StepTo__1(sprite SpriteName)
	StepTo__2(x, y float64)
	StepTo__3(obj specialObj)
	StepTo__4(sprite Sprite, speed float64)
	StepTo__5(sprite SpriteName, speed float64)
	StepTo__6(x, y, speed float64)
	StepTo__7(obj specialObj, speed float64)
	StepTo__8(sprite Sprite, speed float64, animation SpriteAnimationName)
	StepTo__9(sprite SpriteName, speed float64, animation SpriteAnimationName)
	StepTo__a(x, y, speed float64, animation SpriteAnimationName)
	StepTo__b(obj specialObj, speed float64, animation SpriteAnimationName)
	Glide__0(x, y float64, secs float64)
	Glide__1(sprite Sprite, secs float64)
	Glide__2(sprite SpriteName, secs float64)
	Glide__3(obj specialObj, secs float64)
	Glide__4(pos Pos, secs float64)

	// Heading and Rotation Methods
	Heading() Direction
	SetHeading(dir Direction)
	ChangeHeading(dir Direction)
	SetRotationStyle(style RotationStyle)
	Turn__0(dir Direction)
	Turn__1(dir Direction, speed float64)
	Turn__2(dir Direction, speed float64, animation SpriteAnimationName)
	TurnTo__0(target Sprite)
	TurnTo__1(target SpriteName)
	TurnTo__2(dir Direction)
	TurnTo__3(target specialObj)
	TurnTo__4(target Sprite, speed float64)
	TurnTo__5(target SpriteName, speed float64)
	TurnTo__6(dir Direction, speed float64)
	TurnTo__7(target specialObj, speed float64)
	TurnTo__8(target Sprite, speed float64, animation SpriteAnimationName)
	TurnTo__9(target SpriteName, speed float64, animation SpriteAnimationName)
	TurnTo__a(dir Direction, speed float64, animation SpriteAnimationName)
	TurnTo__b(target specialObj, speed float64, animation SpriteAnimationName)
	BounceOffEdge()

	// Size Methods
	Size() float64
	SetSize(size float64)
	ChangeSize(delta float64)

	// Layer Methods
	SetLayer__0(layer layerAction)
	SetLayer__1(dir dirAction, delta int)

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
	Touching__0(sprite SpriteName) bool
	Touching__1(sprite Sprite) bool
	Touching__2(obj specialObj) bool
	TouchingColor(color Color) bool

	// Communication Methods
	Say__0(msg any)
	Say__1(msg any, secs float64)
	Think__0(msg any)
	Think__1(msg any, secs float64)
	Ask(msg any)
	Quote__0(message string)
	Quote__1(message string, secs float64)
	Quote__2(message, description string)
	Quote__3(message, description string, secs float64)

	// Event Methods
	OnCloned__0(onCloned func(data any))
	OnCloned__1(onCloned func())
	OnTouchStart__0(sprite SpriteName, onTouchStart func(Sprite))
	OnTouchStart__1(sprite SpriteName, onTouchStart func())
	OnTouchStart__2(sprites []SpriteName, onTouchStart func(Sprite))
	OnTouchStart__3(sprites []SpriteName, onTouchStart func())

	// Sound Methods
	Volume() float64
	SetVolume(volume float64)
	ChangeVolume(delta float64)
	GetSoundEffect(kind SoundEffectKind) float64
	SetSoundEffect(kind SoundEffectKind, value float64)
	ChangeSoundEffect(kind SoundEffectKind, delta float64)
	Play__0(name SoundName, loop bool)
	Play__1(name SoundName)
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

// ============================================================================
// Helper Functions
// ============================================================================

// toRotationStyle converts a string representation to a RotationStyle constant
func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right":
		return LeftRight
	case "none":
		return None
	}
	return Normal
}
