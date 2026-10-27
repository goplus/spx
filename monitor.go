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
	"math"
	"reflect"
	"strings"
	"syscall"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	spxlog "github.com/goplus/spx/v4/internal/log"
	"github.com/goplus/spx/v4/internal/tools"
	"github.com/goplus/spx/v4/internal/ui"
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
	evaluating  bool
	refresh     uint64
}

// monitorPanel separates widget lifecycle from the engine-backed view.
type monitorPanel interface {
	Render(ui.MonitorStyle, ui.MonitorValue)
	SetVisible(bool)
	UpdateScale(float64)
	UpdatePos(mathf.Vec2)
}

type preparedMonitor struct {
	config coreproject.MonitorShape
	style  ui.MonitorStyle
}

// monitorBinding resolves reads and optional writes in the same variable scope.
type monitorBinding struct {
	read  func() ui.MonitorValue
	write func(float64) bool
}

// -----------------------------------------------------------------------------
// Widget Methods
// -----------------------------------------------------------------------------
func (p *Monitor) GetName() WidgetName {
	return p.name
}

func (p *Monitor) Visible() bool {
	return p.visible
}

func (p *Monitor) Show() {
	p.setVisible(true)
}

func (p *Monitor) Hide() {
	p.setVisible(false)
}

func (p *Monitor) Xpos() float64 {
	return p.pos.X
}

func (p *Monitor) Ypos() float64 {
	return p.pos.Y
}

func (p *Monitor) SetXpos(x float64) {
	p.setXYpos(x, p.pos.Y)
}

func (p *Monitor) SetYpos(y float64) {
	p.setXYpos(p.pos.X, y)
}

func (p *Monitor) SetXYpos(x float64, y float64) {
	p.setXYpos(x, y)
}

func (p *Monitor) ChangeXpos(dx float64) {
	p.setXYpos(p.pos.X+dx, p.pos.Y)
}

func (p *Monitor) ChangeYpos(dy float64) {
	p.setXYpos(p.pos.X, p.pos.Y+dy)
}

func (p *Monitor) ChangeXYpos(dx float64, dy float64) {
	p.setXYpos(p.pos.X+dx, p.pos.Y+dy)
}

func (p *Monitor) Size() float64 {
	return p.size
}

func (p *Monitor) SetSize(size float64) {
	p.size = size
	p.isDirty = true
}

func (p *Monitor) ChangeSize(delta float64) {
	p.size += delta
	p.isDirty = true
}

func (p *Monitor) onUpdate(delta float64) {
	if p.visible && p.updateInput != nil && p.updateInput() {
		p.isDirty = true
	}
	p.updateTimer += delta
	due := p.updateTimer >= monitorUpdateIntervalS
	if !p.isDirty && !due {
		return
	}

	if due {
		p.updateTimer = 0
	}
	p.refresh++
	refresh := p.refresh

	// Snapshot visibility so synchronous getter changes apply on the next refresh.
	visible := p.visible
	hasValue := visible && !p.evaluating
	var value ui.MonitorValue
	if hasValue {
		value = p.evaluate()
	}
	if refresh != p.refresh {
		// A later refresh already applied the UI state while the getter waited.
		// Publish only its value, preserving visibility and pending layout edits.
		if p.visible {
			p.panel.Render(p.style, value)
		}
		return
	}
	if visible {
		p.panel.UpdateScale(p.size)
		p.panel.UpdatePos(p.pos)
		if hasValue {
			p.panel.Render(p.style, value)
		}
	}
	p.panel.SetVisible(visible)
	p.isDirty = false
}

// Only evaluation is exclusive; waiting getters do not block lightweight UI.
func (p *Monitor) evaluate() ui.MonitorValue {
	p.evaluating = true
	completed := false
	defer func() {
		p.evaluating = false
		if !completed {
			p.isDirty = true // Retry canceled getters even after a UI refresh.
		}
	}()
	value := p.eval()
	completed = true
	return value
}

// -----------------------------------------------------------------------------
// Visibility Control
// -----------------------------------------------------------------------------
func (p *Monitor) setVisible(visible bool) {
	if visible == p.visible {
		return
	}

	p.visible = visible
	p.isDirty = true
}

func (p *Monitor) setXYpos(x float64, y float64) {
	p.pos = mathf.NewVec2(x, y)
	p.isDirty = true
}

