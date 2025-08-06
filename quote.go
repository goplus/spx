/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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
	"fmt"
	"math"

	"github.com/goplus/spx/internal/gdi"
	xfont "github.com/goplus/spx/internal/gdi/font"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font"
)

const (
	quotePadding     = 5.0
	quoteLineWidth   = 8.0
	quoteHeadLen     = 16.0
	quoteTextPadding = 3.0
	quoteBorderRadis = 10.0
)

var (
	quoteMsgFont gdi.Font
	quoteDesFont gdi.Font
)

func init() {
	const dpi = 72
	quoteMsgFont = xfont.NewDefault(&xfont.Options{
		Size:    35,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	quoteDesFont = xfont.NewDefault(&xfont.Options{
		Size:    18,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
}

type quoter struct {
	sprite      *SpriteImpl
	message     string
	description string

	cachedImg *ebiten.Image
}

func (p *SpriteImpl) quote_(message, description string) {
	old := p.quoteObj
	if old == nil {
		p.quoteObj = &quoter{sprite: p, message: message, description: description}
		p.g.addShape(p.quoteObj)
	} else {
		old.message, old.description = message, description
		old.cachedImg = nil
		p.g.activateShape(old)
	}
}

func (p *SpriteImpl) waitStopQuote(secs float64) {
	p.g.Wait(secs)
	p.doStopQuote()
}

func (p *SpriteImpl) doStopQuote() {
	if p.quoteObj != nil {
		p.g.removeShape(p.quoteObj)
		p.quoteObj = nil
	}
}

func (p *quoter) draw(dc drawContext) {
	img := p.getImage()
	if img == nil {
		return
	}
	imgW, imgH := img.Size()
	w, h := dc.Size()
	op := new(ebiten.DrawImageOptions)
	x := p.sprite.x + float64(w)/2 - float64(imgW)/2
	y := -p.sprite.y - quotePadding - float64(imgH) + float64(h)/2 + float64(imgH)/2
	op.GeoM.Translate(x, y)
	dc.DrawImage(img, op)
}

func (p *quoter) getImage() *ebiten.Image {
	if p.cachedImg != nil {
		return p.cachedImg
	}
	bound := p.sprite.getRotatedRect()
	w := math.Max(bound.Size.Height, bound.Size.Width)
	w += quotePadding + quoteLineWidth
	h := w * 1.15
	quoteHeight := h
	msgRender := gdi.NewTextRender(quoteMsgFont, 135, 2)
	msgRender.AddText(p.message)
	msgW, msgH := msgRender.Size()
	h += float64(msgH / 2)
	desRender := gdi.NewTextRender(quoteDesFont, 135, 2)
	var desW, desH int
	if p.description != "" {
		desRender.AddText((p.description))
		desW, desH = desRender.Size()
		h += float64(desH + quoteTextPadding)
	}

	svg := gdi.NewSVG(int(w), int(h))
	mainH := int(h) - msgH/2
	dy := 0.0
	if p.description != "" {
		dy = float64(desH) + quoteTextPadding
		mainH -= int(dy)
	}
	half := fmt.Sprintf("m 0 %f q 0 %f %f %f h %f q %f %f 0 %f h -%f v %f h %f q %f %f 0 %f h -%f q %f 0 %f %f z",
		dy+quoteBorderRadis,

		-quoteBorderRadis,
		quoteBorderRadis,
		-quoteBorderRadis,

		quoteHeadLen,

		quoteLineWidth/2,
		quoteLineWidth/2,
		quoteLineWidth,

		quoteHeadLen+3,

		float64(mainH)-2*quoteLineWidth,

		quoteHeadLen+3,

		quoteLineWidth/2,
		quoteLineWidth/2,
		quoteLineWidth,

		quoteHeadLen,

		-quoteBorderRadis,
		-quoteBorderRadis,
		-quoteBorderRadis,
	)
	svg.Def()
	svg.Path(half, `id="quote"`)
	svg.DefEnd()
	// "["
	style := "fill:rgb(144,169,55);stroke:black;"
	svg.Use(0, 0, "#quote", style)
	// "]"
	svg.Gtransform(fmt.Sprintf("rotate(%.1f %f %f)", 180.0, w/2, quoteHeight/2+dy))
	svg.Use(0, 0, "#quote", style)
	svg.Gend()
	svg.End()

	img, err := svg.ToImage()
	if err != nil {
		panic(err)
	}
	p.cachedImg = ebiten.NewImageFromImage(img)
	msgRender.Draw(p.cachedImg, (int(w)-msgW)/2, int(h)-msgH, colornames.White, 0)
	if p.description != "" {
		desRender.Draw(p.cachedImg, (int(w)-desW)/2, 0, colornames.White, 0)
	}
	return p.cachedImg
}

func (p *quoter) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}
