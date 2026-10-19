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

package engine

import (
	"math"
	"testing"

	mathf "github.com/goplus/spbase/mathf"
)

func TestHeadingToPoint(t *testing.T) {
	from := mathf.NewVec2(0, 0)

	tests := []struct {
		name string
		to   mathf.Vec2
		want float64
	}{
		{name: "right", to: mathf.NewVec2(10, 0), want: 90},
		{name: "up", to: mathf.NewVec2(0, 10), want: 0},
		{name: "left", to: mathf.NewVec2(-10, 0), want: -90},
		{name: "down", to: mathf.NewVec2(0, -10), want: 180},
		{name: "bottom left keeps raw heading", to: mathf.NewVec2(-10, -10), want: 225},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HeadingToPoint(from, tt.to); !(math.Abs(got-tt.want) <= 1e-9) {
				t.Fatalf("HeadingToPoint(%v, %v) = %v, want %v", from, tt.to, got, tt.want)
			}
		})
	}
}

func TestSign(t *testing.T) {
	for _, tt := range []struct {
		value float64
		want  int64
	}{
		{-5, -1}, {-0.25, -1}, {math.Inf(-1), -1},
		{0, 0}, {math.Copysign(0, -1), 0},
		{0.25, 1}, {5, 1}, {math.Inf(1), 1},
	} {
		if got := Sign(tt.value); got != tt.want {
			t.Errorf("Sign(%v) = %d, want %d", tt.value, got, tt.want)
		}
	}
}

func TestAngleConversions(t *testing.T) {
	for _, tt := range []struct{ degrees, radians float64 }{
		{0, 0}, {90, math.Pi / 2}, {-90, -math.Pi / 2},
		{180, math.Pi}, {360, 2 * math.Pi}, {-450, -2.5 * math.Pi},
	} {
		if got := DegToRad(tt.degrees); !(math.Abs(got-tt.radians) <= 1e-12) {
			t.Errorf("DegToRad(%v) = %v, want %v", tt.degrees, got, tt.radians)
		}
		if got := RadToDeg(tt.radians); !(math.Abs(got-tt.degrees) <= 1e-12) {
			t.Errorf("RadToDeg(%v) = %v, want %v", tt.radians, got, tt.degrees)
		}
		if got := Abs(tt.degrees); got != math.Abs(tt.degrees) {
			t.Errorf("Abs(%v) = %v, want %v", tt.degrees, got, math.Abs(tt.degrees))
		}
	}
	from, to := mathf.NewVec2(3, -4), mathf.NewVec2(6, -1)
	if got := AngleToPoint(from, to); !(math.Abs(got-math.Pi/4) <= 1e-12) {
		t.Errorf("AngleToPoint(%v, %v) = %v, want pi/4", from, to, got)
	}
	if got := HeadingToPoint(from, to); !(math.Abs(got-45) <= 1e-12) {
		t.Errorf("HeadingToPoint(%v, %v) = %v, want 45", from, to, got)
	}
}
