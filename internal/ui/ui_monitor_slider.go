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
	"strconv"
)

// MonitorSlider describes Scratch's min-anchored range and discrete step.
type MonitorSlider struct {
	Min, Max, Step float64
}

// monitorSliderState tracks the last rendered range independently of the view.
type monitorSliderState struct {
	config   MonitorSlider
	position float64
}

func (pself *UiMonitor) ReadSliderChange() (float64, bool) {
	return pself.readSliderChange(&mgr.UiMgr)
}

func (pself *UiMonitor) readSliderChange(sink monitorRenderSink) (float64, bool) {
	if pself.active != MonitorAppearanceSlider {
		return 0, false
	}
	position := sink.GetRangeValue(pself.views[MonitorAppearanceSlider].input.GetId())
	if position == pself.slider.position {
		return 0, false
	}
	pself.slider.position = position
	config := pself.slider.config
	// Divide whole hundredths to avoid multiplication noise for decimal steps.
	units := 1 / config.Step
	steps := math.Round((position - config.Min) / config.Step)
	value := (config.Min*units + steps) / units
	return value, true
}

func (pself *UiMonitor) renderSlider(sink monitorRenderSink, slider MonitorSlider, text string) {
	pself.slider.config = slider
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		number = slider.Min/2 + slider.Max/2
	}
	id := pself.views[MonitorAppearanceSlider].input.GetId()
	// HTML ranges exclude a maximum that does not fall on a valid step.
	maximum := slider.Min + slider.steps()*slider.Step
	number = math.Max(slider.Min, math.Min(maximum, number))
	sink.SetRange(id, slider.Min, maximum, slider.Step, number)
	pself.slider.position = sink.GetRangeValue(id)
}

func (s MonitorSlider) steps() float64 {
	return math.Floor((s.Max-s.Min)/s.Step + 1e-9)
}
