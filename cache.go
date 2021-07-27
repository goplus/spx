/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"image"

	"github.com/goplus/spx/internal/gdi"

	"github.com/hajimehoshi/ebiten"
	"github.com/qiniu/x/objcache"
)

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
	w, h := sp.size()
	img, err := ebiten.NewImage(w, h, defaultFilterMode)
	if err != nil {
		panic(err)
	}
	defer img.Dispose()

	p.drawOn(img, 0, 0, sp.fs)
	return gdi.NewSpriteFromScreen(img)
}

func (p *sprKey) drawOn(target *ebiten.Image, x, y float64, fs FileSystem) {
	c := p.costume
	img, centerX, centerY := c.needImage(fs)

	scale := p.scale / float64(c.bitmapResolution)
	screenW, screenH := target.Size()

	op := new(ebiten.DrawImageOptions)
	geo := &op.GeoM

	if p.direction == 90 {
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
		geo.Rotate(toRadian(p.direction - 90))
		geo.Translate(float64(screenW>>1)+x, float64(screenH>>1)-y)
	}

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

func (p *spriteDrawInfo) drawOn(dc drawContext, fs FileSystem) {
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
	src, err := ebiten.NewImageFromImage(sp.Image(), defaultFilterMode)
	if err != nil {
		panic(err)
	}
	defer src.Dispose()

	op := new(ebiten.DrawImageOptions)
	x := float64(sp.Rect.Min.X) + p.x
	y := float64(sp.Rect.Min.Y) - p.y
	op.GeoM.Translate(x, y)
	dc.DrawImage(src, op)
}

func (p *Sprite) getDrawInfo() *spriteDrawInfo {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return &spriteDrawInfo{
		sprKey: sprKey{
			scale:         p.scale,
			direction:     p.direction,
			costume:       p.costumes[p.currentCostumeIndex],
			rotationStyle: p.rotationStyle,
		},
		x:       p.x,
		y:       p.y,
		visible: p.visible,
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
