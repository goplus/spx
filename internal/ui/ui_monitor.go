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

package ui

//lint:file-ignore ST1001 UI glue intentionally dot-imports mathf to mirror engine type names.

import (
	"slices"

	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v3/internal/engine"
)

type UiMonitor struct {
	UiNode
	views     [monitorAppearanceCount]monitorView
	active    MonitorAppearance
	listItems []string
	slider    monitorSliderState
}

// MonitorStyle describes the fixed presentation of a stage monitor.
type MonitorStyle struct {
	Appearance MonitorAppearance
	Label      string
	Color      Color
	Dimensions Vec2
	Slider     MonitorSlider
}

// MonitorValue keeps list items separate from scalar text. The appearance,
// rather than a nil check on Items, determines how an empty list is displayed.
type MonitorValue struct {
	Text  string
	Items []string
}

type MonitorAppearance uint8

const (
	MonitorAppearanceDefault MonitorAppearance = iota
	MonitorAppearanceDefaultLarge
	MonitorAppearanceScratch
	MonitorAppearanceScratchLarge
	MonitorAppearanceList
	MonitorAppearanceSlider
	monitorAppearanceCount
)

type monitorView struct {
	root        *UiNode
	label       *UiNode
	value       *UiNode
	colorTarget *UiNode
	input       *UiNode
}

type monitorViewSpec struct {
	root        string
	label       string
	value       string
	colorTarget string
	input       string
}

var monitorViewSpecs = [monitorAppearanceCount]monitorViewSpec{
	MonitorAppearanceDefault: {
		root:  "BG",
		label: "BG/H/LabelName",
		value: "BG/H/C/H/LabelValue",
	},
	MonitorAppearanceDefaultLarge: {
		root:  "ValueOnly",
		value: "ValueOnly/LabelValue",
	},
	MonitorAppearanceScratch: {
		root:        "ScratchBG",
		label:       "ScratchBG/H/LabelMargin/LabelName",
		value:       "ScratchBG/H/ValueMargin/C/LabelValue",
		colorTarget: "ScratchBG/H/ValueMargin/C",
	},
	MonitorAppearanceScratchLarge: {
		root:        "ScratchValueOnly",
		value:       "ScratchValueOnly/C/LabelValue",
		colorTarget: "ScratchValueOnly/C",
	},
	MonitorAppearanceList: {root: "ScratchList"},
	MonitorAppearanceSlider: {
		root:        "ScratchSlider",
		label:       "ScratchSlider/V/H/LabelMargin/LabelName",
		value:       "ScratchSlider/V/H/ValueMargin/C/LabelValue",
		colorTarget: "ScratchSlider/V/H/ValueMargin/C",
		input:       "ScratchSlider/V/Slider",
	},
}

type monitorRenderSink interface {
	SetVisible(engine.Object, bool)
	SetText(engine.Object, string)
	SetColor(engine.Object, Color)
	SetListItems(engine.Object, string, engine.Array, Color)
	SetSize(engine.Object, Vec2)
	SetRange(engine.Object, float64, float64, float64, float64)
	GetRangeValue(engine.Object) float64
}

func (p MonitorAppearance) IsScratch() bool {
	return p == MonitorAppearanceScratch || p == MonitorAppearanceScratchLarge || p == MonitorAppearanceList || p == MonitorAppearanceSlider
}

// !!Warning: this method is called from the engine callback context
func (pself *UiMonitor) OnStart() {
	pself.bindViews(func(path string) *UiNode {
		return engine.BridgeBindUI[UiNode](pself.GetId(), path)
	})
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	mgr.UiMgr.SetVisible(pself.GetId(), isOn)
}

func (pself *UiMonitor) UpdateScale(x float64) {
	x *= engine.WindowScale()
	mgr.UiMgr.SetScale(pself.GetId(), engine.UniformVec2(x))
}

func (pself *UiMonitor) UpdatePos(wpos Vec2) {
	mgr.UiMgr.SetGlobalPosition(pself.GetId(), ViewToUI(wpos))
}

func (pself *UiMonitor) Render(style MonitorStyle, value MonitorValue) {
	pself.render(&mgr.UiMgr, style, value)
}

func NewUiMonitor() *UiMonitor {
	return engine.NewUiNode[UiMonitor]()
}

func (pself *UiMonitor) bindViews(bind func(string) *UiNode) {
	optional := func(path string) *UiNode {
		if path == "" {
			return nil
		}
		return bind(path)
	}
	for i, spec := range monitorViewSpecs {
		pself.views[i] = monitorView{
			root:        bind(spec.root),
			value:       optional(spec.value),
			label:       optional(spec.label),
			colorTarget: optional(spec.colorTarget),
			input:       optional(spec.input),
		}
	}
	pself.active = monitorAppearanceCount
}

func (pself *UiMonitor) render(sink monitorRenderSink, style MonitorStyle, value MonitorValue) {
	appearance := normalizeMonitorAppearance(style.Appearance)
	changed := pself.active != appearance
	if changed {
		for i, view := range pself.views {
			sink.SetVisible(view.root.GetId(), MonitorAppearance(i) == appearance)
		}
		pself.active = appearance
	}

	if appearance == MonitorAppearanceSlider {
		pself.renderSlider(sink, style.Slider, value.Text)
	}

	view := pself.views[appearance]
	if appearance == MonitorAppearanceList {
		id := view.root.GetId()
		sink.SetSize(id, style.Dimensions)
		// The style is fixed for a monitor; only changed items cross the bridge.
		if changed || !slices.Equal(pself.listItems, value.Items) {
			sink.SetListItems(id, style.Label, value.Items, style.Color)
			pself.listItems = slices.Clone(value.Items)
		}
		return
	}

	if view.label != nil {
		sink.SetText(view.label.GetId(), style.Label)
	}
	if view.value != nil {
		sink.SetText(view.value.GetId(), value.Text)
	}
	if view.colorTarget != nil {
		sink.SetColor(view.colorTarget.GetId(), style.Color)
	}
}

func normalizeMonitorAppearance(appearance MonitorAppearance) MonitorAppearance {
	if appearance >= monitorAppearanceCount {
		return MonitorAppearanceDefault
	}
	return appearance
}
