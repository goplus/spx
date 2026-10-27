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

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type monitorRenderSpy struct {
	visible      map[engine.Object]bool
	text         map[engine.Object]string
	color        map[engine.Object]mathf.Color
	items        map[engine.Object][]string
	size         map[engine.Object]mathf.Vec2
	maxWidths    map[engine.Object][]float64
	naturalWidth map[engine.Object]map[string]float64
	appliedWidth map[engine.Object]float64
	layouts      map[engine.Object]monitorRenderSpyLayout
	listCalls    int
	ranges       map[engine.Object]float64
}

type monitorRenderSpyLayout struct {
	label, value       engine.Object
	horizontalOverhead float64
	minimum            float64
}

type monitorEngineUiSpy struct {
	gdx.IUiMgr
	canvasScale mathf.Vec2
	visible     map[gdx.Object]bool
	texts       map[gdx.Object]string
	maxWidths   map[gdx.Object]float64
	colors      map[gdx.Object]mathf.Color
	sizes       map[gdx.Object]mathf.Vec2
	scales      map[gdx.Object]mathf.Vec2
	positions   map[gdx.Object]mathf.Vec2
}

type parsedMonitorScene struct {
	nodes     map[string]string
	nodeTypes map[string]string
	resources map[string]string
	externals map[string]string
}

var (
	monitorNodeHeader     = regexp.MustCompile(`^\[node name="([^"]+)" type="([^"]+)"(?: parent="([^"]+)")?[^]]*\]$`)
	monitorResourceHeader = regexp.MustCompile(`^\[sub_resource [^]]*id="([^"]+)"\]$`)
	monitorExternalHeader = regexp.MustCompile(`^\[ext_resource [^]]*id="([^"]+)"\]$`)
)

func TestUiMonitorRender(t *testing.T) {
	color := mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)
	for appearance := MonitorAppearanceDefault; appearance < MonitorAppearanceList; appearance++ {
		t.Run(monitorViewSpecs[appearance].root, func(t *testing.T) {
			panel, _ := newBoundMonitor(t)
			spy := newMonitorRenderSpy()

			panel.render(spy, MonitorStyle{Appearance: appearance, Label: "score", Color: color}, MonitorValue{Text: "42"})
			if len(spy.visible) != len(panel.views) {
				t.Fatalf("visibility updates = %d, want %d", len(spy.visible), len(panel.views))
			}
			for i, view := range panel.views {
				if got, want := spy.visible[view.root.GetId()], MonitorAppearance(i) == appearance; got != want {
					t.Errorf("view %d visibility = %v, want %v", i, got, want)
				}
			}
			assertMonitorPayload(t, spy, panel.views[appearance], "score", "42", color)

			spy.reset()
			panel.render(spy, MonitorStyle{Appearance: appearance, Label: "points", Color: color}, MonitorValue{Text: "43"})
			if len(spy.visible) != 0 {
				t.Errorf("unchanged appearance updated visibility: %v", spy.visible)
			}
			assertMonitorPayload(t, spy, panel.views[appearance], "points", "43", color)
		})
	}
}

func TestUiMonitorInvalidAppearanceFallsBackToDefault(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	spy := newMonitorRenderSpy()
	panel.render(spy, MonitorStyle{Appearance: MonitorAppearance(255), Label: "score"}, MonitorValue{Text: "0"})

	if panel.active != MonitorAppearanceDefault {
		t.Fatalf("active appearance = %d, want default", panel.active)
	}
}

