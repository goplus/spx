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
	"reflect"
	"strings"
	"syscall"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/tools"
	"github.com/goplus/spx/v3/internal/ui"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
const (
	getVarPrefix           = "getVar:"
	monitorUpdateIntervalS = 0.2
	monitorModeDefault     = 1
	monitorModeLarge       = 2
	monitorModeList        = 4
	monitorStyleScratch    = "scratch"
)

// -----------------------------------------------------------------------------
// Monitor
// -----------------------------------------------------------------------------
type Monitor struct {
	name        WidgetName
	size        float64
	target      string
	val         string
	eval        func() ui.MonitorValue
	style       ui.MonitorStyle
	pos         mathf.Vec2
	visible     bool
	panel       monitorPanel
	isDirty     bool
	updateTimer float64
}

// monitorPanel separates widget lifecycle from the engine-backed view.
type monitorPanel interface {
	Render(ui.MonitorStyle, ui.MonitorValue)
	SetVisible(bool)
	UpdateScale(float64)
	UpdatePos(mathf.Vec2)
}

// -----------------------------------------------------------------------------
// Construction
// -----------------------------------------------------------------------------
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
func newMonitor(g reflect.Value, v coreproject.StageShape) (*Monitor, error) {
	target := v["target"].(string)
	val := v["val"].(string)
	name := v["name"].(string)
	size := 1.0
	if v["size"] != nil {
		size, _ = tools.GetFloat(v["size"])
	}
	appearance := parseMonitorAppearance(v)
	eval := buildMonitorEval(g, target, val, appearance)
	if eval == nil {
		return nil, syscall.ENOENT
	}
	color := parseMonitorColor(v, appearance)
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	visible := v["visible"].(bool)

	panel := ui.NewUiMonitor()
	monitor := &Monitor{
		target: target, val: val, eval: eval, name: name, size: size,
		visible: visible, pos: mathf.NewVec2(x, y), panel: panel,
		style: ui.MonitorStyle{
			Appearance: appearance, Label: label, Color: color,
			Dimensions: parseListMonitorDimensions(v),
		},
		isDirty: true, // Initial dirty state to ensure first render.
	}

	return monitor, nil
}

func parseMonitorAppearance(v coreproject.StageShape) ui.MonitorAppearance {
	if v["mode"] == "list" || v["mode"] == float64(monitorModeList) {
		return ui.MonitorAppearanceList
	}
	mode := int(v["mode"].(float64))
	style, _ := coreproject.ShapeValue(v, "style", "default").(string)
	if style == monitorStyleScratch {
		if mode == monitorModeLarge {
			return ui.MonitorAppearanceScratchLarge
		}
		return ui.MonitorAppearanceScratch
	}
	if mode == monitorModeDefault {
		return ui.MonitorAppearanceDefault
	}
	return ui.MonitorAppearanceDefaultLarge
}

func parseMonitorColor(v coreproject.StageShape, appearance ui.MonitorAppearance) mathf.Color {
	if color, err := mathf.NewColorAny(coreproject.ShapeValue(v, "color")); err == nil {
		return color
	}
	switch {
	case appearance == ui.MonitorAppearanceList:
		return mathf.NewColorRGBAi(0xff, 0x66, 0x1a, 0xff)
	case appearance.IsScratch():
		return mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)
	default:
		return mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)
	}
}

func (pself *Monitor) onUpdate(delta float64) {
	pself.updateTimer += delta
	due := pself.updateTimer >= monitorUpdateIntervalS
	if !pself.isDirty && !due {
		return
	}

	if due {
		pself.updateTimer = 0
	}

	// A getter can change visibility; apply that change on the next refresh.
	visible := pself.visible
	if visible {
		pself.panel.Render(pself.style, pself.eval())
		pself.panel.UpdateScale(pself.size)
		pself.panel.UpdatePos(pself.pos)
	}
	pself.panel.SetVisible(visible)
	pself.isDirty = false
}

// -----------------------------------------------------------------------------
// Evaluation
// -----------------------------------------------------------------------------
func getTarget(g reflect.Value, target string) (reflect.Value, int) {
	if target == "" {
		return g, 1 // spx.Game
	}
	if val := coreproject.FindFieldPtr(g, target, 0); val != nil {
		if _, ok := val.(Shape); ok {
			return reflect.ValueOf(val).Elem(), 2 // (spx.Sprite, *Game)
		}
	}
	return reflect.Value{}, -1
}

func buildMonitorEval(g reflect.Value, t, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
	target, from := getTarget(g, t)
	if from < 0 {
		return nil
	}
	name := strings.TrimPrefix(val, getVarPrefix)
	if appearance == ui.MonitorAppearanceList {
		if name == "" {
			return nil
		}
		if eval := coreproject.ResolveMemberValueEval(target, name, from); eval != nil {
			return func() ui.MonitorValue { return ui.MonitorValue{Items: listMonitorItems(eval())} }
		}
		return nil
	}
	if val == getVarPrefix {
		spxlog.Error("Bind monitor error: name is empty")
		return nil
	}
	if eval := coreproject.ResolveMemberStringEval(target, name, from); eval != nil {
		return func() ui.MonitorValue { return ui.MonitorValue{Text: eval()} }
	}
	spxlog.Error("Bind monitor error: cannot find property or method (getter): %s", name)
	return nil
}

// -----------------------------------------------------------------------------
// Visibility Control
// -----------------------------------------------------------------------------
func (pself *Monitor) setVisible(visible bool) {
	if visible == pself.visible {
		return
	}

	pself.visible = visible
	pself.isDirty = true
}

// -----------------------------------------------------------------------------
// Widget Methods
// -----------------------------------------------------------------------------
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
	pself.isDirty = true
}

func (pself *Monitor) Size() float64 {
	return pself.size
}

func (pself *Monitor) SetSize(size float64) {
	pself.size = size
	pself.isDirty = true
}

func (pself *Monitor) ChangeSize(delta float64) {
	pself.size += delta
	pself.isDirty = true
}
