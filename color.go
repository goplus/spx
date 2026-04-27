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

import "github.com/goplus/spbase/mathf"

// Color represents an RGBA color.
//
// XGo treats structs whose fields are named X_0, X_1, ... as tuple types, so
// script code can construct colors with Color(r, g, b, a).
type Color struct {
	X_0, X_1, X_2, X_3 float64
}

// HSB creates a color from HSB values.
// h, s, b in range [0, 100], just like Scratch
func HSB(h, s, b float64) Color {
	color := mathf.NewColorHSV(h*3.6, s/100, b/100)
	color.A = 1
	return toSpxColor(color)
}

// HSBA creates a color from HSBA values.
// h, s, b, a in range [0, 100], just like Scratch
func HSBA(h, s, b, a float64) Color {
	color := HSB(h, s, b)
	color.X_3 = a / 100
	return color
}

// NewColor creates a color from a supported color value.
// It accepts packed RGB numbers, hex strings, and color names.
func NewColor(color any) Color {
	c, err := mathf.NewColorAny(color)
	if err != nil {
		return Color{}
	}
	return toSpxColor(c)
}

func toMathfColor(c Color) mathf.Color {
	return mathf.Color{R: c.X_0, G: c.X_1, B: c.X_2, A: c.X_3}
}

func toSpxColor(c mathf.Color) Color {
	return Color{c.R, c.G, c.B, c.A}
}
