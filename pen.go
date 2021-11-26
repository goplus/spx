package spx

import (
	"fmt"
	"image/color"

	"github.com/goplus/spx/internal/gdi"

	spxfs "github.com/goplus/spx/fs"
)

// -------------------------------------------------------------------------------------

type penLine struct {
	x1, y1 int
	x2, y2 int
	clr    color.RGBA
	width  int
}

func (p *penLine) draw(dc gdi.Canvas) {
	clr := p.clr
	style := fmt.Sprintf("stroke-linecap:round;stroke-width:%d;stroke:rgb(%d,%d,%d)", p.width, clr.R, clr.G, clr.B)
	if clr.A != 0xff {
		style = fmt.Sprintf("%s;stroke-opacity:%.2f", style, float64(clr.A)/0xff)
	}
	dc.Line(p.x1, p.y1, p.x2, p.y2, style)
}

// -------------------------------------------------------------------------------------

type turtleCanvas struct {
	objs []interface{}
}

func (p *turtleCanvas) eraseAll() {
	p.objs = nil
}

func (p *turtleCanvas) penLine(obj *penLine) {
	p.objs = append(p.objs, obj)
}

func (p *turtleCanvas) stampCostume(obj *spriteDrawInfo) {
	p.objs = append(p.objs, obj)
}

func (p turtleCanvas) draw(dc drawContext, fs spxfs.Dir) {
	if p.objs == nil {
		return
	}

	var canvas gdi.Canvas
	for _, obj := range p.objs {
		switch o := obj.(type) {
		case *penLine:
			if canvas.Target == nil {
				canvas = gdi.Start(dc.Image)
			}
			o.draw(canvas)
		case *spriteDrawInfo:
			if canvas.Target != nil {
				canvas.End()
				canvas.Target = nil
			}
			o.drawOn(dc, fs)
		}
	}
	if canvas.Target != nil {
		canvas.End()
	}
}

// -------------------------------------------------------------------------------------
