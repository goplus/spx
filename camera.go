package spx

import (
	"github.com/goplus/spx/internal/camera"
)

type Camera struct {
	camera.FreeCamera
	g *Game
}

func NewCamera(g *Game, winW, winH float64, worldW, worldH float64) *Camera {
	c := &Camera{}
	c.g = g
	c.FreeCamera = *camera.NewFreeCamera(winW, winH, worldW, worldH)
	return c
}
func (c *Camera) GetXY() (float64, float64) {
	cx := c.GetPos().X
	cy := c.GetPos().Y
	return cx, cy
}
func (c *Camera) SetXY(x float64, y float64) {
	c.MoveTo(x, y)
}
func (c *Camera) Move(x float64, y float64) {
	c.Move(x, y)
}
func (c *Camera) On(obj interface{}) {
	switch v := obj.(type) {
	case string:
		if sp := c.g.findSprite(v); sp != nil {
			cx, cy := sp.getXY()
			c.MoveTo(cx, cy)
			return
		}
		panic("cameraOn: sprite not found - " + v)
	case specialObj:
		if v == Mouse {
			cx := c.g.MouseX()
			cy := c.g.MouseY()
			c.MoveTo(cx, cy)
			return
		}
	case Spriter:
		cx, cy := spriteOf(v).getXY()
		c.MoveTo(cx, cy)
		return
	}
	panic("cameraOn: unexpected input")
}
