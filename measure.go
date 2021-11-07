package spx

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"github.com/goplus/spx/internal/gdi"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	measureEdgeLen   = 6
	measureLineWidth = 2
)

type measure struct {
	size    float64
	x       float64
	y       float64
	heading float64

	// computed properties
	text         string
	color        color.Color
	cachedImg    *ebiten.Image
	svgLineStyle string
	svgRotate    string
	svgSize      int // size*scale + 0.5 + measureLineWidth
}

func newMeasure(v specsp) *measure {
	size := v["size"].(float64)
	scale := getSpcspVal(v, "scale", 1.0).(float64)
	text := strconv.FormatFloat(size, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	heading := getSpcspVal(v, "heading", 0.0).(float64)
	svgSize := int(size*scale + 0.5 + measureLineWidth)
	c := int(getSpcspVal(v, "color", 0.0).(float64))
	r, g, b := uint8(c>>16), uint8((c>>8)&0xff), uint8(c&0xff)
	return &measure{
		heading:      heading,
		size:         size,
		text:         text,
		color:        color.RGBA{R: r, G: g, B: b, A: 0xff},
		x:            v["x"].(float64),
		y:            v["y"].(float64),
		svgLineStyle: fmt.Sprintf("stroke-width:%d;stroke:rgb(%d, %d, %d);", measureLineWidth, r, g, b),
		svgRotate:    fmt.Sprintf("rotate(%.1f %d %d)", heading, svgSize>>1, svgSize>>1),
		svgSize:      svgSize,
	}
}

func getSpcspVal(ss specsp, key string, defaultVal ...interface{}) interface{} {
	v, ok := ss[key]
	if ok {
		return v
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return v
}

func (m *measure) draw(dc drawContext) {
	if m.cachedImg != nil {
		screenW, screenH := dc.Size()
		op := new(ebiten.DrawImageOptions)
		x := float64((screenW-m.svgSize)>>1) + m.x
		y := float64((screenH-m.svgSize)>>1) - m.y
		op.GeoM.Translate(x, y)
		dc.DrawImage(m.cachedImg, op)
		return
	}
	// lines
	lines, err := m.getLines()
	if err != nil {
		panic(err)
	}
	m.cachedImg = ebiten.NewImageFromImage(lines)
	// text
	render := gdi.NewTextRender(defaultFont, 0x80000, 0)
	render.AddText(m.text)
	render.Draw(m.cachedImg, m.svgSize>>1, m.svgSize>>1, m.color, 0)
}

func (m *measure) getLines() (image.Image, error) {
	size := m.svgSize
	svg := gdi.NewSVG(size, size)
	svg.Gtransform(m.svgRotate)
	svg.Line((size+measureLineWidth)/2, 0, (size+measureLineWidth)/2, size, m.svgLineStyle)
	svg.Line((size-measureEdgeLen)/2, 0, (size+measureEdgeLen)/2, 0, m.svgLineStyle)
	svg.Line((size-measureEdgeLen)/2, size-measureLineWidth/2, (size+measureEdgeLen)/2, size-measureLineWidth/2, m.svgLineStyle)
	svg.Gend()
	svg.End()
	return svg.ToImage()
}

func (m *measure) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}
