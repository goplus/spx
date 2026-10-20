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
	"github.com/goplus/spx/v3/internal/engine"
)

type monitorRenderSpy struct {
	visible   map[engine.Object]bool
	text      map[engine.Object]string
	color     map[engine.Object]mathf.Color
	items     map[engine.Object][]string
	size      map[engine.Object]mathf.Vec2
	listCalls int
	ranges    map[engine.Object]float64
}

func newMonitorRenderSpy() *monitorRenderSpy {
	return &monitorRenderSpy{
		visible: make(map[engine.Object]bool),
		ranges:  make(map[engine.Object]float64),
		text:    make(map[engine.Object]string),
		color:   make(map[engine.Object]mathf.Color),
		items:   make(map[engine.Object][]string),
		size:    make(map[engine.Object]mathf.Vec2),
	}
}

func (p *monitorRenderSpy) SetVisible(id engine.Object, visible bool) { p.visible[id] = visible }
func (p *monitorRenderSpy) SetText(id engine.Object, text string)     { p.text[id] = text }
func (p *monitorRenderSpy) SetColor(id engine.Object, color mathf.Color) {
	p.color[id] = color
}

func (p *monitorRenderSpy) SetSize(id engine.Object, size mathf.Vec2) { p.size[id] = size }
func (p *monitorRenderSpy) SetListItems(id engine.Object, name string, items engine.Array, color mathf.Color) {
	p.listCalls++
	p.items[id] = slices.Clone(items.([]string))
	p.text[id] = name
	p.color[id] = color
}

func (p *monitorRenderSpy) reset() {
	clear(p.visible)
	clear(p.text)
	clear(p.color)
	clear(p.items)
	clear(p.size)
	p.listCalls = 0
}

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

func (p *monitorRenderSpy) SetRange(id engine.Object, min, max, step, value float64) {
	p.ranges[id] = math.Max(min, math.Min(max, math.Floor((value-min)/step+0.5)*step+min))
}
func (p *monitorRenderSpy) GetRangeValue(id engine.Object) float64 { return p.ranges[id] }

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
