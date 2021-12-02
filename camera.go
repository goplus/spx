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

func (c *Camera) worldToScreen(point *math32.Vector2) *math32.Vector2 {
	return c.freecamera.WorldToScreen(point)
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
	case *Sprite:
	case nil:
	case Spriter:
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

func (c *Camera) updateOnObj() {
	switch v := c.on_.(type) {
	case *Sprite:
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
