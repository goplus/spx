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
	"reflect"
	"strings"
	"syscall"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/tools"
	"github.com/goplus/spx/v2/internal/ui"
)

// -------------------------------------------------------------------------------------

const (
	getVarPrefix           = "getVar:"
	monitorUpdateIntervalS = 0.2 // Monitor update interval in seconds
)

// Monitor class.
type Monitor struct {
	game        *Game
	name        WidgetName
	size        float64
	target      string
	val         string
	eval        func() string
	mode        int
	color       mathf.Color
	pos         mathf.Vec2
	label       string
	visible     bool
	panel       *ui.UiMonitor
	isDirty     bool
	updateTimer float64
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
		isDirty: true, // Initial dirty state to ensure first render
	}

	return monitor, nil
}

func (pself *Monitor) onUpdate(delta float64) {
	pself.updateTimer += delta
	needsUpdate := pself.isDirty || pself.updateTimer >= monitorUpdateIntervalS
	if !needsUpdate {
		return
	}

	if pself.updateTimer >= monitorUpdateIntervalS {
		pself.updateTimer = 0
	}

	pself.panel.SetVisible(pself.visible)
	if !pself.visible {
		pself.setDirtyFlag(false)
		return
	}
	val := pself.eval() // only evaluate when visible
	pself.panel.ShowAll(pself.mode == 1)
	pself.panel.UpdateScale(pself.size)
	pself.panel.UpdatePos(pself.pos)
	pself.panel.UpdateText(pself.label, val)
	pself.setDirtyFlag(false)
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
	// Try exact match first (from specified index)
	if valPtr := findFieldPtr(target, name, from); valPtr != nil {
		return reflect.ValueOf(valPtr).Elem()
	}
	return reflect.Value{}
}

// aliasNameOf mimics gogen's aliasNameOf logic:
// For methods, lowercase names are mapped to uppercase (e.g., "add" -> "Add")
// For fields, no aliasing is performed (must use exact exported name)
func aliasNameOf(name string, isMethod bool) string {
	if isMethod && name != "" {
		if c := name[0]; c >= 'a' && c <= 'z' {
			return string(rune(c)+('A'-'a')) + name[1:]
		}
	}
	return ""
}

// methodHasAutoProperty checks if a method value is a valid auto-property (getter):
// Must have 0 parameters (excluding receiver) and 1 return value
func methodHasAutoProperty(m reflect.Value) bool {
	if !m.IsValid() {
		return false
	}
	mType := m.Type()
	return mType.NumIn() == 0 && mType.NumOut() == 1 // NumIn excludes receiver for bound methods
}

// resolveMember resolves a member (field or method) by name following gogen.Member semantics.
// Returns an accessor function if found, nil otherwise.
func resolveMember(target reflect.Value, name string, from int) func() string {
	// Try as field first (fields don't support lowercase aliases in gogen)
	ref := getValueRef(target, name, from)
	if ref.IsValid() {
		return func() string {
			return fmt.Sprint(ref.Interface())
		}
	}

	// Try as method with alias support (lowercase -> uppercase)
	// For method lookup, use pointer type
	targetForMethod := target
	if target.Kind() != reflect.Ptr && target.CanAddr() {
		targetForMethod = target.Addr()
	}

	aliasName := aliasNameOf(name, true)

	// Try original name first
	m := targetForMethod.MethodByName(name)
	if m.IsValid() && methodHasAutoProperty(m) {
		fmt.Println("autoproperty false", name)
		return makeAutoPropertyAccessor(m, false)
	}

	// Only try alias if original name didn't find any method
	if !m.IsValid() && aliasName != name {
		mAlias := targetForMethod.MethodByName(aliasName)
		// Only execute methodHasAutoProperty and makeAutoPropertyAccessor
		// when name method not found but aliasName method found
		if mAlias.IsValid() {
			if methodHasAutoProperty(mAlias) {
				fmt.Println("autoproperty true", aliasName)
				return makeAutoPropertyAccessor(mAlias, true)
			}
		}
	}

	return nil
}

func buildMonitorEval(g reflect.Value, t, val string) func() string {
	target, from := getTarget(g, t)
	if from < 0 {
		return nil
	}
	switch {
	case strings.HasPrefix(val, getVarPrefix):
		name := val[len(getVarPrefix):]
		if name == "" {
			spxlog.Error("Bind monitor error: name is empty")
			return nil
		}

		if eval := resolveMember(target, name, from); eval != nil {
			return eval
		}
		spxlog.Error("Bind monitor error: cannot find property or method (getter): %s", name)
	default:
		name := val
		if eval := resolveMember(target, name, from); eval != nil {
			return eval
		}
		spxlog.Error("Bind monitor error: cannot find property or method (getter): %s", name)
	}
	return nil
}

// makeAutoPropertyAccessor creates a runtime accessor for an auto-property method
func makeAutoPropertyAccessor(m reflect.Value, autoProperty bool) func() string {
	return func() string {
		if autoProperty {
			result := m.Call(nil)[0].Interface()
			// special case for float
			if fVal, ok := result.(float64); ok {
				return fmt.Sprintf("%.2f", fVal)
			}
			if f32Val, ok := result.(float32); ok {
				return fmt.Sprintf("%.2f", f32Val)
			}
			return fmt.Sprint(result)
		}

		// Return method pointer when autoProperty is false
		return fmt.Sprintf("%p", m.Interface())
	}
}

func (pself *Monitor) setVisible(visible bool) {
	if visible == pself.visible {
		return
	}

	pself.visible = visible
	pself.setDirtyFlag(true)
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
	pself.setVisible(true)
}

func (pself *Monitor) Hide() {
	pself.setVisible(false)
}

func (pself *Monitor) Xpos() float64 {
	return pself.pos.X
}

func (pself *Monitor) Ypos() float64 {
	return pself.pos.Y
}

func (pself *Monitor) SetXpos(x float64) {
	pself.setXYpos(x, pself.pos.Y)
}

func (pself *Monitor) SetYpos(y float64) {
	pself.setXYpos(pself.pos.X, y)
}

func (pself *Monitor) SetXYpos(x float64, y float64) {
	pself.setXYpos(x, y)
}

func (pself *Monitor) ChangeXpos(dx float64) {
	pself.setXYpos(pself.pos.X+dx, pself.pos.Y)
}

func (pself *Monitor) ChangeYpos(dy float64) {
	pself.setXYpos(pself.pos.X, pself.pos.Y+dy)
}

func (pself *Monitor) ChangeXYpos(dx float64, dy float64) {
	pself.setXYpos(pself.pos.X+dx, pself.pos.Y+dy)
}

func (pself *Monitor) setXYpos(x float64, y float64) {
	pself.pos = mathf.NewVec2(x, y)
	pself.setDirtyFlag(true)
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
	pself.setDirtyFlag(true)
}

func (pself *Monitor) setDirtyFlag(isDirty bool) {
	pself.isDirty = isDirty
}
