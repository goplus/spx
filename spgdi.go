package spx

import (
	"image"
	"image/color"
	"log"
	"reflect"

	"github.com/goplus/spx/internal/gdi"
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
	geo     ebiten.GeoM
	visible bool
}

func (p *spriteDrawInfo) getPixelGeo(cx, cy float64) *ebiten.GeoM {
	c := p.sprite.costumes[p.sprite.currentCostumeIndex]
	scale := p.sprite.scale / float64(c.bitmapResolution)
	direction := p.sprite.direction + c.faceRight
	geo := &ebiten.GeoM{}
	geo.Scale(1.0/scale, 1.0/scale)
	geo.Rotate(toRadian(direction - 90))
	geo.Scale(1.0, -1.0)
	geo.Translate(cx, cy)

	return geo
}
func (p *spriteDrawInfo) getPixel(pos *math32.Vector2, gdiImg *gdi.SpxImage, geo *ebiten.GeoM) (color.Color, *math32.Vector2) {

	img := gdiImg.OriImg()
	pos2 := math32.NewVector2(pos.X-p.sprite.x, pos.Y-p.sprite.y)
	x, y := geo.Apply(pos2.X, pos2.Y)
	pixelpos := math32.NewVector2(x, y)

	if x < 0 || y < 0 || x >= float64(img.Bounds().Size().X) || y >= float64(img.Bounds().Size().Y) {
		return color.Transparent, pixelpos
	}
	point := img.Rect.Min
	color := img.At(point.X+int(x), point.Y+int(y))
	return color, pixelpos
}

func (p *spriteDrawInfo) drawOn(dc drawContext, fs spxfs.Dir) {
	p.doDrawOn(dc, fs)
}

func (p *spriteDrawInfo) draw(dc drawContext, ctx *Sprite) {
	p.doDrawOn(dc, ctx.g.fs)
}

func (p *spriteDrawInfo) updateMatrix() {
	c := p.sprite.costumes[p.sprite.currentCostumeIndex]

	img, centerX, centerY := c.needImage(p.sprite.g.fs)
	rect := image.Rectangle{}
	rect.Min.X = 0
	rect.Min.Y = 0
	rect.Max = img.Bounds().Size()

	scale := p.sprite.scale / float64(c.bitmapResolution)
	worldW, wolrdH := p.sprite.g.worldSize_()

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
	geo.Translate(float64(worldW>>1), float64(wolrdH>>1))
	p.geo = geo
}

func (p *spriteDrawInfo) doDrawOn(dc drawContext, fs spxfs.Dir) {
	if !p.visible {
		return
	}

	c := p.sprite.costumes[p.sprite.currentCostumeIndex]
	img, _, _ := c.needImage(fs)

	p.updateMatrix()

	op := new(ebiten.DrawImageOptions)
	op.Filter = ebiten.FilterLinear
	op.GeoM = p.geo
	dc.DrawImage(img.EbiImg(), op)
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
	c := p.costumes[p.currentCostumeIndex]
	img, cx, cy := c.needImage(p.g.fs)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)

	pixel, _ := p.getDrawInfo().getPixel(pos, img, geo)
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
	c := p.costumes[p.currentCostumeIndex]
	img, cx, cy := c.needImage(p.g.fs)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)

	//check boun rect pixel
	for x := boundRect.X; x < boundRect.Width+boundRect.X; x++ {
		for y := boundRect.Y; y < boundRect.Height+boundRect.Y; y++ {
			color1, _ := p.getDrawInfo().getPixel(math32.NewVector2(x, y), img, geo)
			_, _, _, a := color1.RGBA()
			if a != 0 {
				return true
			}
		}
	}
	return false
}
func (p *Sprite) touchedColor(dst *Sprite, color Color) bool {
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

	c := p.costumes[p.currentCostumeIndex]
	pimg, cx, cy := c.needImage(p.g.fs)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)

	c2 := dst.costumes[dst.currentCostumeIndex]
	dstimg, cx2, cy2 := c2.needImage(p.g.fs)
	geo2 := dst.getDrawInfo().getPixelGeo(cx2, cy2)

	cr, cg, cb, ca := color.RGBA()
	//check boun rect pixel
	for x := boundRect.X; x < boundRect.Width+boundRect.X; x++ {
		for y := boundRect.Y; y < boundRect.Height+boundRect.Y; y++ {
			pos := math32.NewVector2(x, y)
			color1, _ := p.getDrawInfo().getPixel(pos, pimg, geo)
			color2, _ := dst.getDrawInfo().getPixel(pos, dstimg, geo2)
			_, _, _, a1 := color1.RGBA()
			r, g, b, a2 := color2.RGBA()
			if a1 != 0 && a2 != 0 && r == cr && g == cg && b == cb && a2 == ca {
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
		log.Printf("touchingSprite  curr(%f,%f) currRect(%s) currBoundRect(%s)  dst(%f,%f) dstRect(%s) dstRectBoundRect(%s) boundRect(%s)",
			p.x, p.y, currRect, currBoundRect, dst.x, dst.y, dstRect, dstRectBoundRect, boundRect)
	}

	c := p.costumes[p.currentCostumeIndex]
	pimg, cx, cy := c.needImage(p.g.fs)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)

	c2 := dst.costumes[dst.currentCostumeIndex]
	dstimg, cx2, cy2 := c2.needImage(p.g.fs)
	geo2 := dst.getDrawInfo().getPixelGeo(cx2, cy2)
	//check boun rect pixel
	for x := boundRect.X; x < boundRect.Width+boundRect.X; x++ {
		for y := boundRect.Y; y < boundRect.Height+boundRect.Y; y++ {
			pos := math32.NewVector2(x, y)
			color1, _ := p.getDrawInfo().getPixel(pos, pimg, geo)
			color2, _ := dst.getDrawInfo().getPixel(pos, dstimg, geo2)
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

	pos := &math32.Vector2{
		X: float64(rRect.Center.X),
		Y: float64(rRect.Center.Y),
	}

	worldW, wolrdH := p.g.worldSize_()
	pos.Y = -pos.Y
	pos = &math32.Vector2{
		X: float64(pos.X) + float64(worldW)/2.0,
		Y: float64(pos.Y) + float64(wolrdH)/2.0,
	}

	return int(pos.X), int(pos.Y) - int(rRect.Size.Height)/2
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

	pos := p.g.Camera.screenToWorld(math32.NewVector2(float64(hc.Pos.X), float64(hc.Pos.Y)))
	worldW, wolrdH := p.g.worldSize_()
	pos = &math32.Vector2{
		X: float64(pos.X) - float64(worldW)/2.0,
		Y: float64(pos.Y) - float64(wolrdH)/2.0,
	}
	pos.Y = -pos.Y
	if !rRect.Contains(pos) {
		return
	}
	c2 := p.costumes[p.currentCostumeIndex]
	img, cx, cy := c2.needImage(p.g.fs)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)
	color1, pos := p.getDrawInfo().getPixel(pos, img, geo)
	if debugInstr {
		log.Printf("hit color1(%v) p(%s)", color1, pos)
	}
	_, _, _, a := color1.RGBA()
	if a == 0 {
		return
	}
	return hitResult{Target: p}, true
}