func TestCalculateMaxValueLabelLocalWidth(t *testing.T) {
	tests := []struct {
		name                                                     string
		viewportWidth, monitorLeftX, valueLabelCanvasScale, want float64
	}{
		{name: "left edge", viewportWidth: 480, monitorLeftX: 5, valueLabelCanvasScale: 0.45, want: (480-5)/0.45 - scratchValueOnlyHorizontalLayoutOverhead},
		{name: "middle", viewportWidth: 480, monitorLeftX: 240, valueLabelCanvasScale: 0.45, want: (480-240)/0.45 - scratchValueOnlyHorizontalLayoutOverhead},
		{name: "left of viewport", viewportWidth: 480, monitorLeftX: -20, valueLabelCanvasScale: 0.45, want: 480/0.45 - scratchValueOnlyHorizontalLayoutOverhead},
		{name: "past right edge", viewportWidth: 480, monitorLeftX: 500, valueLabelCanvasScale: 0.45, want: minValueLabelLocalWidth},
		{name: "invalid viewport", viewportWidth: 0, monitorLeftX: 0, valueLabelCanvasScale: 0.45, want: 0},
		{name: "invalid scale", viewportWidth: 480, monitorLeftX: 0, valueLabelCanvasScale: 0, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := calculateMaxValueLabelLocalWidth(
				test.viewportWidth,
				test.monitorLeftX,
				test.valueLabelCanvasScale,
				scratchValueOnlyHorizontalLayoutOverhead,
			)
			if math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("max value label width = %v, want %v", got, test.want)
			}
		})
	}
}

func TestUiMonitorScratchValueOnlyWrapsValueLabel(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	view := panel.views[MonitorAppearanceCompatibleLarge]
	spy := newMonitorRenderSpy()
	spy.naturalWidth[view.value.GetId()] = map[string]float64{
		"a long monitor value": 500,
		"short":                80,
	}
	spy.layouts[view.root.GetId()] = monitorRenderSpyLayout{
		value: view.value.GetId(), horizontalOverhead: scratchValueOnlyHorizontalLayoutOverhead, minimum: 111,
	}
	panel.maxValueLabelLocalWidth = 300

	style := MonitorStyle{Appearance: MonitorAppearanceCompatibleLarge}
	panel.render(spy, style, MonitorValue{Text: "a long monitor value"})
	if got := spy.maxWidths[view.value.GetId()]; !slices.Equal(got, []float64{300}) {
		t.Fatalf("value width limits = %v, want [300]", got)
	}
	if got := float64(spy.size[view.root.GetId()].X); got != 300+scratchValueOnlyHorizontalLayoutOverhead {
		t.Fatalf("wrapped root width = %v, want %v", got, 300+scratchValueOnlyHorizontalLayoutOverhead)
	}
	if got := spy.text[view.value.GetId()]; got != "a long monitor value" {
		t.Fatalf("wrapped text = %q, want original text", got)
	}

	spy.reset()
	panel.render(spy, style, MonitorValue{Text: "short"})
	if got := spy.maxWidths[view.value.GetId()]; !slices.Equal(got, []float64{300}) {
		t.Fatalf("short value width limits = %v, want [300]", got)
	}
	if got := float64(spy.size[view.root.GetId()].X); got != 111 {
		t.Fatalf("short root width = %v, want authored minimum 111", got)
	}
}

func TestUiMonitorOtherAppearancesDoNotUseScratchValueOnlyWidth(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	view := panel.views[MonitorAppearanceCompatible]
	spy := newMonitorRenderSpy()
	panel.maxValueLabelLocalWidth = 400

	panel.render(spy, MonitorStyle{Appearance: MonitorAppearanceCompatible, Label: "score"}, MonitorValue{Text: "value"})
	if got := spy.maxWidths[view.value.GetId()]; len(got) != 0 {
		t.Fatalf("value width limits = %v, want no width limit", got)
	}
	if got := spy.maxWidths[view.label.GetId()]; len(got) != 0 {
		t.Fatalf("label width limits = %v, want no width limit", got)
	}
	if got := spy.text[view.label.GetId()]; got != "score" {
		t.Fatalf("label text = %q, want score", got)
	}
	if got := spy.text[view.value.GetId()]; got != "value" {
		t.Fatalf("value text = %q, want value", got)
	}
}

func TestUiMonitorRenderScalarTextConstrainsBothLabels(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	view := panel.views[MonitorAppearanceCompatibleLarge]
	label := &UiNode{}
	label.SetId(99)
	view.label = label
	spy := newMonitorRenderSpy()
	panel.maxValueLabelLocalWidth = 120

	panel.renderScalarText(spy, MonitorAppearanceCompatibleLarge, view, "name", "value")

	if got := spy.maxWidths[label.GetId()]; !slices.Equal(got, []float64{120}) {
		t.Fatalf("label width limits = %v, want [120]", got)
	}
	if got := spy.maxWidths[view.value.GetId()]; !slices.Equal(got, []float64{120}) {
		t.Fatalf("value width limits = %v, want [120]", got)
	}
}

