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
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
)

type monitorRenderSpy struct {
	visible map[engine.Object]bool
	text    map[engine.Object]string
	color   map[engine.Object]mathf.Color
}

func newMonitorRenderSpy() *monitorRenderSpy {
	return &monitorRenderSpy{
		visible: make(map[engine.Object]bool),
		text:    make(map[engine.Object]string),
		color:   make(map[engine.Object]mathf.Color),
	}
}

func (p *monitorRenderSpy) SetVisible(id engine.Object, visible bool) { p.visible[id] = visible }
func (p *monitorRenderSpy) SetText(id engine.Object, text string)     { p.text[id] = text }
func (p *monitorRenderSpy) SetColor(id engine.Object, color mathf.Color) {
	p.color[id] = color
}

func (p *monitorRenderSpy) reset() {
	clear(p.visible)
	clear(p.text)
	clear(p.color)
}

func TestUiMonitorRender(t *testing.T) {
	color := mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)
	for appearance := MonitorAppearanceDefault; appearance < monitorAppearanceCount; appearance++ {
		t.Run(monitorViewSpecs[appearance].root, func(t *testing.T) {
			panel, _ := newBoundMonitor(t)
			spy := newMonitorRenderSpy()

			panel.render(spy, appearance, "score", "42", color)
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
			panel.render(spy, appearance, "points", "43", color)
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
	panel.render(spy, MonitorAppearance(255), "score", "0", mathf.Color{})

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
	scene := readMonitorScene(t)
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
	for _, spec := range monitorViewSpecs {
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
	for _, path := range []string{"ValueOnly", "ScratchBG", "ScratchValueOnly"} {
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

	scratchStyles := map[string]map[string]string{
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
	for path, properties := range scratchStyles {
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

func readMonitorScene(t *testing.T) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not resolve test source path")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "..", "..", "cmd", "spx", "template", "project", "engine", "ui", "UiMonitor.tscn")
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read UiMonitor scene: %v", err)
	}
	return string(data)
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
