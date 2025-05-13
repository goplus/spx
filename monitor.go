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
	"github.com/realdream-ai/mathf"
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
	color   mathf.Color
	pos     mathf.Vec2
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
	color, err := mathf.NewColorAny(getSpcspVal(v, "color"))
	if err != nil {
		color = mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)
	}
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	visible := v["visible"].(bool)

	panel := ui.NewUiMonitor()
	monitor := &Monitor{
		target: target, val: val, eval: eval, name: name, size: size,
		visible: visible, mode: mode, color: color, pos: mathf.NewVec2(x, y), label: label, panel: panel,
	}

	return monitor, nil
}

func (pself *Monitor) onUpdate(delta float64) {
	val := pself.eval()
	pself.panel.SetVisible(pself.visible)
	if !pself.visible {
		return
	}
	pself.panel.ShowAll(pself.mode == 1)
	pself.panel.UpdateScale(pself.size)
	pself.panel.UpdatePos(pself.pos)
	pself.panel.UpdateText(pself.label, val)
	pself.panel.UpdateColor(pself.color)
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
	getVarPrefix   = "getVar:"
	getTimerPrefix = "getProp:"
)

func buildMonitorEval(g reflect.Value, t, val string) func() string {
	target, from := getTarget(g, t)
	if from < 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(val, getVarPrefix):
		name := val[len(getVarPrefix):]
		// check field
		ref := getValueRef(target, name, from)
		if ref.IsValid() {
			return func() string {
				return fmt.Sprint(ref.Interface())
			}
		}
		// check method
		m := target.Addr().MethodByName(name)
		if m.IsValid() {
			mType := m.Type()
			// only property method (getter) with one parameter and one return value
			if mType.NumIn() == 0 && mType.NumOut() == 1 {
				return func() string {
					result := m.Call(nil)[0].Interface()
					// special case for float
					fVal, succ := result.(float64)
					if succ {
						return fmt.Sprintf("%.2f", fVal)
					}
					f32Val, succ := result.(float32)
					if succ {
						return fmt.Sprintf("%.2f", f32Val)
					}
					return fmt.Sprint(result)
				}
			}
		}
		log.Println("[WARN] Monitor: prop or method(getter) not found -", name, target)
	default:
		log.Println("[WARN] Monitor: unknown command -", val)
	}
	return nil
}

func (p *Monitor) setVisible(visible bool) {
	p.visible = visible
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
	return pself.pos.X
}
func (pself *Monitor) Ypos() float64 {
	return pself.pos.Y
}
func (pself *Monitor) SetXpos(x float64) {
	pself.pos.X = x
}
func (pself *Monitor) SetYpos(y float64) {
	pself.pos.Y = y
}
func (pself *Monitor) SetXYpos(x float64, y float64) {
	pself.pos = mathf.NewVec2(x, y)
}
func (pself *Monitor) ChangeXpos(dx float64) {
	pself.pos.X += dx
}
func (pself *Monitor) ChangeYpos(dy float64) {
	pself.pos.Y += dy
}
func (pself *Monitor) ChangeXYpos(dx float64, dy float64) {
	pself.pos = pself.pos.Add(mathf.NewVec2(dx, dy))
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