func TestUiMonitorEngineTransformsAndRender(t *testing.T) {
	mgr := setupMonitorEngine(t)
	panel, _ := newBoundMonitor(t)
	panel.SetId(100)
	view := panel.views[MonitorAppearanceCompatibleLarge]

	if got := panel.getValueLabelCanvasScale(monitorView{}); got != 0 {
		t.Fatalf("nil value canvas scale = %v, want 0", got)
	}
	if got := panel.getValueLabelCanvasScale(view); got != 0.5 {
		t.Fatalf("value canvas scale = %v, want 0.5", got)
	}

	engine.SetWindowScale(2)
	panel.UpdateScale(1.5)
	if got := mgr.scales[panel.GetId()]; got != mathf.NewVec2(3, 3) {
		t.Fatalf("monitor scale = %v, want (3, 3)", got)
	}

	engine.SetWindowScale(1)
	panel.UpdatePos(mathf.NewVec2(10, 20))
	wantPosition := mathf.NewVec2(240, 140)
	if got := mgr.positions[panel.GetId()]; got != wantPosition {
		t.Fatalf("monitor position = %v, want %v", got, wantPosition)
	}
	if got, want := panel.maxValueLabelLocalWidth, 458.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("max value label width = %v, want %v", got, want)
	}

	panel.Render(MonitorStyle{Appearance: MonitorAppearanceCompatibleLarge, Color: mathf.Color{}}, MonitorValue{Text: "value"})
	if got := mgr.maxWidths[view.value.GetId()]; math.Abs(got-458) > 1e-9 {
		t.Fatalf("rendered value width = %v, want 458", got)
	}
	if got := mgr.sizes[view.root.GetId()]; got != (mathf.Vec2{}) {
		t.Fatalf("rendered root size = %v, want reset size", got)
	}
}

