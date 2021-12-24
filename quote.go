package spx

import (
	"fmt"

	"github.com/goplus/spx/internal/gdi"
	"github.com/goplus/spx/internal/math32"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/colornames"
)

const (
	quotePadding     = 5.0
	quoteLineWidth   = 8.0
	quoteHeadLen     = 20.0
	quoteTextPadding = 2.0
)

type quoter struct {
	sprite      *Sprite
	message     string
	description string

	cachedImg *ebiten.Image
}

func (p *Sprite) quote_(message, description string) {
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

func (p *Sprite) waitStopQuote(secs float64) {
	p.g.Wait(secs)
	p.doStopQuote()
}

func (p *Sprite) doStopQuote() {
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
	bound := p.getSpriteBound()
	if bound == nil {
		return
	}
	w, h := dc.Size()
	op := new(ebiten.DrawImageOptions)
	x := bound.X - quotePadding*2 + float64(w)/2
	y := -bound.Y - quotePadding - bound.Height + float64(h)/2
	op.GeoM.Translate(x, y)
	dc.DrawImage(img, op)
}

func (p *quoter) getImage() *ebiten.Image {
	if p.cachedImg != nil {
		return p.cachedImg
	}
	bound := p.getSpriteBound()
	if bound == nil {
		return nil
	}
	w, h := bound.Width, bound.Height
	msgRender := gdi.NewTextRender(defaultFont, 135, 2)
	msgRender.AddText(p.message)
	msgW, msgH := msgRender.Size()
	h += float64(msgH / 2)
	desRender := gdi.NewTextRender(defaultFont2, 135, 2)
	var desW, desH int
	if p.description != "" {
		desRender.AddText((p.description))
		desW, desH = desRender.Size()
		h += float64(desH + quoteTextPadding)
	}
	w += (quotePadding + quoteLineWidth) * 2
	svg := gdi.NewSVG(int(w), int(h))

	mainH := int(h) - msgH/2
	dy := 0.0
	if p.description != "" {
		dy = float64(desH) + quoteTextPadding
		mainH -= int(dy)
	}

	left := fmt.Sprintf("m 0,%f %f,0 0,%f -%f,0 0,%f %f,0 0,%f -%f,0 z",
		dy,
		quoteHeadLen,
		quoteLineWidth,
		quoteHeadLen-quoteLineWidth,
		float64(mainH)-2*quoteLineWidth,
		quoteHeadLen-quoteLineWidth,
		quoteLineWidth,
		quoteHeadLen)
	right := fmt.Sprintf("m %f,%f %f,0 0,%f -%f, 0 0,-%f %f, 0 0,-%f -%f,0 z",
		w-quoteHeadLen,
		dy,
		quoteHeadLen,
		float64(mainH),
		quoteHeadLen,
		quoteLineWidth,
		quoteHeadLen-quoteLineWidth,
		float64(mainH)-2*quoteLineWidth,
		quoteHeadLen-quoteLineWidth)
	style := "fill:darkgreen;stroke:black"
	svg.Path(left, style)
	svg.Path(right, style)
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

func (p *quoter) getSpriteBound() *math32.Rect {
	rect := p.sprite.getRotatedRect()
	if rect == nil {
		return nil
	}
	return rect.BoundingRect()
}

func (p *quoter) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}
