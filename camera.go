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

	"github.com/realdream-ai/mathf"
)

type Camera struct {
	g   *Game
	on_ interface{}
}

func (c *Camera) init(g *Game) {
	c.g = g
}
func (c *Camera) SetCameraZoom(scale float64) {
	cameraMgr.SetCameraZoom(mathf.NewVec2(scale, scale))
}

func (c *Camera) GetXYpos() (float64, float64) {
	pos := cameraMgr.GetPosition()
	return pos.X, pos.Y
}

func (c *Camera) SetXYpos(x float64, y float64) {
	cameraMgr.SetPosition(mathf.NewVec2(x, y))
}

func (c *Camera) ChangeXYpos(x float64, y float64) {
	c.on_ = nil
	posX, posY := c.GetXYpos()
	c.SetXYpos(posX+x, posY+y)
}

func (c *Camera) getFollowPos() (bool, mathf.Vec2) {
	if c.on_ != nil {
		switch v := c.on_.(type) {
		case SpriteImpl:
			return true, mathf.NewVec2(v.x, v.y)
		}
	}
	return false, mathf.NewVec2(0, 0)
}
func (c *Camera) on(obj interface{}) {
	switch v := obj.(type) {
	case SpriteName:
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

func (c *Camera) On__0(sprite Sprite) {
	c.on(sprite)
}

func (c *Camera) On__1(sprite *SpriteImpl) {
	c.on(sprite)
}

func (c *Camera) On__2(sprite SpriteName) {
	c.on(sprite)
}

func (c *Camera) On__3(obj specialObj) {
	c.on(obj)
}