func TestUiMonitorSceneContract(t *testing.T) {
	scene := readUITemplateScene(t, "UiMonitor")
	parsed := parseMonitorScene(t, scene)

	_, bound := newBoundMonitor(t)
	for path := range bound {
		if _, ok := parsed.nodes[path]; !ok {
			t.Errorf("UiMonitor scene is missing bound node %q", path)
		}
	}
	assertNodeType := func(path, want string) {
		if got := parsed.nodeTypes[path]; got != want {
			t.Errorf("bound node %q type = %q, want %q", path, got, want)
		}
	}
	assertNodeType("ScratchSlider/V/Slider", "HSlider")
	for _, spec := range monitorViewSpecs {
		if spec.root == "ScratchList" {
			assertNodeType(spec.root, "SpxListMonitor")
			continue
		}
		assertNodeType(spec.root, "PanelContainer")
		assertNodeType(spec.value, "Label")
		if spec.label != "" {
			assertNodeType(spec.label, "Label")
		}
		if spec.colorTarget != "" {
			assertNodeType(spec.colorTarget, "PanelContainer")
		}
	}
	if got := monitorSceneProperty(parsed.nodes["."], "visible"); got != "false" {
		t.Errorf("monitor root visibility = %q, want false", got)
	}
	for _, path := range []string{"ValueOnly", "ScratchBG", "ScratchValueOnly", "ScratchList", "ScratchSlider"} {
		if got := monitorSceneProperty(parsed.nodes[path], "visible"); got != "false" {
			t.Errorf("%s initial visibility = %q, want false", path, got)
		}
	}
	assertMonitorSceneProperties(t, parsed.nodes["ValueOnly"], map[string]string{
		"custom_minimum_size": "Vector2(111, 54)",
		"scale":               "Vector2(0.45, 0.45)",
	})
	assertMonitorSceneProperties(t, parsed.nodes["BG"], map[string]string{
		"scale": "Vector2(0.45, 0.45)",
	})
	assertMonitorSceneProperties(t, parsed.nodes["ValueOnly/LabelValue"], map[string]string{
		"theme_override_colors/font_color":    "Color(1, 1, 1, 1)",
		"theme_override_font_sizes/font_size": "36",
	})
	assertMonitorSceneProperties(t, parsed.nodes["ScratchValueOnly/C"], map[string]string{
		"custom_minimum_size": "Vector2(107, 50)",
	})
	assertMonitorSceneProperties(t, parsed.nodes["ScratchBG"], map[string]string{
		"scale": "Vector2(0.45, 0.45)",
	})
	assertMonitorSceneProperties(t, parsed.nodes["ScratchBG/H/ValueMargin/C"], map[string]string{
		"custom_minimum_size": "Vector2(89, 0)",
	})
	for _, path := range []string{"ScratchBG/H/LabelMargin/LabelName", "ScratchBG/H/ValueMargin/C/LabelValue"} {
		assertMonitorSceneProperties(t, parsed.nodes[path], map[string]string{
			"theme_override_font_sizes/font_size": "26",
		})
	}
	assertMonitorSceneProperties(t, parsed.nodes["ScratchValueOnly"], map[string]string{
		"scale": "Vector2(0.45, 0.45)",
	})
	assertMonitorSceneProperties(t, parsed.nodes["ScratchValueOnly/C/LabelValue"], map[string]string{
		"theme_override_font_sizes/font_size": "36",
	})

	for _, path := range []string{"BG", "ValueOnly"} {
		style := monitorSceneResource(t, parsed, parsed.nodes[path], "theme_override_styles/panel")
		texture := monitorSceneReference(t, monitorSceneProperty(style, "texture"), "ExtResource")
		if external := parsed.externals[texture]; !strings.Contains(external, `path="res://engine/textures/monitor/frame_name.png"`) {
			t.Errorf("%s texture = %q, want frame_name.png", path, external)
		}
	}

	appearanceStyles := map[string]map[string]string{
		"ScratchBG": {
			"bg_color":               "Color(0.898039, 0.941176, 1, 1)",
			"border_color":           "Color(0.764706, 0.8, 0.85098, 1)",
			"corner_radius_top_left": "9",
		},
		"ScratchBG/H/ValueMargin/C": {
			"bg_color":               "Color(1, 1, 1, 1)",
			"corner_radius_top_left": "9",
		},
		"ScratchValueOnly": {
			"bg_color":               "Color(0.898039, 0.941176, 1, 1)",
			"border_color":           "Color(0.764706, 0.8, 0.85098, 1)",
			"corner_radius_top_left": "9",
		},
		"ScratchValueOnly/C": {
			"content_margin_left":    "9.0",
			"bg_color":               "Color(1, 1, 1, 1)",
			"corner_radius_top_left": "9",
		},
	}
	for path, properties := range appearanceStyles {
		node := parsed.nodes[path]
		resourceID := monitorSceneReference(t, monitorSceneProperty(node, "theme_override_styles/panel"), "SubResource")
		assertMonitorSceneProperties(t, parsed.resources[resourceID], properties)
	}
}

func TestListMonitorRender(t *testing.T) {
	panel, bound := newBoundMonitor(t)
	sink := newMonitorRenderSpy()
	color := mathf.NewColorRGBAi(255, 102, 26, 255)
	size := mathf.NewVec2(100, 200)
	style := MonitorStyle{Appearance: MonitorAppearanceList, Label: "items", Color: color, Dimensions: size}
	panel.render(sink, style, MonitorValue{Items: nil})
	if sink.listCalls != 1 || sink.text[bound["ScratchList"]] != "items" || sink.color[bound["ScratchList"]] != color || sink.size[bound["ScratchList"]] != size {
		t.Fatal("initial empty list render missing")
	}
	for _, view := range panel.views {
		if sink.visible[view.root.GetId()] != (view.root.GetId() == bound["ScratchList"]) {
			t.Fatal("wrong active monitor view")
		}
	}
	items := []string{"one", "two"}
	panel.render(sink, style, MonitorValue{Items: items})
	panel.render(sink, style, MonitorValue{Items: items})
	if sink.listCalls != 2 {
		t.Fatal("unchanged list sent across the bridge")
	}
	items[0] = "changed"
	panel.render(sink, style, MonitorValue{Items: items})
	if sink.listCalls != 3 || !slices.Equal(sink.items[bound["ScratchList"]], items) {
		t.Fatal("in-place update missed")
	}
	panel.render(sink, style, MonitorValue{Items: nil})
	if sink.listCalls != 4 || len(sink.items[bound["ScratchList"]]) != 0 {
		t.Fatal("clearing the list missed")
	}
}

