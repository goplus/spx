package spx

import (
	"image"
	"image/color"
	"log"
	"reflect"

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
	sprite *Sprite

	visible bool
}

func (p *spriteDrawInfo) getGeo() *ebiten.GeoM {
	c := p.sprite.costumes[p.sprite.currentCostumeIndex]
	scale := p.sprite.scale / float64(c.bitmapResolution)
	direction := p.sprite.direction + c.faceRight
	geo := &ebiten.GeoM{}
	geo.Scale(scale, scale)
	geo.Rotate(toRadian(direction - 90))
	geo.Scale(1.0, -1.0)
	return geo
}
func (p *spriteDrawInfo) getPixel(pos *math32.Vector2, img *image.RGBA, geo *ebiten.GeoM, cx float64, cy float64) (color.Color, *math32.Vector2) {

	pos2 := math32.NewVector2(pos.X-p.sprite.x, pos.Y-p.sprite.y)
	x, y := geo.Apply(pos2.X, pos2.Y)
	x = x + cx
	y = y + cy
	pixelpos := math32.NewVector2(x, y)

	if x < 0 || y < 0 || x >= float64(img.Bounds().Size().X) || y >= float64(img.Bounds().Size().Y) {
		return color.Transparent, pixelpos
	}
	color := img.At(int(x), int(y))
	return color, pixelpos
}

func (p *spriteDrawInfo) drawOn(dc drawContext, fs spxfs.Dir) {
	p.doDrawOn(dc, fs)
}

func (p *spriteDrawInfo) draw(dc drawContext, ctx *Sprite) {
	p.doDrawOn(dc, ctx.g.fs)
}

func (p *spriteDrawInfo) doDrawOn(dc drawContext, fs spxfs.Dir) {
	if !p.visible {
		return
	}
	c := p.sprite.costumes[p.sprite.currentCostumeIndex]

	img, centerX, centerY := c.needImage(fs)
	rect := image.Rectangle{}
	rect.Min.X = 0
	rect.Min.Y = 0
	rect.Max = img.Bounds().Size()

	scale := p.sprite.scale / float64(c.bitmapResolution)
	worldW, wolrdH := p.sprite.g.worldSize_()

	op := new(ebiten.DrawImageOptions)
	op.Filter = ebiten.FilterLinear

	geo := ebiten.GeoM{}
	geo.Reset()
	direction := p.sprite.direction + c.faceRight

	geo.Translate(-centerX, -centerY)
	geo.Scale(scale, scale)
	geo.Rotate(toRadian(direction - 90))
	geo.Translate(p.sprite.x, -p.sprite.y)

	geo2 := geo
	geo2.Scale(1.0, -1.0)
	p.sprite.rRect = math32.ApplyGeoForRotatedRect(rect, &geo2)

	op.GeoM = geo
	op.GeoM.Translate(float64(worldW>>1), float64(wolrdH>>1))
	dc.DrawImage(img, op)
}

func (p *Sprite) getDrawInfo() *spriteDrawInfo {
	return &spriteDrawInfo{
		sprite:  p,
		visible: p.isVisible,
	}
}

func (p *Sprite) touchPoint(x, y float64) bool {
	rRect := p.getRotatedRect()
	if rRect == nil {
		return false
	}
	pos := &math32.Vector2{X: x, Y: y}
	ret := rRect.Contains(pos)
	if !ret {
		return false
	}
	geo := p.getDrawInfo().getGeo()

	c := p.costumes[p.currentCostumeIndex]
	img, cx, cy := c.needImageRGBA(p.g.fs)
	pixel, _ := p.getDrawInfo().getPixel(pos, img, geo, cx, cy)
	if reflect.DeepEqual(pixel, color.Transparent) {
		return false
	}
	if debugInstr {
		log.Printf("touchPoint pixel pos(%s) color(%v)", pos.String(), pixel)
	}
	return true
}
func (p *Sprite) touchRotatedRect(dstRect *math32.RotatedRect) bool {
	currRect := p.getRotatedRect()
	if currRect == nil {
		return false
	}
	ret := currRect.IsCollision(dstRect)
	if !ret {
		return false
	}

	//get bound rect
	currBoundRect := currRect.BoundingRect()
	dstRectBoundRect := dstRect.BoundingRect()
	boundRect := currBoundRect.Intersect(dstRectBoundRect)
	if debugInstr {
		log.Printf("touchRotatedRect  currBoundRect(%s) dstRectBoundRect(%s) boundRect(%s)",
			currBoundRect.String(), dstRectBoundRect.String(), boundRect.String())
	}

	geo := p.getDrawInfo().getGeo()

	c := p.costumes[p.currentCostumeIndex]
	img, cx, cy := c.needImageRGBA(p.g.fs)
	//check boun rect pixel
	for x := boundRect.X; x < boundRect.Width+boundRect.X; x++ {
		for y := boundRect.Y; y < boundRect.Height+boundRect.Y; y++ {
			color1, _ := p.getDrawInfo().getPixel(math32.NewVector2(x, y), img, geo, cx, cy)
			if !reflect.DeepEqual(color1, color.Transparent) {
				return true
			}
		}
	}
	return false
}
func (p *Sprite) touchingSprite(dst *Sprite) bool {
	currRect := p.getRotatedRect()
	if currRect == nil {
		return false
	}
	dstRect := dst.getRotatedRect()
	if dstRect == nil {
		return false
	}
	ret := currRect.IsCollision(dstRect)
	if !ret {
		return false
	}

	//get bound rect
	currBoundRect := currRect.BoundingRect()
	dstRectBoundRect := dstRect.BoundingRect()
	boundRect := currBoundRect.Intersect(dstRectBoundRect)
	if debugInstr {
		log.Printf("touchingSprite  currBoundRect(%s) dstRectBoundRect(%s) boundRect(%s)",
			currBoundRect.String(), dstRectBoundRect.String(), boundRect.String())
	}

	c := p.costumes[p.currentCostumeIndex]
	pimg, cx, cy := c.needImageRGBA(p.g.fs)
	geo := p.getDrawInfo().getGeo()

	c2 := dst.costumes[dst.currentCostumeIndex]
	dstimg, cx2, cy2 := c2.needImageRGBA(p.g.fs)
	geo2 := dst.getDrawInfo().getGeo()
	//check boun rect pixel
	for x := boundRect.X; x < boundRect.Width+boundRect.X; x++ {
		for y := boundRect.Y; y < boundRect.Height+boundRect.Y; y++ {
			pos := math32.NewVector2(x, y)
			color1, _ := p.getDrawInfo().getPixel(pos, pimg, geo, cx, cy)
			color2, _ := dst.getDrawInfo().getPixel(pos, dstimg, geo2, cx2, cy2)
			_, _, _, a1 := color1.RGBA()
			_, _, _, a2 := color2.RGBA()
			if a1 != 0 && a2 != 0 {
				return true
			}
		}
	}
	return false
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
	pos.Y = -pos.Y
	if !rRect.Contains(pos) {
		return
	}
	c2 := p.costumes[p.currentCostumeIndex]
	img, cx, cy := c2.needImageRGBA(p.g.fs)
	geo := p.getDrawInfo().getGeo()
	color1, pos := p.getDrawInfo().getPixel(pos, img, geo, cx, cy)
	if debugInstr {
		log.Printf("hit color1(%v) p(%s)", color1, pos)
	}
	_, _, _, a := color1.RGBA()
	if a == 0 {
		return
	}
	return hitResult{Target: p}, true
}
