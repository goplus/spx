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
	"encoding/json"
	"math"
	"os"
	"reflect"
	"regexp"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/ui"
)

type monitorEvalFixture struct {
	Game
	score float64
	unset any
	calls int
}

func (g *monitorEvalFixture) Value() float64 {
	g.calls++
	return g.score
}

func TestMonitorEvalPreservesFormatting(t *testing.T) {
	g := monitorEvalFixture{score: 1.23456789}
	root := reflect.ValueOf(&g).Elem()
	for _, name := range []string{"score", "getVar:score"} {
		eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceDefault)
		if eval == nil || eval().Text != "1.23456789" {
			t.Fatalf("field binding %q lost precision", name)
		}
		g.score = 2.3456789
		if eval().Text != "2.3456789" {
			t.Fatal("field binding stopped tracking changes")
		}
		g.score = 1.23456789
	}
	for _, name := range []string{"value", "getVar:value"} {
		eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceDefault)
		if eval == nil || g.calls != 0 {
			t.Fatal("binding must not invoke a getter")
		}
		if eval().Text != "1.23" || g.calls != 1 {
			t.Fatal("auto-property formatting or invocation changed")
		}
		g.calls = 0
	}
	// An explicitly named method is displayed as its function pointer by the
	// existing string evaluator; only the lowercase auto-property is invoked.
	eval := monitorEvalForTest(root, "", "Value", ui.MonitorAppearanceDefault)
	if eval == nil || !regexp.MustCompile(`^0x[0-9a-f]+$`).MatchString(eval().Text) || g.calls != 0 {
		t.Fatal("explicit method behavior changed")
	}
	if eval := monitorEvalForTest(root, "", "unset", ui.MonitorAppearanceDefault); eval == nil || eval().Text != "<nil>" {
		t.Fatal("nil field formatting changed")
	}
	for _, binding := range [][2]string{{"", ""}, {"", "getVar:"}, {"", "missing"}, {"missing", "score"}} {
		if monitorEvalForTest(root, binding[0], binding[1], ui.MonitorAppearanceDefault) != nil {
			t.Errorf("unexpected binding for %v", binding)
		}
	}
}

type monitorPanelSpy struct {
	events []string
	style  ui.MonitorStyle
	value  ui.MonitorValue
	scale  float64
	pos    mathf.Vec2
}

func (p *monitorPanelSpy) Render(style ui.MonitorStyle, value ui.MonitorValue) {
	p.events = append(p.events, "render")
	p.style, p.value = style, value
}

func (p *monitorPanelSpy) SetVisible(visible bool) {
	if visible {
		p.events = append(p.events, "show")
	} else {
		p.events = append(p.events, "hide")
	}
}

func (p *monitorPanelSpy) UpdateScale(scale float64) {
	p.events = append(p.events, "scale")
	p.scale = scale
}

func (p *monitorPanelSpy) UpdatePos(pos mathf.Vec2) {
	p.events = append(p.events, "position")
	p.pos = pos
}

func TestMonitorRefreshLifecycle(t *testing.T) {
	for name, appearance := range map[string]ui.MonitorAppearance{"scalar": ui.MonitorAppearanceScratch, "list": ui.MonitorAppearanceList} {
		t.Run(name, func(t *testing.T) {
			panel := &monitorPanelSpy{}
			value := ui.MonitorValue{Text: "42", Items: []string{"one", "two"}}
			m := &Monitor{
				panel: panel, visible: true, isDirty: true, size: 1,
				style: ui.MonitorStyle{Appearance: appearance, Label: "value"},
				eval: func() ui.MonitorValue {
					panel.events = append(panel.events, "eval")
					return value
				},
			}
			refresh := func(delta float64, want ...string) {
				t.Helper()
				panel.events = nil
				m.onUpdate(delta)
				if !slices.Equal(panel.events, want) {
					t.Fatalf("refresh(%v) calls = %v, want %v", delta, panel.events, want)
				}
			}
			visible := []string{"eval", "render", "scale", "position", "show"}
			refresh(0, visible...)
			if panel.style != m.style || !reflect.DeepEqual(panel.value, value) {
				t.Fatal("style or value was lost during refresh")
			}
			refresh(0.1)
			m.SetXYpos(12, -34)
			m.SetSize(2)
			refresh(0, visible...)
			if panel.pos != mathf.NewVec2(12, -34) || panel.scale != 2 || m.updateTimer != 0.1 {
				t.Fatal("dirty refresh changed the timer or lost widget state")
			}
			refresh(0.1, visible...)
			if m.updateTimer != 0 {
				t.Fatal("periodic refresh did not reset the timer")
			}
			m.Hide()
			refresh(0, "hide")
			refresh(0.1)
			refresh(0.1, "hide")
			m.Show()
			refresh(0, visible...)
			m.Show()
			refresh(0)
			refresh(1, visible...)
			refresh(0)
			m.updateTimer = math.NaN()
			refresh(0)
			m.ChangeXpos(1)
			refresh(0, visible...)
		})
	}
}

