package spx

import (
	"fmt"
	"log"

	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type FreeCamera struct {
	viewPort    *math32.Vector2
	position    *math32.Vector2
	zoom        *math32.Vector2
	rotation    float64
	worldMatrix ebiten.GeoM
}

func (cam *FreeCamera) init(viewPort *math32.Vector2) {

	cam.viewPort = viewPort
	cam.position = math32.NewVector2(0, 0)
	cam.zoom = math32.NewVector2(1, 1)
	cam.rotation = 0

	cam.updateMatrix()
	return
}

func (c *FreeCamera) String() string {
	return fmt.Sprintf(
		"T: %.2f, R: %.2f, S: %.2f, ViewPort: %.2f",
		c.position, c.rotation, c.zoom, c.viewPort,
	)
}

func (c *FreeCamera) viewportCenter() *math32.Vector2 {
	return c.viewPort.Scale(0.5)
}

func (c *FreeCamera) updateMatrix() {
	c.worldMatrix.Reset()

	c.worldMatrix.Translate(c.position.Inverted().Coords())
	c.worldMatrix.Translate(c.viewportCenter().Inverted().Coords())
	c.worldMatrix.Scale(c.zoom.Coords())
	c.worldMatrix.Rotate(c.rotation)
	c.worldMatrix.Translate(c.viewportCenter().Coords())
}

func (c *FreeCamera) Render(world, screen *ebiten.Image) error {
	options := &ebiten.DrawImageOptions{
		GeoM: c.worldMatrix,
	}
	screen.DrawImage(world, options)
	return nil
}

func (c *FreeCamera) ScreenToWorld(point *math32.Vector2) *math32.Vector2 {
	inverseMatrix := c.worldMatrix
	inverseMatrix.Invert()
	return math32.NewVector2(inverseMatrix.Apply(point.Coords()))
}

func (c *FreeCamera) WorldToScreen(point *math32.Vector2) *math32.Vector2 {
	return math32.NewVector2(c.worldMatrix.Apply(point.Coords()))
}

func (c *FreeCamera) Move(x, y int) {
	p := &math32.Vector2{X: float64(x), Y: float64(y)}
	c.position = c.position.Add(p)
	log.Printf("Camera Move %s", c.position.String())
	c.updateMatrix()
}

func (c *FreeCamera) MoveTo(x, y int) {
	c.position = &math32.Vector2{X: float64(x), Y: float64(y)}
	c.updateMatrix()
}

func (c *FreeCamera) FocusOn(x, y int) {
	pos := &math32.Vector2{X: float64(x), Y: float64(y)}
	c.position = pos.Clone().Sub(c.viewportCenter())
	c.updateMatrix()
}

func (c *FreeCamera) Zoom(m float64) {
	c.zoom = c.zoom.Scale(m)
	c.updateMatrix()
}

func (c *FreeCamera) Rotate(theta float64) {
	c.rotation += theta
	c.updateMatrix()
}

func (c *FreeCamera) Reset() {
	c.rotation = 0
	c.zoom.Set(1, 1)
	c.updateMatrix()
}
