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

import "testing"

func TestSliderTextUsesMonitorParsing(t *testing.T) {
	slider := NewSlider(10, 20, true)
	for _, tt := range []struct {
		text string
		want float64
	}{
		{"12", 12},
		{"--12", 15},
		{" 12 ", 15},
		{"", 15},
		{"NaN", 15},
		{"+Inf", 15},
		{"1e309", 15},
		{"0", 10},
		{"50", 20},
	} {
		t.Run(tt.text, func(t *testing.T) {
			if got := slider.ParseValue(tt.text); got != tt.want {
				t.Fatalf("ParseValue(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestSliderStepBoundaries(t *testing.T) {
	slider := NewSlider(0, 10.5, true)
	if got := slider.EffectiveMax(); got != 10 {
		t.Fatalf("EffectiveMax() = %v, want 10", got)
	}
	if got := slider.ParseValue("10.5"); got != 10 {
		t.Fatalf("ParseValue above last step = %v, want 10", got)
	}
	for _, tt := range []struct {
		min, position, want float64
	}{
		{0, 0.3, 0.3},
		{0.005, 0.305, 0.305},
	} {
		if got := NewSlider(tt.min, 1, false).Quantize(tt.position); got != tt.want {
			t.Errorf("Quantize(%v) with minimum %v = %.17g, want %.17g", tt.position, tt.min, got, tt.want)
		}
	}
}
