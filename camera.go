/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"log"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/math32"
)

type Camera struct {
	g   *Game
	on_ interface{}
}

func (c *Camera) init(g *Game, winW, winH float64, worldW, worldH float64) {
	c.g = g
}

func (c *Camera) isWorldRange(pos *math32.Vector2) bool {
	rect := engine.SyncCameraGetViewportRect()
	if pos.X < float64(rect.Position.X-rect.Size.X/2) || pos.X > float64(rect.Position.X+rect.Size.X) {
		return false
	}
	if pos.Y < float64(rect.Position.Y-rect.Size.Y/2) || pos.Y > float64(rect.Position.Y+rect.Size.Y) {
		return false
	}
	return true
}

func (c *Camera) SetXYpos(x float64, y float64) {
	c.ChangeXYpos(x, y)
}

func (c *Camera) ChangeXYpos(x float64, y float64) {
	c.on_ = nil
	engine.SyncCameraSetCameraPosition(engine.NewVec2(x, y))
}

func (c *Camera) screenToWorld(point *math32.Vector2) *math32.Vector2 {
	return point // TODO tanjp
}

func (c *Camera) getFollowPos() (bool, float64, float64) {
	if c.on_ != nil {
		switch v := c.on_.(type) {
		case SpriteImpl:
			return true, v.x, v.y
		}
	}
	return false, 0, 0
}
func (c *Camera) On(obj interface{}) {
	switch v := obj.(type) {
	case string:
		sp := c.g.findSprite(v)
		if sp == nil {
			log.Println("Camera.On: sprite not found -", v)
			return
		}
		obj = sp
		println("Camera.On: sprite found -", sp.name)
	case *SpriteImpl:
	case nil:
	case Sprite:
		obj = spriteOf(v)
	case specialObj:
		if v != Mouse {
			log.Println("Camera.On: not support -", v)
			return
		}
	default:
		panic("Camera.On: unexpected parameter")
	}
	c.on_ = obj
}
