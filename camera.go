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
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

type side int

const (
	sideLeft side = iota
	sideTop
	sideRight
	sideBottom
)

type Camera interface {
	ViewportRect() (float64, float64, float64, float64)
	SetZoom(scale float64)
	Zoom() float64
	Xpos() float64
	Ypos() float64
	SetXYpos(x float64, y float64)
	ChangeXYpos(x float64, y float64)
	Follow__0(sprite Sprite)
	Follow__1(sprite SpriteName)
}

type cameraImpl struct {
	g                     *Game
	followTarget          any
	isDirty               bool
	dirtyVersion          uint64
	observedFollowVersion uint64
}

func (c *cameraImpl) ViewportRect() (float64, float64, float64, float64) {
	rect := engine.Managers().CameraMgr.GetGlobalCameraRect()
	return rect.Position.X, rect.Position.Y, rect.Size.X, rect.Size.Y
}

func (c *cameraImpl) SetZoom(scale float64) {
	c.setDirtyFlag(true)
	scale *= c.g.displayState.WindowScale
	engine.Managers().CameraMgr.SetCameraZoom(engine.UniformVec2(scale))
}

func (c *cameraImpl) Zoom() float64 {
	scale := engine.Managers().CameraMgr.GetCameraZoom().X
	scale /= c.g.displayState.WindowScale
	return scale
}

func (c *cameraImpl) Xpos() float64 {
	pos := engine.Managers().CameraMgr.GetPosition()
	return pos.X
}

func (c *cameraImpl) Ypos() float64 {
	pos := engine.Managers().CameraMgr.GetPosition()
	return pos.Y
}

func (c *cameraImpl) SetXYpos(x float64, y float64) {
	c.setDirtyFlag(true)
	engine.Managers().CameraMgr.SetPosition(mathf.NewVec2(x, y))
}

func (c *cameraImpl) ChangeXYpos(x float64, y float64) {
	c.followTarget = nil
	posX, posY := c.Xpos(), c.Ypos()
	c.SetXYpos(posX+x, posY+y)
}

func (c *cameraImpl) Follow__0(sprite Sprite) {
	c.follow(sprite)
}

func (c *cameraImpl) Follow__1(sprite SpriteName) {
	c.follow(sprite)
}

func (c *cameraImpl) init(g *Game) {
	c.g = g
	c.SetZoom(1)
	c.setLimits()
}

// onUpdate updates the camera position to follow its target.
func (c *cameraImpl) onUpdate() {
	if c.followTarget == nil {
		return
	}
	shouldUpdate, pos := c.getFollowPos()

	if !shouldUpdate {
		return
	}

	c.setXYposDirect(pos.X, pos.Y)

	if sprite, ok := c.followTarget.(*SpriteImpl); ok {
		c.observedFollowVersion = sprite.spriteState.VisualVersion
	}
}

func (c *cameraImpl) setLimits() {
	p := c.g
	if p.displayState.WorldWidth <= 0 || p.displayState.WorldHeight <= 0 {
		return
	}

	worldLeft, worldTop, worldRight, worldBottom := p.worldBounds()
	world := map[side]int{
		sideLeft:   worldLeft,
		sideTop:    worldBottom,
		sideRight:  worldRight,
		sideBottom: worldTop,
	}

	// Apply camera limits
	for side, value := range world {
		engine.Managers().CameraMgr.SetCameraLimit(int64(side), int64(value))
	}

	// Enable smoothing
	engine.Managers().CameraMgr.SetCameraSmoothing(true)
}

func (c *cameraImpl) setXYposDirect(x float64, y float64) {
	c.setDirtyFlag(true)
	engine.BridgeSetCameraPosition(mathf.NewVec2(x, y))
}

func (c *cameraImpl) setDirtyFlag(isDirty bool) {
	c.isDirty = isDirty
	if isDirty {
		c.dirtyVersion++
	}
}

func (c *cameraImpl) getFollowPos() (bool, mathf.Vec2) {
	if c.followTarget != nil {
		switch v := c.followTarget.(type) {
		case *SpriteImpl:
			changed := c.observedFollowVersion != v.spriteState.VisualVersion
			return c.isDirty || changed, mathf.NewVec2(v.getXY())
		case specialObj:
			if c.followTarget == Mouse {
				return true, c.g.inputMgr.currentMousePos()
			}
		}
	}
	return false, mathf.NewVec2(0, 0)
}

func (c *cameraImpl) follow(obj any) {
	switch v := obj.(type) {
	case SpriteName:
		sp := c.g.shapeMgr.findSprite(v)
		if sp == nil {
			spxlog.Warn("Camera.Follow: sprite not found - %s", v)
			return
		}
		obj = sp
		spxlog.Debug("Camera.Follow: sprite found - %s", sp.name)
	case *SpriteImpl:
	case nil:
	case Sprite:
		obj = spriteOf(v)
		spxlog.Debug("Camera.Follow: obj - %s", obj.(*SpriteImpl).name)
	case specialObj:
		if v != Mouse {
			spxlog.Warn("Camera.Follow: not support - %v", v)
			return
		}
	default:
		panic("Camera.Follow: unexpected parameter")
	}
	c.followTarget = obj
	c.setDirtyFlag(true)
}