func TestMonitorSwitchesBetweenScalarAndList(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	sink := newMonitorRenderSpy()
	color := mathf.Color{}
	size := mathf.NewVec2(100, 200)
	style := MonitorStyle{Appearance: MonitorAppearanceList, Label: "items", Color: color, Dimensions: size}
	panel.render(sink, MonitorStyle{Appearance: MonitorAppearanceCompatible, Label: "score", Color: color}, MonitorValue{Text: "1"})
	panel.render(sink, style, MonitorValue{Items: nil})
	panel.render(sink, style, MonitorValue{Items: []string{}})
	if sink.listCalls != 1 || panel.active != MonitorAppearanceList {
		t.Fatal("empty list must activate once, independently of its nil representation")
	}
	panel.render(sink, MonitorStyle{Appearance: MonitorAppearanceDefaultLarge, Label: "score", Color: color}, MonitorValue{Text: "2"})
	panel.render(sink, style, MonitorValue{Items: nil})
	if sink.listCalls != 2 || panel.active != MonitorAppearanceList {
		t.Fatal("reactivating a list must restore its payload")
	}
}

func TestSliderMonitorBidirectionalSync(t *testing.T) {
	for _, tt := range []struct {
		name           string
		config         MonitorSlider
		text           string
		position, want float64
	}{
		{"integer", MonitorSlider{0, 100, 1}, "25", 37, 37},
		{"decimal", MonitorSlider{-1, 1, 0.01}, "0.25", 0.3, 0.3},
		{"fractional minimum", MonitorSlider{0.5, 10.5, 1}, "2.5", 3.5, 3.5},
		{"fractional decimal minimum", MonitorSlider{0.005, 1, 0.01}, "0.205", 0.305, 0.305},
		{"nonaligned maximum", MonitorSlider{0, 10.5, 1}, "10.5", 9, 9},
		{"out of range", MonitorSlider{0, 100, 1}, "200", 99, 99},
		{"text", MonitorSlider{0, 100, 1}, "hello", 25, 25},
	} {
		t.Run(tt.name, func(t *testing.T) {
			panel, bound := newBoundMonitor(t)
			sink := newMonitorRenderSpy()
			style := MonitorStyle{Appearance: MonitorAppearanceSlider, Label: "score", Slider: tt.config}
			panel.render(sink, style, MonitorValue{Text: tt.text})
			if sink.text[bound["ScratchSlider/V/H/ValueMargin/C/LabelValue"]] != tt.text {
				t.Fatal("displayed variable was clamped")
			}
			if _, changed := panel.readSliderChange(sink); changed {
				t.Fatal("render produced user input")
			}
			sink.ranges[bound["ScratchSlider/V/Slider"]] = tt.position
			got, changed := panel.readSliderChange(sink)
			if !changed || math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("input = %v, %v; want %v", got, changed, tt.want)
			}
			if _, changed = panel.readSliderChange(sink); changed {
				t.Fatal("input replayed")
			}
			panel.render(sink, style, MonitorValue{Text: "-100"})
			if _, changed := panel.readSliderChange(sink); changed {
				t.Fatal("program update produced user input")
			}
		})
	}
}

func TestSliderMonitorEqualBounds(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	sink := newMonitorRenderSpy()
	panel.render(sink, MonitorStyle{Appearance: MonitorAppearanceSlider, Slider: MonitorSlider{5, 5, 1}}, MonitorValue{Text: "20"})
	if _, changed := panel.readSliderChange(sink); changed {
		t.Fatal("fixed range changed variable")
	}
}

