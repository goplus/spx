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
	"fmt"
	"image/color"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/goplus/spx/internal/gdi"
	"github.com/qiniu/x/log"
)

// -------------------------------------------------------------------------------------

// stageMonitor class.
type stageMonitor struct {
	eval    func() string
	mode    int
	color   int
	x, y    float64
	label   string
	visible bool
}

/*
	"type": "stageMonitor",
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
func newStageMonitor(g reflect.Value, v specsp) (*stageMonitor, error) {
	visible := v["visible"].(bool)
	if !visible {
		return nil, syscall.ENOENT
	}

	eval := buildMonitorEval(g, v)
	if eval == nil {
		return nil, syscall.EINVAL
	}

	mode := int(v["mode"].(float64))
	color := int(v["color"].(float64))
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	return &stageMonitor{
		visible: visible, mode: mode, color: color, x: x, y: y, label: label, eval: eval,
	}, nil
}

func getTarget(g reflect.Value, target string) (reflect.Value, int) {
	if target == "" {
		return g, 1 // spx.Game
	}
	for i, n := 0, g.NumField(); i < n; i++ {
		_, val := getFieldPtr(g, i)
		if fld, ok := val.(Shape); ok && spriteOf(fld).name == target {
			return reflect.ValueOf(val).Elem(), 2 // (spx.Sprite, *spx.Game)
		}
	}
	return reflect.Value{}, -1
}

func getValue(target reflect.Value, from int, name string) string {
	for i, n := from, target.NumField(); i < n; i++ {
		fldName, valPtr := getFieldPtr(target, i)
		if name == fldName {
			return fmt.Sprint(reflect.ValueOf(valPtr).Elem().Interface())
		}
	}
	return ""
}

func buildMonitorEval(g reflect.Value, v specsp) func() string {
	const (
		getVar = "getVar:"
	)
	target, from := getTarget(g, v["target"].(string))
	if from < 0 {
		return nil
	}
	val := v["val"].(string)
	switch {
	case strings.HasPrefix(val, getVar):
		name := val[len(getVar):]
		return func() string {
			return getValue(target, from, name)
		}
	}
	log.Println("[WARN] stageMonitor: unknown command -", val)
	return nil
}

/*
func (p *stageMonitor) setVisible(visible bool) {
	p.visible = visible
}
*/

const (
	stmDefaultW      = 47
	stmDefaultSmW    = 41
	stmCornerSize    = 2
	stmVertGapSm     = 4
	stmHoriGapSm     = 5
	stmBackground    = 193 | (196 << 8) | (199 << 16)
	stmBackgroundPen = 0
	stmTextRectPen   = 0xffffff
)

func (p *stageMonitor) draw(dc drawContext) {
	if !p.visible || p.eval == nil {
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
		drawRoundRect(dc, x, y, w, h, int(p.color), int(p.color))
		render.Draw(dc.Image, x+((w-textW)>>1), y, color.White, 0)
	default:
		x, y := int(p.x), int(p.y)
		labelRender := gdi.NewTextRender(defaultFont2, 0x80000, 0)
		labelRender.AddText(p.label)
		labelW, h := labelRender.Size()

		textRender := gdi.NewTextRender(defaultFontSm, 0x80000, 0)
		textRender.AddText(val)
		textW, textH := textRender.Size()
		textRectW := textW
		if textRectW < stmDefaultSmW {
			textRectW = stmDefaultSmW
		}
		w := labelW + textRectW + (stmHoriGapSm * 3)
		h += (stmVertGapSm * 2)
		drawRoundRect(dc, x, y, w, h, stmBackground, stmBackgroundPen)
		labelRender.Draw(dc.Image, x+stmHoriGapSm, y+stmVertGapSm, color.Black, 0)
		x += labelW + (stmHoriGapSm * 2)
		y += stmVertGapSm
		drawRoundRect(dc, x, y, textRectW, textH, int(p.color), stmTextRectPen)
		textRender.Draw(dc.Image, x+((textRectW-textW)>>1), y, color.White, 0)
	}
}

func drawRoundRect(dc drawContext, x, y, w, h int, clr, clrPen int) {
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
		(clr>>16)&0xff, (clr>>8)&0xff, clr&0xff,
		(clrPen>>16)&0xff, (clrPen>>8)&0xff, clrPen&0xff)

	canvas := gdi.Start(dc.Image)
	canvas.Path(glyph, style)
	canvas.End()
}

func (p *stageMonitor) hit(hc hitContext) (hr hitResult, ok bool) {
	return
}

// -------------------------------------------------------------------------------------
