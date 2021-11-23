package camera

import (
	"fmt"

	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type FreeCamera struct {
	viewPort    *math32.Vector2
	worldSize   *math32.Vector2
	position    *math32.Vector2
	zoom        *math32.Vector2
	rotation    float64
	worldMatrix ebiten.GeoM
}

func NewFreeCamera(winW, winH float64, worldW, worldH float64) *FreeCamera {
	cam := &FreeCamera{}
	cam.init(&math32.Vector2{
		X: winW,
		Y: winH,
	}, &math32.Vector2{
		X: worldW,
		Y: worldH,
	})
	return cam
}

func (cam *FreeCamera) init(viewPort *math32.Vector2, worldSize *math32.Vector2) {

	cam.viewPort = viewPort
	cam.worldSize = worldSize
	cam.position = math32.NewVector2(0, 0)
	cam.zoom = math32.NewVector2(1, 1)
	cam.rotation = 0

	return
}

func (c *FreeCamera) String() string {
	return fmt.Sprintf(
		"T: %.2f, R: %.2f, S: %.2f, ViewPort: %.2f",
		c.position, c.rotation, c.zoom, c.viewPort,
	)
}

func (c *FreeCamera) SetViewPort(width, height float64) {
	c.viewPort.X = width
	c.viewPort.Y = height
}

func (c *FreeCamera) CameraCenter() *math32.Vector2 {
	return c.worldSize.Scale(0.5)
}

func (c *FreeCamera) updateMatrix() {
	c.worldMatrix.Reset()

	limit := c.worldSize.Sub(c.viewPort).Scale(0.5)
	cx := c.position.X
	cy := -c.position.Y
	cx = math32.Clamp(cx, -limit.X, limit.X)
	cy = math32.Clamp(cy, -limit.Y, limit.Y)
	pos := &math32.Vector2{
		X: cx,
		Y: cy,
	}

	c.worldMatrix.Translate(c.worldSize.Sub(c.viewPort).Scale(0.5).Inverted().Coords())
	c.worldMatrix.Translate(pos.Inverted().Coords())

	c.worldMatrix.Translate(c.CameraCenter().Inverted().Coords())
	c.worldMatrix.Scale(c.zoom.Coords())
	c.worldMatrix.Rotate(c.rotation)
	c.worldMatrix.Translate(c.CameraCenter().Coords())
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

func (c *FreeCamera) GetPos() *math32.Vector2 {
	return c.position
}

func (c *FreeCamera) Move(x, y float64) {
	p := &math32.Vector2{X: x, Y: y}
	c.position = c.position.Add(p)
	c.updateMatrix()
}

func (c *FreeCamera) MoveTo(x, y float64) {
	c.position = &math32.Vector2{X: x, Y: y}
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
