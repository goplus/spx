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

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/ui"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
const (
	// Style selects the appearance of default and large monitors.
	// Slider and list monitors always use the Scratch appearance.
	monitorStyleCompatible = "scratch"

	getVarPrefix           = "getVar:"
	monitorUpdateIntervalS = 0.2
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
	updateInput func() bool
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

func (pself *Monitor) onUpdate(delta float64) {
	if pself.visible && pself.updateInput != nil && pself.updateInput() {
		pself.isDirty = true
	}
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
// Visibility Control
// -----------------------------------------------------------------------------
func (pself *Monitor) setVisible(visible bool) {
	if visible == pself.visible {
		return
	}

	pself.visible = visible
	pself.isDirty = true
}

func (pself *Monitor) setXYpos(x float64, y float64) {
	pself.pos = mathf.NewVec2(x, y)
	pself.isDirty = true
}

// -----------------------------------------------------------------------------
// Construction
// -----------------------------------------------------------------------------
func newMonitor(g reflect.Value, shape coreproject.StageShape, v coreproject.MonitorShape) (*Monitor, error) {
	appearance := monitorAppearance(v)
	binding, err := bindMonitor(g, v.Target, v.Val, appearance)
	if err != nil {
		return nil, err
	}
	color := parseMonitorColor(shape, appearance)

	panel := ui.NewUiMonitor()
	monitor := &Monitor{
		target: v.Target, val: v.Val, eval: binding.read, name: v.Name, size: v.Size,
		visible: v.Visible, pos: mathf.NewVec2(v.X, v.Y), panel: panel,
		style: ui.MonitorStyle{
			Appearance: appearance, Label: v.Label, Color: color,
			Dimensions: parseListMonitorDimensions(shape),
			Slider:     parseMonitorSlider(shape),
		},
		isDirty: true, // Initial dirty state to ensure first render.
	}
	if binding.write != nil {
		monitor.updateInput = func() bool {
			value, changed := panel.ReadSliderChange()
			return changed && binding.write(value)
		}
	}

	return monitor, nil
}
