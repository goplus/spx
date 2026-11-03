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
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/scratch"
)

// MonitorSlider describes Scratch's min-anchored range and discrete step.
type MonitorSlider = scratch.Slider

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
	return p.style.Slider.Quantize(position), true
}

func (p *UiMonitor) renderSlider(sink monitorRenderSink, text string) {
	slider := p.style.Slider
	id := p.view.input.GetId()
	sink.SetRange(id, slider.Min, slider.EffectiveMax(), slider.Step, slider.ParseValue(text))
	p.sliderPosition = sink.GetRangeValue(id)
	p.sliderReady = true
}