func TestMonitorSwitchDiscardsInactiveSliderInput(t *testing.T) {
	panel, bound := newBoundMonitor(t)
	sink := newMonitorRenderSpy()
	if _, changed := panel.readSliderChange(sink); changed {
		t.Fatal("input before first render")
	}
	style := MonitorStyle{Appearance: MonitorAppearanceSlider, Slider: MonitorSlider{0, 100, 1}}
	panel.render(sink, style, MonitorValue{Text: "25"})
	sink.ranges[bound["ScratchSlider/V/Slider"]] = 37
	panel.render(sink, MonitorStyle{Appearance: MonitorAppearanceList}, MonitorValue{Items: []string{"one"}})
	if _, changed := panel.readSliderChange(sink); changed {
		t.Fatal("inactive slider emitted input")
	}
	panel.render(sink, style, MonitorValue{Text: "42"})
	if _, changed := panel.readSliderChange(sink); changed {
		t.Fatal("reactivation replayed stale input")
	}
	if got := sink.ranges[bound["ScratchSlider/V/Slider"]]; got != 42 {
		t.Fatalf("restored slider value = %v", got)
	}
}

func TestDecorativeUIScenesIgnoreMouse(t *testing.T) {
	for _, name := range []string{"UiDebug", "UiMeasure", "UiQuote", "UiSay"} {
		t.Run(name, func(t *testing.T) {
			parsed := parseMonitorScene(t, readUITemplateScene(t, name))
			for path, node := range parsed.nodes {
				if got := monitorSceneProperty(node, "mouse_filter"); got != "2" {
					t.Errorf("%s mouse_filter = %q, want 2 (ignore)", path, got)
				}
			}
		})
	}
}

func TestDisplayMonitorsStopMouse(t *testing.T) {
	parsed := parseMonitorScene(t, readUITemplateScene(t, "UiMonitor"))
	if got := monitorSceneProperty(parsed.nodes["."], "mouse_filter"); got != "2" {
		t.Errorf("monitor root mouse_filter = %q, want 2 (ignore)", got)
	}
	for _, root := range []string{"BG", "ValueOnly", "ScratchBG", "ScratchValueOnly"} {
		if got := monitorSceneProperty(parsed.nodes[root], "mouse_filter"); got != "0" {
			t.Errorf("%s mouse_filter = %q, want 0 (stop)", root, got)
		}
		for path, node := range parsed.nodes {
			if !strings.HasPrefix(path, root+"/") {
				continue
			}
			if got := monitorSceneProperty(node, "mouse_filter"); got != "2" {
				t.Errorf("%s mouse_filter = %q, want 2 (ignore)", path, got)
			}
		}
	}
	if got := monitorSceneProperty(parsed.nodes["ScratchSlider/V/Slider"], "mouse_filter"); got != "0" {
		t.Errorf("slider mouse_filter = %q, want 0 (stop)", got)
	}
}

func (p *monitorRenderSpy) SetVisible(id engine.Object, visible bool) { p.visible[id] = visible }

func (p *monitorRenderSpy) SetText(id engine.Object, text string) { p.text[id] = text }

func (p *monitorRenderSpy) SetTextMaxWidth(id engine.Object, text string, maxWidth float64) float64 {
	p.text[id] = text
	p.maxWidths[id] = append(p.maxWidths[id], maxWidth)
	natural := float64(len([]rune(text)))
	if widths := p.naturalWidth[id]; widths != nil {
		if width, ok := widths[text]; ok {
			natural = width
		}
	}
	applied := math.Min(natural, maxWidth)
	p.appliedWidth[id] = applied
	return applied
}

func (p *monitorRenderSpy) SetColor(id engine.Object, color mathf.Color) {
	p.color[id] = color
}

func (p *monitorRenderSpy) GetSize(id engine.Object) mathf.Vec2 { return p.size[id] }

func (p *monitorRenderSpy) SetSize(id engine.Object, size mathf.Vec2) {
	p.size[id] = size
	if size != (mathf.Vec2{}) {
		return
	}
	layout, ok := p.layouts[id]
	if !ok {
		return
	}
	width := layout.horizontalOverhead + p.appliedWidth[layout.label] + p.appliedWidth[layout.value]
	if width < layout.minimum {
		width = layout.minimum
	}
	p.size[id] = mathf.NewVec2(width, 0)
}

