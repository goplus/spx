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
	views     [monitorAppearanceCount]monitorView
	active    MonitorAppearance
	listItems []string
	slider    monitorSliderState

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
	GetSize(engine.Object) Vec2
	SetSize(engine.Object, Vec2)
	SetRange(engine.Object, float64, float64, float64, float64)
	GetRangeValue(engine.Object) float64
}

func (p MonitorAppearance) IsCompatible() bool {
	return p == MonitorAppearanceCompatible || p == MonitorAppearanceCompatibleLarge || p == MonitorAppearanceList || p == MonitorAppearanceSlider
}

// !!Warning: this method is called from the engine callback context
func (pself *UiMonitor) OnStart() {
	pself.bindViews(func(path string) *UiNode {
		return engine.BridgeBindUI[UiNode](pself.GetId(), path)
	})
}

func (pself *UiMonitor) SetVisible(isOn bool) {
	engine.Managers().UiMgr.SetVisible(pself.GetId(), isOn)
}

// getValueLabelCanvasScale returns the effective canvas scale of the value
// label, including the scale inherited from its UI ancestors and canvas layer.
// The width limit is calculated in Viewport coordinates and applied in the
// label's local coordinate system, so this scale is the conversion factor.
func (pself *UiMonitor) getValueLabelCanvasScale(view monitorView) float64 {
	if view.value == nil {
		return 0
	}

	scale := engine.Managers().UiMgr.GetCanvasScale(view.value.GetId())
	return math.Abs(float64(scale.X))
}

func (pself *UiMonitor) UpdateScale(monitorScale float64) {
	windowScale := engine.WindowScale()
	monitorScale *= windowScale
	engine.Managers().UiMgr.SetScale(pself.GetId(), engine.UniformVec2(monitorScale))
}

func (pself *UiMonitor) UpdatePos(worldPos Vec2) {
	uiPosition := ViewToUI(worldPos)
	engine.Managers().UiMgr.SetGlobalPosition(pself.GetId(), uiPosition)

	valueLabelCanvasScale := pself.getValueLabelCanvasScale(pself.views[MonitorAppearanceCompatibleLarge])

	viewportRect := engine.Managers().CameraMgr.GetViewportRect()

	pself.maxValueLabelLocalWidth = calculateMaxValueLabelLocalWidth(
		float64(viewportRect.Size.X),
		float64(uiPosition.X),
		valueLabelCanvasScale,
		scratchValueOnlyHorizontalLayoutOverhead,
	)
}

func (pself *UiMonitor) Render(style MonitorStyle, value MonitorValue) {
	pself.render(&engine.Managers().UiMgr, style, value)
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

	pself.renderScalarText(sink, appearance, view, style.Label, value.Text)
	if view.colorTarget != nil {
		sink.SetColor(view.colorTarget.GetId(), style.Color)
	}
}

// renderScalarText applies the viewport-derived width only to the
// ScratchValueOnly value label. Other appearances have different container
// margins and continue to use their scene layout without this constraint.
func (pself *UiMonitor) renderScalarText(sink monitorRenderSink, appearance MonitorAppearance, view monitorView, labelText, valueText string) {
	maxValueLabelLocalWidth := pself.maxValueLabelLocalWidth

	if maxValueLabelLocalWidth <= 0 || appearance != MonitorAppearanceCompatibleLarge {
		if view.label != nil {
			sink.SetText(view.label.GetId(), labelText)
		}
		if view.value != nil {
			sink.SetText(view.value.GetId(), valueText)
		}
		return
	}

	if view.label != nil {
		sink.SetTextMaxWidth(view.label.GetId(), labelText, maxValueLabelLocalWidth)
	}
	if view.value != nil {
		sink.SetTextMaxWidth(view.value.GetId(), valueText, maxValueLabelLocalWidth)
	}

	sink.SetSize(view.root.GetId(), Vec2{})
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
