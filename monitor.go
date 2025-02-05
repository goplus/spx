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
	"strings"
	"syscall"

	"github.com/goplus/spx/internal/gdi"
	xfont "github.com/goplus/spx/internal/gdi/font"
	"github.com/goplus/spx/internal/tools"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
)

// -------------------------------------------------------------------------------------

// Monitor class.
type Monitor struct {
	game    *Game
	name    WidgetName
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
"type": "Monitor",
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
func newMonitor(g reflect.Value, v specsp) (*Monitor, error) {
	target := v["target"].(string)
	val := v["val"].(string)
	name := v["name"].(string)
	size := 1.0
	if v["size"] != nil {
		size, _ = tools.GetFloat(v["size"])
	}
	eval := buildMonitorEval(g, target, val)
	if eval == nil {
		return nil, syscall.ENOENT
	}
	mode := int(v["mode"].(float64))
	color, err := parseColor(getSpcspVal(v, "color"))
	if err != nil {
		color = Color{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	visible := v["visible"].(bool)
	return &Monitor{
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
		log.Println("[WARN] Monitor: var not found -", name, target)
	default:
		log.Println("[WARN] Monitor: unknown command -", val)
	}
	return nil
}

func (p *Monitor) setVisible(visible bool) {
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
	stmBackground    = Color{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff}
	stmBackgroundPen = Color{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff}
	stmValueground   = Color{R: 0x21, G: 0x9f, B: 0xfc, A: 0xff}
	stmValueRectPen  = Color{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff}
)

var (
	size2Font = make(map[int]gdi.Font)
)

func getOrCreateFont(size int) gdi.Font {
	const dpi = 72
	if size <= 0 {
		size = 1
	}
	if font, ok := size2Font[size]; ok {
		return font
	}
	size2Font[size] = xfont.NewDefault(&xfont.Options{
		Size:    float64(size),
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	return size2Font[size]
}

func (p *Monitor) draw(dc drawContext) {
	if !p.visible {
		return
	}
	val := p.eval()
	x, y := p.x, p.y
	x, y = p.game.convertWinSpace2GameSpace(x, y)
	switch p.mode {
	case 2:
		render := gdi.NewTextRender(defaultFont, 0x80000, 0)
		render.AddText(val)
		intw, inth := render.Size()
		textW, h := float64(intw), float64(inth)
		w := textW
		if w < stmDefaultW {
			w = stmDefaultW
		}
		drawRoundRect(dc, x, y, w, h, p.color, p.color)
		if val != "" {
			render.Draw(dc.Image, int(x+((w-textW)/2)), int(y), color.White, 0)
		}
	default:
		font := getOrCreateFont(int(p.size * 12))
		labelRender := gdi.NewTextRender(font, 0x80000, 0)
		labelRender.AddText(p.label)
		intw, inth := labelRender.Size()
		labelW, labelH := float64(intw), float64(inth)

		textRender := gdi.NewTextRender(font, 0x80000, 0)
		textRender.AddText(val)
		intw, inth = textRender.Size()
		textW, textH := float64(intw), float64(inth)
		textRectW := textW
		if textRectW < stmDefaultSmW*p.size {
			textRectW = stmDefaultSmW * p.size
		}
		hGap := stmHoriGapSm * p.size
		vGap := stmVertGapSm * p.size

		w := labelW + textRectW + hGap*2
		h := labelH + vGap*2
		drawRoundRect(dc, x, y, w, h, stmBackground, stmBackgroundPen)
		if p.label != "" {
			labelRender.Draw(dc.Image, int(x+hGap), int(y+vGap), color.Black, 0)
		}

		textGap2Right := -1.0
		textGapV := 2.0
		textGapH := 2.0
		textPaddingOffset := 5.0
		w2 := textRectW + textGapH*2
		x2 := x + w - w2 - textGap2Right
		y2 := y + textGapV
		h2 := h - textGapV*2
		drawRoundRect(dc, x2, y2, w2, h2, stmValueground, stmValueRectPen)
		if val != "" {
			textRender.Draw(dc.Image, int(x2+(w2-textW)/2+textPaddingOffset), int(y+vGap+(labelH-textH)/2), color.White, 0)
		}
	}
}

type rectKey struct {
	x, y, w, h  float64
	clr, clrPen Color
}

var (
	rcMap = make(map[rectKey]*ebiten.Image)
)

func drawRoundRect(dc drawContext, x, y, w, h float64, clr, clrPen Color) {
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

func getCircleRect(dc drawContext, x, y, w, h float64, clr, clrPen Color) (image.Image, error) {
	varTable := []string{
		"$x", fmt.Sprintf("%f", x+h/2),
		"$y", fmt.Sprintf("%f", y),
		"$rx", fmt.Sprintf("%f", h/2),
		"$ry", fmt.Sprintf("%f", h/2),
		"$w", fmt.Sprintf("%f", w-h/2),
		"$h", fmt.Sprintf("%f", 0.0),
	}
	glyphTpl := "M $x $y h $w a $rx $ry 0 0 1 $rx $ry v $h a $rx $ry 0 0 1 -$rx $ry h -$w a $rx $ry 0 0 1 -$rx -$ry v -$h a $rx $ry 0 0 1 $rx -$ry z"
	glyph := strings.NewReplacer(varTable...).Replace(glyphTpl)

	alpha := float32(clr.A) / 255
	style := fmt.Sprintf(
		"fill:rgb(%d, %d, %d);stroke-width:1;stroke:rgb(%d, %d, %d);fill-opacity:%f",
		clr.R, clr.G, clr.B,
		clrPen.R, clrPen.G, clrPen.B, alpha)

	cx, cy := dc.Size()
	svg := gdi.NewSVG(cx, cy)
	svg.Path(glyph, style)
	svg.End()
	return svg.ToImage()
}

func getRoundRect(dc drawContext, x, y, w, h float64, clr, clrPen Color) (image.Image, error) {
	varTable := []string{
		"$x", fmt.Sprintf("%f", x),
		"$y2", fmt.Sprintf("%f", y+stmCornerSize),
		"$w2", fmt.Sprintf("%f", w-stmCornerSize*2),
		"$h2", fmt.Sprintf("%f", h-stmCornerSize*2),
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

func (p *Monitor) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}

// -------------------------------------------------------------------------------------
// IWidget
func (pself *Monitor) GetName() WidgetName {
	return pself.name
}

func (pself *Monitor) Visible() bool {
	return pself.visible
}
func (pself *Monitor) Show() {
	pself.visible = true
}
func (pself *Monitor) Hide() {
	pself.visible = false
}
func (pself *Monitor) Xpos() float64 {
	return pself.x
}
func (pself *Monitor) Ypos() float64 {
	return pself.y
}
func (pself *Monitor) SetXpos(x float64) {
	pself.x = x
}
func (pself *Monitor) SetYpos(y float64) {
	pself.y = y
}
func (pself *Monitor) SetXYpos(x float64, y float64) {
	pself.x, pself.y = x, y
}
func (pself *Monitor) ChangeXpos(dx float64) {
	pself.x += dx
}
func (pself *Monitor) ChangeYpos(dy float64) {
	pself.y += dy
}
func (pself *Monitor) ChangeXYpos(dx float64, dy float64) {
	pself.x += dx
	pself.y += dy
}

func (pself *Monitor) Size() float64 {
	return pself.size
}
func (pself *Monitor) SetSize(size float64) {
	pself.size = size
	pself.updateSize()
}
func (pself *Monitor) ChangeSize(delta float64) {
	pself.size += delta
	pself.updateSize()
}

func (pself *Monitor) updateSize() {

}
