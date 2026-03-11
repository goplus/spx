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

// Gamer is the runtime entry interface expected by the engine.
type Gamer interface {
	engine.IGame
	initGame(sprites []Sprite) *Game
}

// BackdropName identifies a stage backdrop.
type BackdropName = coreevent.BackdropName

// SoundName identifies a registered sound asset.
type SoundName = string