func (p *monitorRenderSpy) SetListItems(id engine.Object, name string, items engine.Array, color mathf.Color) {
	p.listCalls++
	p.items[id] = slices.Clone(items.([]string))
	p.text[id] = name
	p.color[id] = color
}

func (p *monitorRenderSpy) SetRange(id engine.Object, min, max, step, value float64) {
	p.ranges[id] = math.Max(min, math.Min(max, math.Floor((value-min)/step+0.5)*step+min))
}

func (p *monitorRenderSpy) GetRangeValue(id engine.Object) float64 { return p.ranges[id] }

func (p *monitorRenderSpy) reset() {
	clear(p.visible)
	clear(p.text)
	clear(p.color)
	clear(p.items)
	clear(p.size)
	clear(p.maxWidths)
	clear(p.appliedWidth)
	p.listCalls = 0
}

func (m *monitorEngineUiSpy) GetCanvasScale(gdx.Object) mathf.Vec2 { return m.canvasScale }

func (m *monitorEngineUiSpy) SetVisible(id gdx.Object, visible bool) {
	m.visible[id] = visible
}

func (m *monitorEngineUiSpy) SetText(id gdx.Object, text string) { m.texts[id] = text }

func (m *monitorEngineUiSpy) SetTextMaxWidth(id gdx.Object, text string, maxWidth float64) float64 {
	m.texts[id] = text
	m.maxWidths[id] = maxWidth
	return maxWidth
}

func (m *monitorEngineUiSpy) SetColor(id gdx.Object, color mathf.Color) { m.colors[id] = color }

func (m *monitorEngineUiSpy) SetSize(id gdx.Object, size mathf.Vec2) { m.sizes[id] = size }

func (m *monitorEngineUiSpy) SetScale(id gdx.Object, scale mathf.Vec2) { m.scales[id] = scale }

func (m *monitorEngineUiSpy) SetGlobalPosition(id gdx.Object, position mathf.Vec2) {
	m.positions[id] = position
}

func newMonitorRenderSpy() *monitorRenderSpy {
	return &monitorRenderSpy{
		visible:      make(map[engine.Object]bool),
		ranges:       make(map[engine.Object]float64),
		text:         make(map[engine.Object]string),
		color:        make(map[engine.Object]mathf.Color),
		items:        make(map[engine.Object][]string),
		size:         make(map[engine.Object]mathf.Vec2),
		maxWidths:    make(map[engine.Object][]float64),
		naturalWidth: make(map[engine.Object]map[string]float64),
		appliedWidth: make(map[engine.Object]float64),
		layouts:      make(map[engine.Object]monitorRenderSpyLayout),
	}
}

func newMonitorEngineUiSpy() *monitorEngineUiSpy {
	return &monitorEngineUiSpy{
		canvasScale: mathf.NewVec2(0.5, 0.5),
		visible:     make(map[gdx.Object]bool),
		texts:       make(map[gdx.Object]string),
		maxWidths:   make(map[gdx.Object]float64),
		colors:      make(map[gdx.Object]mathf.Color),
		sizes:       make(map[gdx.Object]mathf.Vec2),
		scales:      make(map[gdx.Object]mathf.Vec2),
		positions:   make(map[gdx.Object]mathf.Vec2),
	}
}

func setupMonitorEngine(t *testing.T) *monitorEngineUiSpy {
	t.Helper()
	oldUI, oldCamera := gdx.UiMgr, gdx.CameraMgr
	oldScale := engine.WindowScale()
	enginewrap.Init(func(call func()) { call() })
	mgr := newMonitorEngineUiSpy()
	gdx.UiMgr, gdx.CameraMgr = mgr, uiRenderCamera{}
	engine.SetWindowScale(1)
	t.Cleanup(func() {
		gdx.UiMgr, gdx.CameraMgr = oldUI, oldCamera
		engine.SetWindowScale(oldScale)
		enginewrap.Init(engine.WaitMainThread)
	})
	return mgr
}

