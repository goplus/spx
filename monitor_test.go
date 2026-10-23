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
	"syscall"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/ui"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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
	g := monitorEvalFixture{score: 1.23456789, unset: nil}
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
	visible bool
	events  []string
	style   ui.MonitorStyle
	value   ui.MonitorValue
	scale   float64
	pos     mathf.Vec2
}

func (p *monitorPanelSpy) Render(style ui.MonitorStyle, value ui.MonitorValue) {
	p.events = append(p.events, "render")
	p.style, p.value = style, value
}

func (p *monitorPanelSpy) SetVisible(visible bool) {
	p.visible = visible
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
	for name, appearance := range map[string]ui.MonitorAppearance{"scalar": ui.MonitorAppearanceCompatible, "list": ui.MonitorAppearanceList} {
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
			visible := []string{"eval", "scale", "position", "render", "show"}
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
	if !slices.Equal(panel.events, []string{"scale", "position", "render", "show"}) || m.visible || m.isDirty {
		t.Fatal("getter changed the in-progress refresh's visibility")
	}
	panel.events = nil
	m.onUpdate(monitorUpdateIntervalS)
	if !slices.Equal(panel.events, []string{"hide"}) {
		t.Fatal("getter visibility did not take effect on the next refresh")
	}
}

func TestMonitorAppearance(t *testing.T) {
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
		{name: "scratch", mode: 1, style: "scratch", want: ui.MonitorAppearanceCompatible},
		{name: "scratch large", mode: 2, style: "scratch", want: ui.MonitorAppearanceCompatibleLarge},
		{name: "scratch unsupported mode", mode: 99, style: "scratch", want: ui.MonitorAppearanceCompatible},
		{name: "unknown style", mode: 1, style: "custom", want: ui.MonitorAppearanceDefault},
		{name: "invalid style", mode: 2, style: 1.0, want: ui.MonitorAppearanceDefaultLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shape := coreproject.StageShape{"mode": float64(test.mode)}
			if test.style != nil {
				shape["style"] = test.style
			}
			if got := monitorAppearance(testMonitorShape(t, shape)); got != test.want {
				t.Fatalf("monitorAppearance(%v) = %v, want %v", shape, got, test.want)
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
		"scratch-default":  {ui.MonitorAppearanceCompatible, "Monkey", "getVar:clicked", -235, 140},
		"scratch-large":    {ui.MonitorAppearanceCompatibleLarge, "", "getVar:downs", -115, 140},
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
			appearance: monitorAppearance(testMonitorShape(t, shape)),
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

func TestTutorialMonitorFixtures(t *testing.T) {
	fixtures := []struct {
		demo   string
		config coreproject.MonitorShape
	}{
		{"02-Dragon", coreproject.MonitorShape{
			Target: "Dragon", Val: "getVar:score", Name: "monitor-1", Label: "score",
			Mode: 1, Style: "scratch", Size: 1, X: -240, Y: 180, Visible: true,
		}},
		{"03-Clone", coreproject.MonitorShape{
			Target: "", Val: "getVar:gid", Name: "monitor-1", Label: "gid",
			Mode: 1, Style: "scratch", Size: 1, X: -240, Y: 180, Visible: true,
		}},
	}
	for _, fixture := range fixtures {
		for _, filename := range []string{"index.json", "index_pack.json"} {
			t.Run(fixture.demo+"/"+filename, func(t *testing.T) {
				data, err := os.ReadFile("tutorial/" + fixture.demo + "/assets/" + filename)
				if err != nil {
					t.Fatal(err)
				}
				var project struct {
					ZOrder []any `json:"zorder"`
				}
				if err := json.Unmarshal(data, &project); err != nil {
					t.Fatal(err)
				}
				monitors := 0
				for _, entry := range project.ZOrder {
					shape, ok := entry.(map[string]any)
					if !ok || shape["type"] != "monitor" {
						continue
					}
					monitors++
					got, err := prepareMonitor(shape)
					if err != nil {
						t.Fatal(err)
					}
					if got.config != fixture.config {
						t.Errorf("monitor config = %+v, want %+v", got.config, fixture.config)
					}
					if got.style.Appearance != ui.MonitorAppearanceCompatible {
						t.Errorf("monitor appearance = %v, want Scratch", got.style.Appearance)
					}
					if got.style.Label != fixture.config.Label || got.style.Color != mathf.NewColorRGBAi(0xee, 0x7d, 0x16, 0xff) {
						t.Errorf("monitor label or color changed: %+v", got.style)
					}
				}
				if monitors != 1 {
					t.Errorf("monitor count = %d, want 1", monitors)
				}
			})
		}
	}
}

func monitorEvalForTest(g reflect.Value, target, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
	binding, _ := bindMonitor(g, target, val, appearance)
	return binding.read
}

func testMonitorShape(t *testing.T, fields coreproject.StageShape) coreproject.MonitorShape {
	t.Helper()
	shape := coreproject.StageShape{
		"target": "", "val": "score", "name": "score", "label": "score",
		"mode": 1.0, "x": 0.0, "y": 0.0, "visible": true,
	}
	for key, value := range fields {
		shape[key] = value
	}
	parsed, err := coreproject.ParseMonitorShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestMonitorSliderConfig(t *testing.T) {
	for _, mode := range []any{"slider", float64(3)} {
		if got := monitorAppearance(testMonitorShape(t, coreproject.StageShape{"mode": mode})); got != ui.MonitorAppearanceSlider {
			t.Fatalf("mode %v: %v", mode, got)
		}
	}
	for _, tt := range []struct {
		config coreproject.StageShape
		want   ui.MonitorSlider
	}{
		{coreproject.StageShape{}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": 10.0, "sliderMax": -10.0, "isDiscrete": false}, ui.MonitorSlider{Min: -10, Max: 10, Step: 0.01}},
		{coreproject.StageShape{"sliderMin": math.NaN(), "sliderMax": math.Inf(1)}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": 5.0, "sliderMax": 5.0}, ui.MonitorSlider{Min: 5, Max: 5, Step: 1}},
		{coreproject.StageShape{"sliderMin": -math.MaxFloat64, "sliderMax": math.MaxFloat64}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMax": math.MaxFloat64, "isDiscrete": false}, ui.MonitorSlider{Min: 0, Max: 100, Step: 0.01}},
	} {
		if got := parseMonitorSlider(tt.config); got != tt.want {
			t.Errorf("got %+v, want %+v", got, tt.want)
		}
	}
}

func TestMonitorColorDefaultsAndOverride(t *testing.T) {
	for _, test := range []struct {
		appearance ui.MonitorAppearance
		want       mathf.Color
	}{
		{ui.MonitorAppearanceDefault, mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)},
		{ui.MonitorAppearanceCompatible, mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)},
		{ui.MonitorAppearanceList, mathf.NewColorRGBAi(0xff, 0x66, 0x1a, 0xff)},
	} {
		for _, invalid := range []any{nil, "invalid color"} {
			if got := parseMonitorColor(coreproject.StageShape{"color": invalid}, test.appearance); got != test.want {
				t.Errorf("appearance %v: color = %v, want %v", test.appearance, got, test.want)
			}
		}
		want := mathf.NewColorRGBAi(0x12, 0x34, 0x56, 0xff)
		if got := parseMonitorColor(coreproject.StageShape{"color": "#123456"}, test.appearance); got != want {
			t.Errorf("appearance %v: explicit color = %v, want %v", test.appearance, got, want)
		}
	}
}

func TestMonitorSliderWritesBeforeRefresh(t *testing.T) {
	game := monitorEvalFixture{score: 25}
	root := reflect.ValueOf(&game).Elem()
	binding, err := bindMonitor(root, "", "score", ui.MonitorAppearanceSlider)
	if err != nil {
		t.Fatal(err)
	}
	panel := &monitorPanelSpy{}
	pending := false
	monitor := Monitor{
		visible: true, panel: panel,
		eval: binding.read,
		updateInput: func() bool {
			if !pending {
				return false
			}
			pending = false
			return binding.write(37)
		},
	}
	pending = true
	monitor.onUpdate(0.001)
	if game.score != 37 || panel.value.Text != "37" {
		t.Fatal("input did not update variable and label in the same frame")
	}
	monitor.visible = false
	pending = true
	monitor.onUpdate(0.001)
	if !pending {
		t.Fatal("hidden slider consumed input")
	}
}

type SliderMonitorGame struct {
	Game
	score float64
	Dot   sliderMonitorSprite
}
type sliderMonitorSprite struct {
	SpriteImpl
	*SliderMonitorGame
	score float64
}

func TestMonitorSliderTargetScope(t *testing.T) {
	game := SliderMonitorGame{score: 1}
	game.Dot.SliderMonitorGame = &game
	game.Dot.score = 2
	root := reflect.ValueOf(&game).Elem()
	binding, err := bindMonitor(root, "Dot", "getVar:score", ui.MonitorAppearanceSlider)
	if err != nil || !binding.write(7.5) || game.Dot.score != 7.5 || game.score != 1 {
		t.Fatal("sprite-local slider wrote the wrong scope")
	}
	eval := binding.read
	if eval == nil || eval().Text != "7.5" {
		t.Fatal("slider reader and writer resolve different variables")
	}
	binding, err = bindMonitor(root, "", "score", ui.MonitorAppearanceSlider)
	if err != nil || !binding.write(9) || game.score != 9 || game.Dot.score != 7.5 {
		t.Fatal("stage slider wrote the wrong scope")
	}
}

func TestParseMonitorModes(t *testing.T) {
	shape := coreproject.StageShape{"target": "", "val": "score", "name": "score", "label": "score", "x": 0.0, "y": 0.0, "visible": true}
	for _, mode := range []any{1.0, 2.0, 3.0, 4.0, "slider", "list"} {
		shape["mode"] = mode
		if _, err := coreproject.ParseMonitorShape(shape); err != nil {
			t.Errorf("mode %v: %v", mode, err)
		}
	}
	for _, mode := range []any{nil, true, "unknown"} {
		shape["mode"] = mode
		if _, err := coreproject.ParseMonitorShape(shape); err == nil {
			t.Errorf("accepted invalid mode %v", mode)
		}
	}
}

func TestMonitorBindingCapabilities(t *testing.T) {
	game := monitorEvalFixture{score: 1}
	root := reflect.ValueOf(&game).Elem()
	for _, appearance := range []ui.MonitorAppearance{ui.MonitorAppearanceDefault, ui.MonitorAppearanceCompatibleLarge} {
		binding, err := bindMonitor(root, "", "score", appearance)
		if err != nil || binding.read == nil || binding.write != nil {
			t.Fatal("display-only monitor gained a writer")
		}
	}
	for _, tt := range []struct {
		target, name string
		want         error
	}{
		{"", "value", syscall.EINVAL},
		{"", "Value", syscall.EINVAL},
		{"", "missing", syscall.ENOENT},
		{"missing", "score", syscall.ENOENT},
		{"", "getVar:", syscall.ENOENT},
	} {
		binding, err := bindMonitor(root, tt.target, tt.name, ui.MonitorAppearanceSlider)
		if err != tt.want || binding.read != nil || binding.write != nil {
			t.Errorf("binding %q/%q: error %v, want %v without partial binding", tt.target, tt.name, err, tt.want)
		}
	}
	if game.calls != 0 {
		t.Fatal("binding invoked a reporter")
	}
}

type gameWidgetUiMgr struct {
	*askTestUiMgr
	rangeValue float64
}

func (*gameWidgetUiMgr) SetScale(pkgengine.Object, mathf.Vec2)                 {}
func (*gameWidgetUiMgr) GetCanvasScale(pkgengine.Object) mathf.Vec2            { return mathf.NewVec2(1, 1) }
func (*gameWidgetUiMgr) SetGlobalPosition(pkgengine.Object, mathf.Vec2)        {}
func (*gameWidgetUiMgr) SetColor(pkgengine.Object, mathf.Color)                {}
func (*gameWidgetUiMgr) SetSize(pkgengine.Object, mathf.Vec2)                  {}
func (*gameWidgetUiMgr) SetRotation(pkgengine.Object, float64)                 {}
func (m *gameWidgetUiMgr) SetRange(_ pkgengine.Object, _, _, _, value float64) { m.rangeValue = value }
func (m *gameWidgetUiMgr) GetRangeValue(pkgengine.Object) float64              { return m.rangeValue }
func setupGameWidgetUi(t *testing.T) *gameWidgetUiMgr {
	t.Helper()
	base := installAskTestUiMgr(t)
	previousCamera := pkgengine.CameraMgr
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	t.Cleanup(func() { pkgengine.CameraMgr = previousCamera })
	manager := &gameWidgetUiMgr{askTestUiMgr: base}
	pkgengine.UiMgr = manager
	return manager
}

func TestMonitorConstructionAndWidgetTransforms(t *testing.T) {
	manager := setupGameWidgetUi(t)
	game := &monitorEvalFixture{score: 5}
	prepared, err := prepareMonitor(coreproject.StageShape{"name": "score", "val": "score", "label": "Score", "target": "", "mode": "slider", "x": 1.0, "y": 2.0, "visible": true})
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := newMonitor(reflect.ValueOf(game).Elem(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if monitor.GetName() != "score" || monitor.updateInput == nil {
		t.Fatalf("monitor=%+v", monitor)
	}
	monitor.SetXpos(10)
	monitor.SetYpos(20)
	monitor.ChangeYpos(-3)
	monitor.ChangeXYpos(4, 5)
	monitor.SetSize(2)
	monitor.ChangeSize(0.5)
	if monitor.Xpos() != 14 || monitor.Ypos() != 22 || monitor.Size() != 2.5 || !monitor.isDirty {
		t.Fatalf("widget transform=%+v", monitor)
	}
	monitor.onUpdate(0)
	if monitor.updateInput() {
		t.Fatal("unchanged slider reports input")
	}
	manager.rangeValue = 42
	if !monitor.updateInput() || game.score != 42 {
		t.Fatalf("slider input=%g", game.score)
	}
	prepared.style.Appearance = ui.MonitorAppearanceDefault
	plain, err := newMonitor(reflect.ValueOf(game).Elem(), prepared)
	if err != nil || plain.updateInput != nil {
		t.Fatalf("plain monitor=%+v, err=%v", plain, err)
	}
}