func TestMonitorGetterVisibilityTakesEffectNextRefresh(t *testing.T) {
	panel := &monitorPanelSpy{}
	m := &Monitor{panel: panel, visible: true, isDirty: true}
	m.eval = func() ui.MonitorValue {
		m.Hide()
		return ui.MonitorValue{Text: "hidden"}
	}
	m.onUpdate(0)
	if !slices.Equal(panel.events, []string{"render", "scale", "position", "show"}) || m.visible || m.isDirty {
		t.Fatal("getter changed the in-progress refresh's visibility")
	}
	panel.events = nil
	m.onUpdate(monitorUpdateIntervalS)
	if !slices.Equal(panel.events, []string{"hide"}) {
		t.Fatal("getter visibility did not take effect on the next refresh")
	}
}

func TestParseMonitorAppearance(t *testing.T) {
	tests := []struct {
		name  string
		mode  int
		style any
		want  ui.MonitorAppearance
	}{
		{name: "default", mode: 1, want: ui.MonitorAppearanceDefault},
		{name: "default large", mode: 2, want: ui.MonitorAppearanceDefaultLarge},
		{name: "default unsupported mode", mode: 99, want: ui.MonitorAppearanceDefaultLarge},
		{name: "explicit default", mode: 1, style: "default", want: ui.MonitorAppearanceDefault},
		{name: "scratch", mode: 1, style: "scratch", want: ui.MonitorAppearanceScratch},
		{name: "scratch large", mode: 2, style: "scratch", want: ui.MonitorAppearanceScratchLarge},
		{name: "scratch unsupported mode", mode: 99, style: "scratch", want: ui.MonitorAppearanceScratch},
		{name: "unknown style", mode: 1, style: "custom", want: ui.MonitorAppearanceDefault},
		{name: "invalid style", mode: 2, style: 1.0, want: ui.MonitorAppearanceDefaultLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shape := coreproject.StageShape{"mode": float64(test.mode)}
			if test.style != nil {
				shape["style"] = test.style
			}
			if got := parseMonitorAppearance(shape); got != test.want {
				t.Fatalf("parseMonitorAppearance(%v) = %v, want %v", shape, got, test.want)
			}
		})
	}
}

func TestStageMonitorFixture(t *testing.T) {
	data, err := os.ReadFile("test/StageMonitor/assets/index.json")
	if err != nil {
		t.Fatalf("read StageMonitor fixture: %v", err)
	}
	var project struct {
		ZOrder []coreproject.StageShape `json:"zorder"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("decode StageMonitor fixture: %v", err)
	}

	type monitorFixture struct {
		appearance ui.MonitorAppearance
		target     string
		value      string
		x, y       float64
	}
	want := map[string]monitorFixture{
		"default-implicit": {ui.MonitorAppearanceDefault, "Monkey", "getVar:clicked", -235, 175},
		"default-large":    {ui.MonitorAppearanceDefaultLarge, "", "getVar:downs", -115, 175},
		"scratch-default":  {ui.MonitorAppearanceScratch, "Monkey", "getVar:clicked", -235, 140},
		"scratch-large":    {ui.MonitorAppearanceScratchLarge, "", "getVar:downs", -115, 140},
	}
	for _, shape := range project.ZOrder {
		if shape["type"] != "monitor" {
			continue
		}
		name, _ := shape["name"].(string)
		expected, ok := want[name]
		if !ok {
			t.Errorf("unexpected monitor %q", name)
			continue
		}
		got := monitorFixture{
			appearance: parseMonitorAppearance(shape),
			target:     shape["target"].(string),
			value:      shape["val"].(string),
			x:          shape["x"].(float64),
			y:          shape["y"].(float64),
		}
		if got != expected {
			t.Errorf("monitor %q = %+v, want %+v", name, got, expected)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("missing monitor %q", name)
	}
}

func monitorEvalForTest(g reflect.Value, target, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
	binding, _ := bindMonitor(g, target, val, appearance)
	return binding.read
}
