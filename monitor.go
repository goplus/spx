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
	"image/color"
	"log"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/goplus/spx/internal/gdi"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/colornames"
)

// -------------------------------------------------------------------------------------

// MonitorWidget class.
type MonitorWidget struct {
	name    string
	size    float64
	target  string
	val     string
	eval    func() string
	mode    int
	color   Color
	x, y    float64
	label   string
	visible bool
}

/*
"type": "MonitorWidget",
"target": "",
"val": "getVar:score",
"color": 15629590,
"label": "score",
"mode": 1,
"sliderMin": 0,
"sliderMax": 100,
"x": 5,
"y": 5,
"isDiscrete": true,
"visible": true
*/
func newMonitorWidget(g reflect.Value, v specsp) (*MonitorWidget, error) {
	target := v["target"].(string)
	val := v["val"].(string)
	name := v["name"].(string)
	size := v["size"].(float64)
	if size == 0 {
		size = 1
	}
	eval := buildMonitorEval(g, target, val)
	if eval == nil {
		return nil, syscall.ENOENT
	}
	mode := int(v["mode"].(float64))
	color, err := parseColor(getSpcspVal(v, "color"))
	if err != nil {
		panic(err)
	}
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	visible := v["visible"].(bool)
	return &MonitorWidget{
		target: target, val: val, eval: eval, name: name, size: size,
		visible: visible, mode: mode, color: color, x: x, y: y, label: label,
	}, nil
}

func getTarget(g reflect.Value, target string) (reflect.Value, int) {
	if target == "" {
		return g, 1 // spx.Game
	}
	if val := findFieldPtr(g, target, 0); val != nil {
		if _, ok := val.(Shape); ok {
			return reflect.ValueOf(val).Elem(), 2 // (spx.Sprite, *Game)
		}
	}
	return reflect.Value{}, -1
}

func getValueRef(target reflect.Value, name string, from int) reflect.Value {
	if valPtr := findFieldPtr(target, name, from); valPtr != nil {
		return reflect.ValueOf(valPtr).Elem()
	}
	return reflect.Value{}
}

const (
	getVarPrefix = "getVar:"
)

func buildMonitorEval(g reflect.Value, t, val string) func() string {
	target, from := getTarget(g, t)
	if from < 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(val, getVarPrefix):
		name := val[len(getVarPrefix):]
		ref := getValueRef(target, name, from)
		if ref.IsValid() {
			return func() string {
				return fmt.Sprint(ref.Interface())
			}
		}
		log.Println("[WARN] MonitorWidget: var not found -", name, target)
	default:
		log.Println("[WARN] MonitorWidget: unknown command -", val)
	}
	return nil
}

func (p *MonitorWidget) setVisible(visible bool) {
	p.visible = visible
}

const (
	stmDefaultW   = 47
	stmDefaultSmW = 41
	stmCornerSize = 2
	stmVertGapSm  = 4
	stmHoriGapSm  = 5
)

var (
	stmBackground    = Color{R: 0xc4, G: 0xc7, B: 0xc1, A: 0xff}
	stmBackgroundPen = colornames.Black
	stmValueground   = Color{R: 33, G: 159, B: 252, A: 255}
	stmValueRectPen  = Color{R: 33, G: 159, B: 252, A: 0}
)

func (p *MonitorWidget) draw(dc drawContext) {
	if !p.visible {
		return
	}
	val := p.eval()
	switch p.mode {
	case 2:
		x, y := int(p.x)+2, int(p.y)+6
		render := gdi.NewTextRender(defaultFont, 0x80000, 0)
		render.AddText(val)
		textW, h := render.Size()
		w := textW
		if w < stmDefaultW {
			w = stmDefaultW
		}
		drawRoundRect(dc, x, y, w, h, p.color, p.color)
		render.Draw(dc.Image, x+((w-textW)>>1), y, color.White, 0)
	default:
		x, y := int(p.x), int(p.y)
		labelRender := gdi.NewTextRender(defaultFont2, 0x80000, 0)
		labelRender.Scale = p.size
		labelRender.AddText(p.label)
		labelW, h := labelRender.Size()

		textRender := gdi.NewTextRender(defaultFontSm, 0x80000, 0)
		textRender.Scale = p.size
		textRender.AddText(val)
		textW, textH := textRender.Size()
		textRectW := textW
		if textRectW < int(stmDefaultSmW*p.size) {
			textRectW = int(stmDefaultSmW * p.size)
		}
		hGap := stmHoriGapSm * p.size
		vGap := stmVertGapSm * p.size
		w := labelW + textRectW + int(hGap*2)
		h += int(vGap * 2)
		drawRoundRect(dc, x, y, w, h, stmBackground, stmBackgroundPen)
		labelRender.Draw(dc.Image, x+int(hGap), y+int(vGap), color.Black, 0)
		x += labelW + int(hGap*2)
		y += int(vGap / 2)
		h2 := textH + int(float64(vGap*1.3))
		drawRoundRect(dc, x, y, textRectW, h2, stmValueground, stmValueRectPen)
		y += int(vGap / 2)
		textRender.Draw(dc.Image, x+((textRectW-textW)>>1)+h2/2, y, color.White, 0)
	}
}