// -----------------------------------------------------------------------------
// Construction
// -----------------------------------------------------------------------------
func newMonitor(g reflect.Value, prepared preparedMonitor) (*Monitor, error) {
	v := prepared.config
	appearance := prepared.style.Appearance
	binding, err := bindMonitor(g, v.Target, v.Val, appearance)
	if err != nil {
		return nil, err
	}

	panel := ui.NewUiMonitor()
	monitor := &Monitor{
		target: v.Target, val: v.Val, eval: binding.read, name: v.Name, size: v.Size,
		visible: v.Visible, pos: mathf.NewVec2(v.X, v.Y), panel: panel,
		style:   prepared.style,
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

func prepareMonitor(shape coreproject.StageShape) (preparedMonitor, error) {
	config, err := coreproject.ParseMonitorShape(shape)
	if err != nil {
		return preparedMonitor{}, err
	}
	appearance := monitorAppearance(config)
	return preparedMonitor{config: config, style: ui.MonitorStyle{
		Appearance: appearance, Label: config.Label, Color: parseMonitorColor(shape, appearance),
		Dimensions: parseListMonitorDimensions(shape), Slider: parseMonitorSlider(shape),
	}}, nil
}

func monitorAppearance(v coreproject.MonitorShape) ui.MonitorAppearance {
	if v.Mode == coreproject.MonitorModeSlider {
		return ui.MonitorAppearanceSlider
	}
	if v.Mode == coreproject.MonitorModeList {
		return ui.MonitorAppearanceList
	}
	mode := int(v.Mode)
	if v.Style == monitorStyleCompatible {
		if mode == coreproject.MonitorModeLarge {
			return ui.MonitorAppearanceCompatibleLarge
		}
		return ui.MonitorAppearanceCompatible
	}
	if mode == coreproject.MonitorModeDefault {
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
	case appearance.IsCompatible():
		return mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)
	default:
		return mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)
	}
}

func parseListMonitorDimensions(v coreproject.StageShape) mathf.Vec2 {
	dimension := func(key string, fallback, minimum float64) float64 {
		value := monitorNumber(v, key, fallback)
		if value <= 0 {
			return fallback
		}
		return math.Max(value, minimum)
	}
	return mathf.NewVec2(dimension("width", 100, 100), dimension("height", 200, 60))
}

func parseMonitorSlider(v coreproject.StageShape) ui.MonitorSlider {
	min, max := monitorNumber(v, "sliderMin", 0), monitorNumber(v, "sliderMax", 100)
	if min > max {
		min, max = max, min
	}
	step := 1.0
	if discrete, ok := v["isDiscrete"].(bool); ok && !discrete {
		step = 0.01
	}
	if math.IsInf((max-min)/step, 0) || math.IsInf(min/step, 0) || math.IsInf(max/step, 0) {
		min, max = 0, 100
	}
	return ui.MonitorSlider{Min: min, Max: max, Step: step}
}

// monitorNumber reads finite dimensions and range limits. Monitor scale retains
// its existing parsing rules and deliberately does not use this helper.
func monitorNumber(v coreproject.StageShape, key string, fallback float64) float64 {
	value, ok := tools.GetFloat(v[key])
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
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

func bindMonitor(g reflect.Value, targetName, val string, appearance ui.MonitorAppearance) (monitorBinding, error) {
	target, from := getTarget(g, targetName)
	if from < 0 {
		return monitorBinding{}, syscall.ENOENT
	}
	binding := monitorBinding{read: buildMonitorEval(target, from, val, appearance)}
	if binding.read == nil {
		return monitorBinding{}, syscall.ENOENT
	}
	if appearance == ui.MonitorAppearanceSlider {
		binding.write = coreproject.ResolveMemberNumberSetter(target, strings.TrimPrefix(val, getVarPrefix), from)
		if binding.write == nil {
			return monitorBinding{}, syscall.EINVAL
		}
	}
	return binding, nil
}

func buildMonitorEval(target reflect.Value, from int, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
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

// listMonitorItems snapshots the current contents without retaining the source
// slice or list. Replacing or growing a list is reflected on the next refresh.
func listMonitorItems(value any) []string {
	switch list := value.(type) {
	case List:
		return listMonitorItems(list.data)
	case *List:
		if list == nil {
			return nil
		}
		return listMonitorItems(list.data)
	}
	list := reflect.ValueOf(value)
	for list.IsValid() && (list.Kind() == reflect.Pointer || list.Kind() == reflect.Interface) {
		list = list.Elem()
	}
	if !list.IsValid() || (list.Kind() != reflect.Slice && list.Kind() != reflect.Array) {
		return nil
	}
	items := make([]string, list.Len())
	for i := range items {
		items[i] = toString(list.Index(i).Interface())
	}
	return items
}
