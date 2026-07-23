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
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
)

// -----------------------------------------------------------------------------
// Shared Types
// -----------------------------------------------------------------------------
type engineManagers = enginewrap.EngineManagers

type threadObj = coroutine.ThreadObj

type event any

// -----------------------------------------------------------------------------
// Event Payloads
// -----------------------------------------------------------------------------
type eventStart struct{}

type eventKeyDown struct {
	Key Key
}

type eventLeftButtonDown struct {
	Pos mathf.Vec2
}

type eventLeftButtonUp struct {
	Pos mathf.Vec2
}

type eventMouseMove struct {
	Pos mathf.Vec2
}

type eventTimer struct {
	Time float64
}

var (
	cachedBounds map[string]mathf.Rect2
)

// -----------------------------------------------------------------------------
// Engine Access
// -----------------------------------------------------------------------------
func (p *Game) engine() *engineManagers {
	return &p.engineMgr
}

func (p *SpriteImpl) engine() *engineManagers {
	return p.g.engine()
}

func (c *componentBase) engine() *engineManagers {
	return c.sprite.engine()
}

// -----------------------------------------------------------------------------
// Runtime Helpers
// -----------------------------------------------------------------------------
func nameOf(this any) string {
	if spr, ok := this.(*SpriteImpl); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	engine.Panic("scriptEventBindings: unexpected this object")
	return ""
}

func isGame(obj threadObj) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj threadObj) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}
