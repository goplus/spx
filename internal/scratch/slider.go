/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package scratch

import (
	"math"
	"strconv"
)

// Slider describes Scratch's min-anchored range and discrete step.
type Slider struct {
	Min, Max, Step float64
}

// NewSlider normalizes monitor bounds and selects integer or hundredth steps.
func NewSlider(min, max float64, discrete bool) Slider {
	if math.IsNaN(min) || math.IsInf(min, 0) {
		min = 0
	}
	if math.IsNaN(max) || math.IsInf(max, 0) {
		max = 100
	}
	if min > max {
		min, max = max, min
	}
	step := 1.0
	if !discrete {
		step = 0.01
	}
	if math.IsInf((max-min)/step, 0) || math.IsInf(min/step, 0) || math.IsInf(max/step, 0) {
		min, max = 0, 100
	}
	return Slider{Min: min, Max: max, Step: step}
}

// EffectiveMax excludes a maximum that does not fall on a valid step.
func (s Slider) EffectiveMax() float64 {
	steps := math.Floor((s.Max-s.Min)/s.Step + 1e-9)
	return s.Min + steps*s.Step
}

// ParseValue clamps a numeric string to the slider range. Invalid or nonfinite
// values use the midpoint; parsing deliberately follows strconv.ParseFloat.
func (s Slider) ParseValue(text string) float64 {
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		number = s.Min/2 + s.Max/2
	}
	return math.Max(s.Min, math.Min(s.EffectiveMax(), number))
}

// Quantize rounds a position to a step anchored at the minimum.
func (s Slider) Quantize(position float64) float64 {
	// Divide whole hundredths to avoid multiplication noise for decimal steps.
	units := 1 / s.Step
	steps := math.Round((position - s.Min) / s.Step)
	return (s.Min*units + steps) / units
}
