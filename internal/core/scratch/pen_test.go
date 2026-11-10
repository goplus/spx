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

package scratch

import (
	"math"
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestPenWrappingPreservesNumericEdges(t *testing.T) {
	for _, rule := range []struct {
		name   string
		wrap   func(float64) float64
		period float64
	}{
		{"hue", WrapPenHue, 100},
		{"legacy shade", WrapLegacyPenShade, 200},
	} {
		t.Run(rule.name, func(t *testing.T) {
			for _, tt := range []struct {
				name        string
				value, want float64
			}{
				{"zero", 0, 0},
				{"negative zero", math.Copysign(0, -1), math.Copysign(0, -1)},
				{"one cycle", rule.period, 0},
				{"negative cycle", -rule.period, math.Copysign(0, -1)},
				{"negative value", -25, rule.period - 25},
				{"positive wrap", rule.period + 25, 25},
				{"smallest positive", math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64},
				{"smallest negative", -math.SmallestNonzeroFloat64, rule.period},
				{"NaN", math.NaN(), math.NaN()},
				{"positive infinity", math.Inf(1), math.NaN()},
				{"negative infinity", math.Inf(-1), math.NaN()},
			} {
				t.Run(tt.name, func(t *testing.T) {
					got := rule.wrap(tt.value)
					if math.IsNaN(tt.want) && math.IsNaN(got) {
						return
					}
					if math.Float64bits(got) != math.Float64bits(tt.want) {
						t.Fatalf("wrap(%v) = %v, want %v", tt.value, got, tt.want)
					}
				})
			}
		})
	}
}

func TestLegacyPenColorShadeAndTransparency(t *testing.T) {
	for _, tt := range []struct {
		name                string
		shade, transparency float64
		want                mathf.Color
	}{
		{"dark shade", 25, 25, mathf.NewColor(35.0/60, 0, 0, 0.75)},
		{"mirrored dark shade", 175, 25, mathf.NewColor(35.0/60, 0, 0, 0.75)},
		{"full hue", 50, 0, mathf.NewColor(1, 0, 0, 1)},
		{"light shade", 80, 100, mathf.NewColor(1, 0.5, 0.5, 0)},
		{"mirrored light shade", 120, 100, mathf.NewColor(1, 0.5, 0.5, 0)},
		{"negative transparency", 50, -25, mathf.NewColor(1, 0, 0, 1.25)},
		{"excess transparency", 50, 125, mathf.NewColor(1, 0, 0, -0.25)},
		{"NaN transparency", 50, math.NaN(), mathf.NewColor(1, 0, 0, math.NaN())},
		{"positive infinite transparency", 50, math.Inf(1), mathf.NewColor(1, 0, 0, math.Inf(-1))},
		{"negative infinite transparency", 50, math.Inf(-1), mathf.NewColor(1, 0, 0, math.Inf(1))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := LegacyPenColor(0, tt.shade, tt.transparency)
			if got.R != tt.want.R || got.G != tt.want.G || got.B != tt.want.B ||
				(got.A != tt.want.A && !(math.IsNaN(got.A) && math.IsNaN(tt.want.A))) {
				t.Fatalf("LegacyPenColor(0, %v, %v) = %v, want %v", tt.shade, tt.transparency, got, tt.want)
			}
		})
	}
}
