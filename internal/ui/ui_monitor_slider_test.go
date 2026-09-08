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
	"math"
	"testing"
)

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
