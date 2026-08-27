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
	"os"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/ui"
)

func TestParseMonitorAppearance(t *testing.T) {
	tests := []struct {
		name  string
		mode  int
		style any
		want  ui.MonitorAppearance
	}{
		{name: "default", mode: 1, want: ui.MonitorAppearanceDefault},
		{name: "default large", mode: 2, want: ui.MonitorAppearanceDefaultLarge},
		{name: "default unsupported mode", mode: 3, want: ui.MonitorAppearanceDefaultLarge},
		{name: "explicit default", mode: 1, style: "default", want: ui.MonitorAppearanceDefault},
		{name: "scratch", mode: 1, style: "scratch", want: ui.MonitorAppearanceScratch},
		{name: "scratch large", mode: 2, style: "scratch", want: ui.MonitorAppearanceScratchLarge},
		{name: "scratch unsupported mode", mode: 3, style: "scratch", want: ui.MonitorAppearanceScratch},
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
