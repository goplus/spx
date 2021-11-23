package spx

import (
	"image"

	"github.com/goplus/spx/internal/math32"

	"github.com/hajimehoshi/ebiten/v2"

	spxfs "github.com/goplus/spx/fs"
)

// -------------------------------------------------------------------------------------

type drawContext struct {
	*ebiten.Image
}

type hitContext struct {
	Pos image.Point
}

type hitResult struct {
	Target interface{}
}

type Shape interface {
	draw(dc drawContext)
	hit(hc hitContext) (hr hitResult, ok bool)
}

// -------------------------------------------------------------------------------------

type spriteDrawInfo struct {
	sprite  *Sprite
	visible bool
}

func (p *spriteDrawInfo) drawOn(dc drawContext, fs spxfs.Dir) {
	p.doDrawOn(dc, fs)
}

func (p *spriteDrawInfo) draw(dc drawContext, ctx *Sprite) {
	p.doDrawOn(dc, ctx.g.fs)
}

func (p *spriteDrawInfo) doDrawOn(dc drawContext, fs spxfs.Dir) {
	c := p.sprite.costumes[p.sprite.currentCostumeIndex]

	img, centerX, centerY := c.needImage(fs)
	rect := image.Rectangle{}
	rect.Min.X = 0
	rect.Min.Y = 0
	rect.Max = img.Bounds().Size()

	x := p.sprite.x
	y := p.sprite.y

	scale := p.sprite.scale / float64(c.bitmapResolution)
	worldW, wolrdH := dc.Size()

	op := new(ebiten.DrawImageOptions)
	op.Filter = ebiten.FilterLinear
	geo := &op.GeoM

	direction := p.sprite.direction + c.faceRight
	if direction == 90 {
		x = float64(worldW>>1) + x - centerX*scale
		y = float64(wolrdH>>1) - y - centerY*scale
		if scale != 1 {
			geo.Scale(scale, scale)
		}
		geo.Translate(x, y)
	} else {
		geo.Translate(-centerX, -centerY)
		if scale != 1 {
			geo.Scale(scale, scale)
		}
		geo.Rotate(toRadian(direction - 90))
		geo.Translate(float64(worldW>>1)+x, float64(wolrdH>>1)-y)
	}
	geo2 := *geo
	geo2.Translate(-float64(worldW>>1), -float64(wolrdH>>1))
	p.sprite.rRect = math32.ApplyGeoForRotatedRect(rect, &geo2)

	dc.DrawImage(img, op)
}

func (p *Sprite) getDrawInfo() *spriteDrawInfo {
	return &spriteDrawInfo{
		sprite:  p,
		visible: p.isVisible,
	}
}

func (p *Sprite) getRotatedRect() (rRect *math32.RotatedRect) {
	di := p.getDrawInfo()
	if !di.visible {
		return
	}
	rRect = di.sprite.rRect
	return
}

func (p *Sprite) getTrackPos() (topx, topy int) {
	rRect := p.getRotatedRect()

	worldW, wolrdH := p.g.worldSize_()
	pos := &math32.Vector2{
		X: float64(rRect.Center.X) + float64(worldW)/2.0,
		Y: float64(rRect.Center.Y) + float64(wolrdH)/2.0,
	}

	return int(pos.X), int(pos.Y) - int(rRect.Size.Height)/2.0
}

func (p *Sprite) draw(dc drawContext) {
	di := p.getDrawInfo()
	if !di.visible {
		return
	}
	di.draw(dc, p)
}

// Hit func.
func (p *Sprite) hit(hc hitContext) (hr hitResult, ok bool) {
	rRect := p.getRotatedRect()
	if rRect == nil {
		return
	}
	worldW, wolrdH := p.g.worldSize_()
	pos := &math32.Vector2{
		X: float64(hc.Pos.X) - float64(worldW)/2.0,
		Y: float64(hc.Pos.Y) - float64(wolrdH)/2.0,
	}
	if rRect.Contains(pos) {
		return hitResult{Target: p}, true
	}

	return
}
