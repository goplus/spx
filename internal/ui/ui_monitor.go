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
	"math"
	"slices"

	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v4/internal/engine"
)

type UiMonitor struct {
	UiNode
	style          MonitorStyle
	view           monitorView
	listItems      []string
	sliderPosition float64
	sliderReady    bool

	maxValueLabelLocalWidth float64
}

const (
	minValueLabelLocalWidth             = 1.0
	scratchValueOnlyOuterContentMarginX = 2.0
	scratchValueOnlyValueContentMarginX = 9.0

	// These values mirror the horizontal content margins in
	// UiMonitor.tscn: ScratchValueOnly's outer panel contributes 2+2 and
	// its C child contributes 9+9 in the LabelValue local coordinate system.
	scratchValueOnlyHorizontalLayoutOverhead = 2*scratchValueOnlyOuterContentMarginX +
		2*scratchValueOnlyValueContentMarginX
)

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
	MonitorAppearanceCompatible
	MonitorAppearanceCompatibleLarge
	MonitorAppearanceList
	MonitorAppearanceSlider
	monitorAppearanceCount
)

type monitorView struct {
	root  *UiNode
	value *UiNode
	input *UiNode
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
	MonitorAppearanceCompatible: {
		root:        "ScratchBG",
		label:       "ScratchBG/H/LabelMargin/LabelName",
		value:       "ScratchBG/H/ValueMargin/C/LabelValue",
		colorTarget: "ScratchBG/H/ValueMargin/C",
	},
	MonitorAppearanceCompatibleLarge: {
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
	SetTextMaxWidth(engine.Object, string, float64) float64
	SetColor(engine.Object, Color)
	SetListItems(engine.Object, string, engine.Array, Color)
	SetSize(engine.Object, Vec2)
	SetRange(engine.Object, float64, float64, float64, float64)
	GetRangeValue(engine.Object) float64
}

func NewUiMonitor(style MonitorStyle) (panel *UiMonitor) {
	engine.WaitMainThread(func() {
		panel = engine.CreateEngineUIForType[UiMonitor]("")
		panel.initialize(&engine.Managers().UiMgr, style, func(path string) *UiNode {
			return engine.BridgeBindUI[UiNode](panel.GetId(), path)
		})
	})
	return panel
}

func (p MonitorAppearance) IsCompatible() bool {
	return p == MonitorAppearanceCompatible || p == MonitorAppearanceCompatibleLarge || p == MonitorAppearanceList || p == MonitorAppearanceSlider
}

func (p *UiMonitor) SetVisible(isOn bool) {
	engine.Managers().UiMgr.SetVisible(p.GetId(), isOn)
}

func (p *UiMonitor) UpdateScale(monitorScale float64) {
	windowScale := engine.WindowScale()
	monitorScale *= windowScale
	engine.Managers().UiMgr.SetScale(p.GetId(), engine.UniformVec2(monitorScale))
}

func (p *UiMonitor) UpdatePos(worldPos Vec2) {
	uiPosition := ViewToUI(worldPos)
	engine.Managers().UiMgr.SetGlobalPosition(p.GetId(), uiPosition)

	if p.style.Appearance != MonitorAppearanceCompatibleLarge {
		return
	}
	// Convert the viewport width to the selected value label's local units.
	valueLabelCanvasScale := math.Abs(float64(engine.Managers().UiMgr.GetCanvasScale(p.view.value.GetId()).X))

	viewportRect := engine.Managers().CameraMgr.GetViewportRect()

	p.maxValueLabelLocalWidth = calculateMaxValueLabelLocalWidth(
		float64(viewportRect.Size.X),
		float64(uiPosition.X),
		valueLabelCanvasScale,
		scratchValueOnlyHorizontalLayoutOverhead,
	)
}

func (p *UiMonitor) Render(value MonitorValue) {
	p.render(&engine.Managers().UiMgr, value)
}

// initialize runs on the engine thread before the monitor can be used.
func (p *UiMonitor) initialize(sink monitorRenderSink, style MonitorStyle, bind func(string) *UiNode) {
	style.Appearance = normalizeMonitorAppearance(style.Appearance)
	p.style = style
	spec := monitorViewSpecs[style.Appearance]
	optional := func(path string) *UiNode {
		if path == "" {
			return nil
		}
		return bind(path)
	}
	p.view = monitorView{root: bind(spec.root), value: optional(spec.value), input: optional(spec.input)}
	// BG is the prefab's only initially visible variant.
	if style.Appearance != MonitorAppearanceDefault {
		sink.SetVisible(bind(monitorViewSpecs[MonitorAppearanceDefault].root).GetId(), false)
	}
	sink.SetVisible(p.view.root.GetId(), true)
	if spec.label != "" {
		sink.SetText(bind(spec.label).GetId(), style.Label)
	}
	if spec.colorTarget != "" {
		sink.SetColor(bind(spec.colorTarget).GetId(), style.Color)
	}
	if style.Appearance == MonitorAppearanceList {
		sink.SetSize(p.view.root.GetId(), style.Dimensions)
		sink.SetListItems(p.view.root.GetId(), style.Label, []string(nil), style.Color)
	}
}

func (p *UiMonitor) render(sink monitorRenderSink, value MonitorValue) {
	if p.style.Appearance == MonitorAppearanceList {
		if !slices.Equal(p.listItems, value.Items) {
			sink.SetListItems(p.view.root.GetId(), p.style.Label, value.Items, p.style.Color)
			p.listItems = slices.Clone(value.Items)
		}
		return
	}
	if p.style.Appearance == MonitorAppearanceSlider {
		p.renderSlider(sink, value.Text)
	}
	if p.style.Appearance == MonitorAppearanceCompatibleLarge && p.maxValueLabelLocalWidth > 0 {
		sink.SetTextMaxWidth(p.view.value.GetId(), value.Text, p.maxValueLabelLocalWidth)
		sink.SetSize(p.view.root.GetId(), Vec2{})
	} else {
		sink.SetText(p.view.value.GetId(), value.Text)
	}
}

// calculateMaxValueLabelLocalWidth converts the remaining Viewport width into
// the LabelValue local coordinate system. The layout overhead is subtracted
// after conversion because it is expressed in the scene's local units.
func calculateMaxValueLabelLocalWidth(viewportWidth, monitorLeftX, valueLabelCanvasScale, localHorizontalOverhead float64) float64 {
	if math.IsNaN(viewportWidth) || math.IsInf(viewportWidth, 0) || viewportWidth <= 0 ||
		math.IsNaN(monitorLeftX) || math.IsInf(monitorLeftX, 0) ||
		math.IsNaN(valueLabelCanvasScale) || math.IsInf(valueLabelCanvasScale, 0) || valueLabelCanvasScale <= 0 ||
		math.IsNaN(localHorizontalOverhead) || math.IsInf(localHorizontalOverhead, 0) || localHorizontalOverhead < 0 {
		return 0
	}

	available := math.Min(viewportWidth, viewportWidth-monitorLeftX)

	return math.Max(minValueLabelLocalWidth, available/valueLabelCanvasScale-localHorizontalOverhead)
}

func normalizeMonitorAppearance(appearance MonitorAppearance) MonitorAppearance {
	if appearance >= monitorAppearanceCount {
		return MonitorAppearanceDefault
	}
	return appearance
}
