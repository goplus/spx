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
	"syscall"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/ui"
)

func TestMonitorSliderConfig(t *testing.T) {
	for _, mode := range []any{"slider", float64(3)} {
		if got := parseMonitorAppearance(coreproject.StageShape{"mode": mode}); got != ui.MonitorAppearanceSlider {
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
	} {
		if got := parseMonitorSlider(tt.config); got != tt.want {
			t.Errorf("got %+v, want %+v", got, tt.want)
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

func TestReloadMonitorModes(t *testing.T) {
	shape := coreproject.StageShape{"target": "", "val": "score", "name": "score", "label": "score", "x": 0.0, "y": 0.0, "visible": true}
	for _, mode := range []any{1.0, 2.0, 3.0, 4.0, "slider", "list"} {
		shape["mode"] = mode
		if err := validateReloadMonitor(shape); err != nil {
			t.Errorf("mode %v: %v", mode, err)
		}
	}
	for _, mode := range []any{nil, true, "unknown"} {
		shape["mode"] = mode
		if err := validateReloadMonitor(shape); err == nil {
			t.Errorf("accepted invalid mode %v", mode)
		}
	}
}

func TestMonitorBindingCapabilities(t *testing.T) {
	game := monitorEvalFixture{score: 1}
	root := reflect.ValueOf(&game).Elem()
	for _, appearance := range []ui.MonitorAppearance{ui.MonitorAppearanceDefault, ui.MonitorAppearanceScratchLarge} {
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
