package spx

import (
	"github.com/goplus/spx/internal/camera"
	"github.com/hajimehoshi/ebiten/v2"
)

type Camera struct {
	freecamera camera.FreeCamera
	g          *Game
	on_        interface{}
}

func newCamera(g *Game, winW, winH float64, worldW, worldH float64) *Camera {
	c := &Camera{}
	c.g = g
	c.freecamera = *camera.NewFreeCamera(winW, winH, worldW, worldH)
	return c
}

func (c *Camera) MoveTo(x float64, y float64) {
	c.on_ = nil
	c.freecamera.MoveTo(x, y)
}
func (c *Camera) Move(x float64, y float64) {
	c.on_ = nil
	c.freecamera.Move(x, y)
}

func (c *Camera) On(obj interface{}) {
	c.on_ = obj
}
func (c *Camera) updateOnObj() {
	if c.on_ == nil {
		return
	}
	switch v := c.on_.(type) {
	case string:
		if sp := c.g.findSprite(v); sp != nil {
			cx, cy := sp.getXY()
			c.freecamera.MoveTo(cx, cy)
			return
		}
	case specialObj:
		if v == Mouse {
			cx := c.g.MouseX()
			cy := c.g.MouseY()
			c.freecamera.MoveTo(cx, cy)
			return
		}
	case Spriter:
		cx, cy := spriteOf(v).getXY()
		c.freecamera.MoveTo(cx, cy)
		return
	}
	return
}

func (c *Camera) Render(world, screen *ebiten.Image) error {
	c.updateOnObj()
	c.freecamera.Render(world, screen)
	return nil
}