func newBoundMonitor(t *testing.T) (*UiMonitor, map[string]engine.Object) {
	t.Helper()
	panel := &UiMonitor{}
	bound := make(map[string]engine.Object)
	nextID := engine.Object(1)
	panel.bindViews(func(path string) *UiNode {
		if _, exists := bound[path]; exists {
			t.Fatalf("duplicate monitor binding %q", path)
		}
		bound[path] = nextID
		node := &UiNode{}
		node.SetId(nextID)
		nextID++
		return node
	})
	return panel, bound
}

func assertMonitorPayload(t *testing.T, spy *monitorRenderSpy, view monitorView, name, value string, color mathf.Color) {
	t.Helper()
	if spy.listCalls != 0 || len(spy.size) != 0 {
		t.Fatal("scalar monitor used list-only operations")
	}
	wantTextCount := 1
	if view.label != nil {
		wantTextCount++
		if got := spy.text[view.label.GetId()]; got != name {
			t.Errorf("label text = %q, want %q", got, name)
		}
	}
	if got := spy.text[view.value.GetId()]; got != value {
		t.Errorf("value text = %q, want %q", got, value)
	}
	if len(spy.text) != wantTextCount {
		t.Errorf("text updates = %v, want only active view", spy.text)
	}

	if view.colorTarget == nil {
		if len(spy.color) != 0 {
			t.Errorf("default appearance changed color: %v", spy.color)
		}
		return
	}
	if got := spy.color[view.colorTarget.GetId()]; got != color || len(spy.color) != 1 {
		t.Errorf("color updates = %v, want {%d: %v}", spy.color, view.colorTarget.GetId(), color)
	}
}

func parseMonitorScene(t *testing.T, source string) parsedMonitorScene {
	t.Helper()
	parsed := parsedMonitorScene{
		nodes:     make(map[string]string),
		nodeTypes: make(map[string]string),
		resources: make(map[string]string),
		externals: make(map[string]string),
	}
	var section strings.Builder
	flush := func() {
		text := section.String()
		header, _, _ := strings.Cut(text, "\n")
		switch {
		case monitorNodeHeader.MatchString(header):
			match := monitorNodeHeader.FindStringSubmatch(header)
			path := "."
			if match[3] == "." {
				path = match[1]
			} else if match[3] != "" {
				path = match[3] + "/" + match[1]
			}
			parsed.nodes[path] = text
			parsed.nodeTypes[path] = match[2]
		case monitorResourceHeader.MatchString(header):
			parsed.resources[monitorResourceHeader.FindStringSubmatch(header)[1]] = text
		case monitorExternalHeader.MatchString(header):
			parsed.externals[monitorExternalHeader.FindStringSubmatch(header)[1]] = text
		}
		section.Reset()
	}

	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "[") && section.Len() > 0 {
			flush()
		}
		section.WriteString(line)
		section.WriteByte('\n')
	}
	flush()
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan UiMonitor scene: %v", err)
	}
	return parsed
}

func monitorSceneProperty(section, name string) string {
	prefix := name + " = "
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func monitorSceneResource(t *testing.T, scene parsedMonitorScene, node, property string) string {
	t.Helper()
	id := monitorSceneReference(t, monitorSceneProperty(node, property), "SubResource")
	resource, ok := scene.resources[id]
	if !ok {
		t.Fatalf("missing sub-resource %q", id)
	}
	return resource
}

func monitorSceneReference(t *testing.T, value, kind string) string {
	t.Helper()
	prefix, suffix := kind+`("`, `")`
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, suffix) {
		t.Fatalf("invalid %s reference %q", kind, value)
	}
	return strings.TrimSuffix(strings.TrimPrefix(value, prefix), suffix)
}

func assertMonitorSceneProperties(t *testing.T, section string, want map[string]string) {
	t.Helper()
	if section == "" {
		t.Fatal("missing UiMonitor scene section")
	}
	for property, value := range want {
		if got := monitorSceneProperty(section, property); got != value {
			t.Errorf("%s = %q, want %q", property, got, value)
		}
	}
}

func readUITemplateScene(t *testing.T, name string) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not resolve test source path")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "..", "..", "cmd", "spx", "template", "project", "engine", "ui", name+".tscn")
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s scene: %v", name, err)
	}
	return string(data)
}
