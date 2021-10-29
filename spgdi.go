package spx

import (
	"image"

	"github.com/goplus/spx/internal/gdi"
	"github.com/goplus/spx/internal/math32"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/qiniu/x/objcache"

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

type sprKey struct {
	scale         float64
	direction     float64
	costume       *costume
	rotationStyle RotationStyle
}

func (p *sprKey) tryGet() *gdi.Sprite {
	if val, ok := grpSpr.TryGet(*p); ok {
		return val.(*gdi.Sprite)
	}
	return nil
}

func (p *sprKey) get(sp *Sprite) *gdi.Sprite {
	val, _ := grpSpr.Get(sp, *p)
	return val.(*gdi.Sprite)
}

func (p *sprKey) doGet(sp *Sprite) *gdi.Sprite {
	w, h := sp.g.size()
	img := ebiten.NewImage(w, h)
	defer img.Dispose()

	p.drawOn(img, 0, 0, sp.g.fs)

	spi2 := gdi.NewSprite(img, p.costume.rect)
	//spi := gdi.NewSpriteFromScreen(img)
	//log.Printf(" spi %s, spi2 %s", spi.Rect, spi2.Rect)
	return spi2
}

func (p *sprKey) drawOn(target *ebiten.Image, x, y float64, fs spxfs.Dir) {
	c := p.costume

	img, centerX, centerY := c.needImage(fs)
	c.rect = img.Bounds()

	scale := p.scale / float64(c.bitmapResolution)
	screenW, screenH := target.Size()

	op := new(ebiten.DrawImageOptions)
	geo := &op.GeoM

	direction := p.direction + c.faceLeft
	if direction == 90 {

		x = float64(screenW>>1) + x - centerX*scale
		y = float64(screenH>>1) - y - centerY*scale
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
		geo.Translate(float64(screenW>>1)+x, float64(screenH>>1)-y)

	}

	c.rect = math32.ApplyGeoForRect(c.rect, geo)

	target.DrawImage(img, op)

}

func doGetSpr(ctx objcache.Context, key objcache.Key) (val objcache.Value, err error) {
	sp := ctx.(*Sprite)
	di := key.(sprKey)
	spr := di.doGet(sp)
	return spr, nil
}

var (
	grpSpr *objcache.Group = objcache.NewGroup("spr", 0, doGetSpr)
)

// -------------------------------------------------------------------------------------

type spriteDrawInfo struct {
	sprKey
	x, y    float64
	visible bool
}

func (p *spriteDrawInfo) drawOn(dc drawContext, fs spxfs.Dir) {
	sp := p.tryGet()
	if sp == nil {
		p.sprKey.drawOn(dc.Image, p.x, p.y, fs)
	} else {
		p.doDrawOn(dc, sp)
	}
}

func (p *spriteDrawInfo) draw(dc drawContext, ctx *Sprite) {
	sp := p.get(ctx)
	p.doDrawOn(dc, sp)
}

func (p *spriteDrawInfo) doDrawOn(dc drawContext, sp *gdi.Sprite) {
	img := sp.Image()
	if img.Rect.Empty() {
		return
	}
	src := ebiten.NewImageFromImage(img)
	defer src.Dispose()

	op := new(ebiten.DrawImageOptions)
	x := float64(sp.Rect.Min.X) + p.x
	y := float64(sp.Rect.Min.Y) - p.y
	op.GeoM.Translate(x, y)
	dc.DrawImage(src, op)
}

func (p *Sprite) getDrawInfo() *spriteDrawInfo {
	return &spriteDrawInfo{
		sprKey: sprKey{
			scale:         p.scale,
			direction:     p.direction,
			costume:       p.costumes[p.currentCostumeIndex],
			rotationStyle: p.rotationStyle,
		},
		x:       p.x,
		y:       p.y,
		visible: p.isVisible,
	}
}

func (p *Sprite) getGdiSprite() (spr *gdi.Sprite, pt image.Point) {
	di := p.getDrawInfo()
	if !di.visible {
		return
	}

	spr = di.get(p)
	pt = image.Pt(int(di.x), -int(di.y))
	return
}

func (p *Sprite) getTrackPos() (topx, topy int) {
	spr, pt := p.getGdiSprite()
	if spr == nil {
		return
	}

	trackp := getTrackPos(spr)
	pt = trackp.Add(pt)
	return pt.X, pt.Y
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
	sp, pt := p.getGdiSprite()
	if sp == nil {
		return
	}

	pt = hc.Pos.Sub(pt)
	_, _, _, a := sp.Image().At(pt.X, pt.Y).RGBA()
	if a > 0 {
		return hitResult{Target: p}, true
	}
	return
}

// -------------------------------------------------------------------------------------

func getTrackPos(spr *gdi.Sprite) image.Point {
	pt, _ := grpTrackPos.Get(nil, spr)
	return pt.(image.Point)
}

func doGetTrackPos(ctx objcache.Context, key objcache.Key) (val objcache.Value, err error) {
	spr := key.(*gdi.Sprite)
	pt := spr.GetTrackPos()
	return pt, nil
}

var (
	grpTrackPos *objcache.Group = objcache.NewGroup("tp", 0, doGetTrackPos)
)

// -------------------------------------------------------------------------------------
