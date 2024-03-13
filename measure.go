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
	"fmt"
	"image"
	"strconv"
	"strings"

	"github.com/goplus/spx/internal/gdi"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	measureEdgeLen    = 6
	measureLineWidth  = 2
	measureTextMargin = 8
)

type measure struct {
	size    float64
	x       float64
	y       float64
	heading float64

	// computed properties
	text         string
	color        Color
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
	c, err := parseColor(getSpcspVal(v, "color", 0.0))
	if err != nil {
		panic(err)
	}
	return &measure{
		heading:      heading,
		size:         size,
		text:         text,
		color:        c,
		x:            v["x"].(float64),
		y:            v["y"].(float64),
		svgLineStyle: fmt.Sprintf("stroke-width:%d;stroke:rgb(%d, %d, %d);", measureLineWidth, c.R, c.G, c.B),
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
	textW, textH := render.Size()
	x, y := m.getTextPos(textW, textH)
	render.Draw(m.cachedImg, x, y, m.color, 0)
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

func (m *measure) getTextPos(textW, textH int) (int, int) {
	rotation := (int(m.heading)%360 + 360) % 360
	center := m.svgSize / 2

	switch {
	case rotation == 0:
		return center + measureTextMargin, center - textH/2 // right of center
	case rotation == 90:
		return center - textW/2, center - measureTextMargin - textH // above center
	case rotation == 180:
		return center - textW - measureTextMargin, center - textH/2 // left of center
	case rotation == 270:
		return center - textW/2, center + measureTextMargin // under center
	case rotation > 0 && rotation <= 45:
		return center + measureTextMargin, center + (measureTextMargin-textH)/2
	case rotation > 45 && rotation < 90:
		return center - (measureTextMargin+textW)/2, center - measureTextMargin - textH
	case rotation > 90 && rotation <= 135:
		return center + (measureTextMargin-textW)/2, center - measureTextMargin - textH
	case rotation > 135 && rotation < 180:
		return center - measureTextMargin - textW, center
	case rotation > 180 && rotation <= 225:
		return center - measureTextMargin - textW, center - textH
	case rotation > 225 && rotation < 270:
		return center - textW/3, center + measureTextMargin
	case rotation > 270 && rotation <= 315:
		return center - textW, center + measureTextMargin
	default:
		return center + measureTextMargin, center - textH
	}
}

func (m *measure) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}
