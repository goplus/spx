/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"image"
	"image/color"
	"log"
	"math"
	"reflect"

	"github.com/goplus/spx/internal/effect"
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
	c := p.sprite.costumes[p.sprite.costumeIndex_]
	scale := p.sprite.scale / float64(c.bitmapResolution)
	direction := p.sprite.direction + c.faceRight
	direction = direction - 90
	geo := &ebiten.GeoM{}
	geo.Scale(1.0/scale, 1.0/scale)

	if p.sprite.rotationStyle == Normal {
		geo.Rotate(toRadian(direction))
	} else if p.sprite.rotationStyle == LeftRight {
		if math.Abs(p.sprite.direction) > 155 && math.Abs(p.sprite.direction) < 205 {
			geo.Scale(-1, 1)
		}
		if math.Abs(p.sprite.direction) > 0 && math.Abs(p.sprite.direction) < 25 {
			geo.Scale(-1, 1)
		}
	}
	geo.Scale(1.0, -1.0)
	geo.Translate(cx, cy)
	return geo
}

func (p *spriteDrawInfo) getPixel(pos *math32.Vector2, gdiImg gdi.Image, geo *ebiten.GeoM) (color.Color, *math32.Vector2) {
	img := gdiImg.Origin()
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

func (p *spriteDrawInfo) getUpdateRotateRect(x, y float64) *math32.RotatedRect {
	c := p.sprite.costumes[p.sprite.costumeIndex_]

	img, centerX, centerY := c.needImage(p.sprite.g.fs)
	p.sprite.applyPivot(c, &centerX, &centerY)
	rect := image.Rectangle{}
	rect.Min.X = 0
	rect.Min.Y = 0
	rect.Max = img.Bounds().Size()

	scale := p.sprite.scale / float64(c.bitmapResolution)

	geo := ebiten.GeoM{}
	geo.Reset()
	direction := p.sprite.direction + c.faceRight

	geo.Translate(-centerX, -centerY)
	geo.Scale(scale, scale)
	geo.Rotate(toRadian(direction - 90))
	geo.Translate(x, -y)

	geo2 := geo
	geo2.Scale(1.0, -1.0)
	rRect := math32.ApplyGeoForRotatedRect(rect, &geo2)
	return rRect
}

func (p *spriteDrawInfo) updateMatrix() {
	c := p.sprite.costumes[p.sprite.costumeIndex_]

	img, centerX, centerY := c.needImage(p.sprite.g.fs)
	p.sprite.applyPivot(c, &centerX, &centerY)
	rect := image.Rectangle{}
	rect.Min.X = 0
	rect.Min.Y = 0
	rect.Max = img.Bounds().Size()

	scale := p.sprite.scale / float64(c.bitmapResolution)
	worldW, wolrdH := p.sprite.g.worldSize_()

	geo := ebiten.GeoM{}
	geo.Reset()
	direction := p.sprite.direction + c.faceRight
	direction = direction - 90

	geo.Translate(-centerX, -centerY)
	geo.Scale(scale, scale)
	if p.sprite.rotationStyle == Normal {
		geo.Rotate(toRadian(direction))
	} else if p.sprite.rotationStyle == LeftRight {
		dirDeg := p.sprite.direction
		// convert to 0 ~ 360
		dirDeg = math.Mod(dirDeg, 360.0)
		if dirDeg < 0 {
			dirDeg += 360
		}
		// convert to -180 ~ 180
		if dirDeg > 180 {
			dirDeg -= 360
		}
		if dirDeg < -45 || dirDeg > 135 {
			geo.Scale(-1, 1)
		}
	}

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

	c := p.sprite.costumes[p.sprite.costumeIndex_]
	img, _, _ := c.needImage(fs)

	p.updateMatrix()

	if effs := p.sprite.greffUniforms; effs != nil {
		op := new(ebiten.DrawRectShaderOptions)
		op.GeoM = p.geo
		op.Uniforms = effs
		s, err := ebiten.NewShader(effect.ShaderFrag)
		if err != nil {
			panic(err)
		}
		op.Images[0] = img.Ebiten()
		imgSize := img.Ebiten().Bounds().Size()
		dc.DrawRectShader(imgSize.X, imgSize.Y, s, op)
	} else {
		op := new(ebiten.DrawImageOptions)
		op.Filter = ebiten.FilterLinear
		op.GeoM = p.geo
		dc.DrawImage(img.Ebiten(), op)
	}
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
	c := p.costumes[p.costumeIndex_]
	img, cx, cy := c.needImage(p.g.fs)
	p.applyPivot(c, &cx, &cy)
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
	c := p.costumes[p.costumeIndex_]
	img, cx, cy := c.needImage(p.g.fs)
	p.applyPivot(c, &cx, &cy)
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

func (p *Sprite) touchedColor_(dst *Sprite, color Color) bool {
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

	c := p.costumes[p.costumeIndex_]
	pimg, cx, cy := c.needImage(p.g.fs)
	p.applyPivot(c, &cx, &cy)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)

	c2 := dst.costumes[dst.costumeIndex_]
	dstimg, cx2, cy2 := c2.needImage(p.g.fs)
	dst.applyPivot(c2, &cx2, &cy2)
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

	c := p.costumes[p.costumeIndex_]
	pimg, cx, cy := c.needImage(p.g.fs)
	p.applyPivot(c, &cx, &cy)
	geo := p.getDrawInfo().getPixelGeo(cx, cy)

	c2 := dst.costumes[dst.costumeIndex_]
	dstimg, cx2, cy2 := c2.needImage(p.g.fs)
	dst.applyPivot(c2, &cx2, &cy2)
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
	if rRect == nil {
		return
	}

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
	c := p.costumes[p.costumeIndex_]
	img, cx, cy := c.needImage(p.g.fs)
	p.applyPivot(c, &cx, &cy)
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

func (p *Sprite) applyPivot(c *costume, cx, cy *float64) {
	*cx += p.pivot.X * float64(c.bitmapResolution)
	*cy -= p.pivot.Y * float64(c.bitmapResolution)
}
