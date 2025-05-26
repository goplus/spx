/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"math/rand"
	"time"

	"github.com/goplus/spx/internal/engine"
	"github.com/realdream-ai/mathf"
)

// -----------------------------------------------------------------------------

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Rand__0(from, to int) float64 {
	if to < from {
		to = from
	}
	return float64(from + rand.Intn(to-from+1))
}

func Rand__1(from, to float64) float64 {
	if to < from {
		to = from
	}
	return rand.Float64()*(to-from) + from
}

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

type Color struct {
	r, g, b, a float64
}

func toMathfColor(c Color) mathf.Color {
	return mathf.Color{R: c.r, G: c.g, B: c.b, A: c.a}
}
func toSpxColor(c mathf.Color) Color {
	return Color{c.R, c.G, c.B, c.A}
}

// h, s, b in range [0, 100], just like Scratch
func HSB(h, s, b float64) Color {
	color := mathf.NewColorHSV(h*3.6, s/100, b/100)
	color.A = 1
	return toSpxColor(color)
}

// h, s, b, a in range [0, 100], just like Scratch
func HSBA(h, s, b, a float64) Color {
	color := HSB(h, s, b)
	color.a = a / 100
	return color
}

// -----------------------------------------------------------------------------

func Exit__0(code int) {
	engine.RequestExit(int64(code))
}

func Exit__1() {
	engine.RequestExit(0)
}

// -----------------------------------------------------------------------------