type rectKey struct {
	x, y, w, h  int
	clr, clrPen Color
}

var (
	rcMap = make(map[rectKey]*ebiten.Image)
)

func drawRoundRect(dc drawContext, x, y, w, h int, clr, clrPen Color) {
	key := rectKey{x, y, w, h, clr, clrPen}
	if i, ok := rcMap[key]; ok {
		dc.DrawImage(i, nil)
		return
	}
	img, err := getCircleRect(dc, x, y, w, h, clr, clrPen)
	if err != nil {
		panic(err)
	}
	rcMap[key] = ebiten.NewImageFromImage(img)
}

func getCircleRect(dc drawContext, x, y, w, h int, clr, clrPen Color) (image.Image, error) {
	varTable := []string{
		"$x", strconv.Itoa(x + h/2),
		"$y", strconv.Itoa(y),
		"$rx", strconv.Itoa(h / 2),
		"$ry", strconv.Itoa(h / 2),
		"$w", strconv.Itoa(w - h/2),
		"$h", strconv.Itoa(0),
	}
	glyphTpl := "M $x $y h $w a $rx $ry 0 0 1 $rx $ry v $h a $rx $ry 0 0 1 -$rx $ry h -$w a $rx $ry 0 0 1 -$rx -$ry v -$h a $rx $ry 0 0 1 $rx -$ry z"
	glyph := strings.NewReplacer(varTable...).Replace(glyphTpl)

	style := fmt.Sprintf(
		"fill:rgb(%d, %d, %d);stroke-width:1;stroke:rgb(%d, %d, %d);fill-opacity:0.5",
		clr.R, clr.G, clr.B,
		clrPen.R, clrPen.G, clrPen.B)

	cx, cy := dc.Size()
	svg := gdi.NewSVG(cx, cy)
	svg.Path(glyph, style)
	svg.End()
	return svg.ToImage()
}

func getRoundRect(dc drawContext, x, y, w, h int, clr, clrPen Color) (image.Image, error) {
	varTable := []string{
		"$x", strconv.Itoa(x),
		"$y2", strconv.Itoa(y + stmCornerSize),
		"$w2", strconv.Itoa(w - stmCornerSize*2),
		"$h2", strconv.Itoa(h - stmCornerSize*2),
	}
	glyphTpl := "M $x $y2 s 0 -2 2 -2 h $w2 s 2 0 2 2 v $h2 s 0 2 -2 2 h -$w2 s -2 0 -2 -2 z"
	glyph := strings.NewReplacer(varTable...).Replace(glyphTpl)

	style := fmt.Sprintf(
		"fill:rgb(%d, %d, %d);stroke-width:0.7;stroke:rgb(%d, %d, %d)",
		clr.R, clr.G, clr.B,
		clrPen.R, clrPen.G, clrPen.B)

	cx, cy := dc.Size()
	svg := gdi.NewSVG(cx, cy)
	svg.Path(glyph, style)
	svg.End()
	return svg.ToImage()
}

func (p *MonitorWidget) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}

// -------------------------------------------------------------------------------------
// IWidget
func (pself *MonitorWidget) GetName() string {
	return pself.name
}

func (pself *MonitorWidget) Visible() bool {
	return pself.visible
}
func (pself *MonitorWidget) Show() {
	pself.visible = true
}
func (pself *MonitorWidget) Hide() {
	pself.visible = false
}
func (pself *MonitorWidget) Xpos() float64 {
	return pself.x
}
func (pself *MonitorWidget) Ypos() float64 {
	return pself.y
}
func (pself *MonitorWidget) SetXpos(x float64) {
	pself.x = x
}
func (pself *MonitorWidget) SetYpos(y float64) {
	pself.y = y
}
func (pself *MonitorWidget) SetXYpos(x float64, y float64) {
	pself.x, pself.y = x, y
}
func (pself *MonitorWidget) ChangeXpos(dx float64) {
	pself.x += dx
}
func (pself *MonitorWidget) ChangeYpos(dy float64) {
	pself.y += dy
}
func (pself *MonitorWidget) ChangeXYpos(dx float64, dy float64) {
	pself.x += dx
	pself.y += dy
}

func (pself *MonitorWidget) Size() float64 {
	return pself.size
}
func (pself *MonitorWidget) SetSize(size float64) {
	pself.size = size
	pself.updateSize()
}
func (pself *MonitorWidget) ChangeSize(delta float64) {
	pself.size += delta
	pself.updateSize()
}

func (pself *MonitorWidget) updateSize() {
	// TODO(tanjp) updateSize not implemented
}
