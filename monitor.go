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
	"log"
	"reflect"
	"strings"
	"syscall"

	"github.com/goplus/spx/internal/tools"
	"github.com/goplus/spx/internal/ui"
)

// -------------------------------------------------------------------------------------

// Monitor class.
type Monitor struct {
	game    *Game
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
	panel   *ui.UiMonitor
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

	panel := ui.NewUiMonitor()
	monitor := &Monitor{
		target: target, val: val, eval: eval, name: name, size: size,
		visible: visible, mode: mode, color: color, x: x, y: y, label: label, panel: panel,
	}

	return monitor, nil
}

func (pself *Monitor) onUpdate(delta float64) {
	if !pself.visible {
		return
	}
	val := pself.eval()
	pself.panel.UpdateScale(pself.size)
	pself.panel.UpdatePos(pself.x, pself.y)
	pself.panel.UpdateText(pself.label, val)
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

type rectKey struct {
	x, y, w, h  float64
	clr, clrPen Color
}

// -------------------------------------------------------------------------------------
// IWidget
func (pself *Monitor) GetName() string {
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
