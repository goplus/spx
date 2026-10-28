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

	"github.com/goplus/spx/v4/internal/engine"
)

// MonitorSlider describes Scratch's min-anchored range and discrete step.
type MonitorSlider struct {
	Min, Max, Step float64
}

func (p *UiMonitor) ReadSliderChange() (float64, bool) {
	return p.readSliderChange(&engine.Managers().UiMgr)
}

func (p *UiMonitor) readSliderChange(sink monitorRenderSink) (float64, bool) {
	if !p.sliderReady {
		return 0, false
	}
	position := sink.GetRangeValue(p.view.input.GetId())
	if position == p.sliderPosition {
		return 0, false
	}
	p.sliderPosition = position
	config := p.style.Slider
	// Divide whole hundredths to avoid multiplication noise for decimal steps.
	units := 1 / config.Step
	steps := math.Round((position - config.Min) / config.Step)
	value := (config.Min*units + steps) / units
	return value, true
}

func (p *UiMonitor) renderSlider(sink monitorRenderSink, text string) {
	slider := p.style.Slider
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		number = slider.Min/2 + slider.Max/2
	}
	id := p.view.input.GetId()
	// HTML ranges exclude a maximum that does not fall on a valid step.
	maximum := slider.Min + slider.steps()*slider.Step
	number = math.Max(slider.Min, math.Min(maximum, number))
	sink.SetRange(id, slider.Min, maximum, slider.Step, number)
	p.sliderPosition = sink.GetRangeValue(id)
	p.sliderReady = true
}

func (s MonitorSlider) steps() float64 {
	return math.Floor((s.Max-s.Min)/s.Step + 1e-9)
}
