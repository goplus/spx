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
	monitorStyleScratch    = "scratch"
)

// -----------------------------------------------------------------------------
// Monitor
// -----------------------------------------------------------------------------
type Monitor struct {
	game        *Game
	name        WidgetName
	size        float64
	target      string
	val         string
	eval        func() string
	appearance  ui.MonitorAppearance
	color       mathf.Color
	pos         mathf.Vec2
	label       string
	visible     bool
	panel       *ui.UiMonitor
	isDirty     bool
	updateTimer float64
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
	eval := buildMonitorEval(g, target, val)
	if eval == nil {
		return nil, syscall.ENOENT
	}
	appearance := parseMonitorAppearance(v)
	color, err := mathf.NewColorAny(coreproject.ShapeValue(v, "color"))
	if err != nil {
		if appearance.IsScratch() {
			color = mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)
		} else {
			color = mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)
		}
	}
	label := v["label"].(string)
	x := v["x"].(float64)
	y := v["y"].(float64)
	visible := v["visible"].(bool)

	panel := ui.NewUiMonitor()
	monitor := &Monitor{
		target: target, val: val, eval: eval, name: name, size: size,
		visible: visible, appearance: appearance, color: color,
		pos: mathf.NewVec2(x, y), label: label, panel: panel,
		isDirty: true, // Initial dirty state to ensure first render.
	}

	return monitor, nil
}

func parseMonitorAppearance(v coreproject.StageShape) ui.MonitorAppearance {
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

func (pself *Monitor) onUpdate(delta float64) {
	pself.updateTimer += delta
	needsUpdate := pself.isDirty || pself.updateTimer >= monitorUpdateIntervalS
	if !needsUpdate {
		return
	}

	if pself.updateTimer >= monitorUpdateIntervalS {
		pself.updateTimer = 0
	}

	if !pself.visible {
		pself.panel.SetVisible(false)
		pself.setDirtyFlag(false)
		return
	}
	val := pself.eval()
	pself.panel.Render(pself.appearance, pself.label, val, pself.color)
	pself.panel.UpdateScale(pself.size)
	pself.panel.UpdatePos(pself.pos)
	pself.panel.SetVisible(true)
	pself.setDirtyFlag(false)
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

		if eval := coreproject.ResolveMemberStringEval(target, name, from); eval != nil {
			return eval
		}
		spxlog.Error("Bind monitor error: cannot find property or method (getter): %s", name)
	default:
		name := val
		if eval := coreproject.ResolveMemberStringEval(target, name, from); eval != nil {
			return eval
		}
		spxlog.Error("Bind monitor error: cannot find property or method (getter): %s", name)
	}
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
	pself.setDirtyFlag(true)
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

// -----------------------------------------------------------------------------
// Dirty State
// -----------------------------------------------------------------------------
func (pself *Monitor) setDirtyFlag(isDirty bool) {
	pself.isDirty = isDirty
}
