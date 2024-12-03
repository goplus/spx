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

	"github.com/goplus/spx/internal/camera"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type Camera struct {
	freecamera camera.FreeCamera
	g          *Game
	on_        interface{}
}

func (c *Camera) init(g *Game, winW, winH float64, worldW, worldH float64) {
	c.freecamera = *camera.NewFreeCamera(winW, winH, worldW, worldH)
	c.g = g
}

func (c *Camera) isWorldRange(pos *math32.Vector2) bool {
	return c.freecamera.IsWorldRange(pos)
}

func (c *Camera) SetXYpos(x float64, y float64) {
	c.on_ = nil
	c.freecamera.MoveTo(x, y)
}

func (c *Camera) ChangeXYpos(x float64, y float64) {
	c.on_ = nil
	c.freecamera.Move(x, y)
}

func (c *Camera) screenToWorld(point *math32.Vector2) *math32.Vector2 {
	return c.freecamera.ScreenToWorld(point)
}

/* unused:
func (c *Camera) worldToScreen(point *math32.Vector2) *math32.Vector2 {
	return c.freecamera.WorldToScreen(point)
}
*/

func (c *Camera) on(obj interface{}) {
	switch v := obj.(type) {
	case SpriteName:
		sp := c.g.findSprite(v)
		if sp == nil {
			log.Println("Camera.On: sprite not found -", v)
			return
		}
		obj = sp
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

func (c *Camera) updateOnObj() {
	switch v := c.on_.(type) {
	case *SpriteImpl:
		cx, cy := v.getXY()
		c.freecamera.MoveTo(cx, cy)
	case nil:
	case specialObj:
		cx := c.g.MouseX()
		cy := c.g.MouseY()
		c.freecamera.MoveTo(cx, cy)
	}
}

func (c *Camera) render(world, screen *ebiten.Image) error {
	c.updateOnObj()
	c.freecamera.Render(world, screen)
	return nil
}
