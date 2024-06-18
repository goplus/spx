package camera

import (
	"fmt"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
)

type FreeCamera struct {
	viewPort           *math32.Vector2
	worldSize          *math32.Vector2
	position           *math32.Vector2
	zoom               *math32.Vector2
	rotation           float64
	world2ScreenMatrix ebiten.GeoM
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
	cam.updateMatrix()
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
	renderScale := engine.GetRenderScale()
	limit := c.worldSize.Scale(renderScale).Sub(c.viewPort).Scale(0.5)
	cx := c.position.X
	cy := -c.position.Y
	cx = math32.Clamp(cx, -limit.X, limit.X)
	cy = math32.Clamp(cy, -limit.Y, limit.Y)
	pos := &math32.Vector2{
		X: cx,
		Y: cy,
	}
	c.position.X = pos.X
	c.position.Y = -pos.Y

	c.world2ScreenMatrix.Reset()
	c.world2ScreenMatrix.Translate(c.position.Inverted().Coords())   // convert to camera's local space
	c.world2ScreenMatrix.Scale(renderScale, renderScale)             // convert to render space
	c.world2ScreenMatrix.Scale(1, -1)                                // invert Y // ebiten's (0,0) is top left corner
	c.world2ScreenMatrix.Translate((c.viewPort).Scale(0.5).Coords()) // move the pose to cartesian coordinates //(0,0) is left bottom corner
	// TODO @tanjp support rotation and scale
}

func (c *FreeCamera) Render(world, screen *ebiten.Image) error {

	options := &ebiten.DrawImageOptions{
		//GeoM: c.world2ScreenMatrix, // TODO apply a world2renderspace  matrix
	}
	screen.DrawImage(world, options)
	return nil
}

func (c *FreeCamera) ScreenToWorld(point *math32.Vector2) *math32.Vector2 {
	c.updateMatrix()
	inverseMatrix := c.world2ScreenMatrix
	inverseMatrix.Invert()
	return math32.NewVector2(inverseMatrix.Apply(point.Coords()))
}

func (c *FreeCamera) WorldToScreen(point *math32.Vector2) *math32.Vector2 {
	return math32.NewVector2(c.world2ScreenMatrix.Apply(point.Coords()))
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

func (c *FreeCamera) IsWorldRange(pos *math32.Vector2) bool {
	if pos.X >= -c.worldSize.X/2.0 && pos.X <= c.worldSize.X/2.0 && pos.Y >= -c.worldSize.Y/2.0 && pos.Y <= c.worldSize.Y/2.0 {
		return true
	}
	return false
}
