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
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
)

// Config configures game startup and runtime behavior.
type Config = coreproject.Config

// Shape is the common runtime item type stored by the game.
type Shape any

// Event types.
type (
	// BackdropName identifies a stage backdrop.
	//
	//xgo:class:resource backdrop
	//xgo:class:resource-discovery backdrops.*
	BackdropName = coreevent.BackdropName

	// Direction represents a heading or swipe direction.
	Direction = coreevent.Direction

	// IEventSinks exposes script event binding helpers.
	IEventSinks = coreevent.IEventSinks

	// Key represents a keyboard key code.
	Key = coreevent.Key

	// StopKind controls how script execution is stopped.
	StopKind = coreevent.StopKind

	// MsgName identifies a broadcast message.
	MsgName = coreevent.MsgName
)

// Name types.
type (
	// SoundName identifies a registered sound asset.
	//
	//xgo:class:resource sound
	//xgo:class:resource-discovery sounds.*
	SoundName = string

	// SpriteName identifies a sprite instance or prototype by name.
	//
	//xgo:class:resource sprite
	//xgo:class:resource-discovery sprites.*
	SpriteName = string

	// SpriteCostumeName identifies a sprite costume by name.
	//
	//xgo:class:resource sprite.costume
	//xgo:class:resource-discovery costumes.*
	SpriteCostumeName = string

	// SpriteAnimationName identifies a sprite animation by name.
	//
	//xgo:class:resource sprite.animation
	//xgo:class:resource-discovery fAnimations.*
	SpriteAnimationName = string

	// WidgetName identifies a widget by name.
	//
	//xgo:class:resource widget
	//xgo:class:resource-discovery zorder.*@($type == "monitor" || $type == "stageMonitor")
	WidgetName = string
)

// Widget is the common interface for runtime UI objects stored by the game.
//
//xgo:class:resource widget
type Widget interface {
	GetName() WidgetName
	Visible() bool
	Show()
	Hide()

	Xpos() float64
	Ypos() float64
	SetXpos(x float64)
	SetYpos(y float64)
	SetXYpos(x float64, y float64)
	ChangeXpos(dx float64)
	ChangeYpos(dy float64)
	ChangeXYpos(dx float64, dy float64)

	Size() float64
	SetSize(size float64)
	ChangeSize(delta float64)
}

// ShapeGetter exposes the runtime shape collection used by shared helpers.
type ShapeGetter interface {
	getAllShapes() []Shape
}

// Gamer is the runtime entry interface expected by the engine.
type Gamer interface {
	engine.IGame
	initGame(sprites []Sprite) *Game
}
