/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package clrutil

import (
	"math"
	"math/rand"
)

// HSV2RGB converts hue (0-360), saturation (0-1), and brightness (0-1) to RGB.
//
func HSV2RGB(h, s, v float64) (uint8, uint8, uint8) {
	var r, g, b float64
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s = math.Max(0, math.Min(s, 1))
	v = math.Max(0, math.Min(v, 1))

	i := math.Floor(h / 60)
	f := (h / 60) - i
	p := v * (1 - s)
	q := v * (1 - (s * f))
	t := v * (1 - (s * (1 - f)))
	switch int(i) {
	case 0:
		r = v
		g = t
		b = p
	case 1:
		r = q
		g = v
		b = p
	case 2:
		r = p
		g = v
		b = t
	case 3:
		r = p
		g = q
		b = v
	case 4:
		r = t
		g = p
		b = v
	case 5:
		r = v
		g = p
		b = q
	}
	return uint8(math.Floor(r * 255)), uint8(math.Floor(g * 255)), uint8(math.Floor(b * 255))
}

// RGB2HSV converts RGB to an array containing the hue, saturation, and brightness.
//
func RGB2HSV(R, G, B uint8) (h, s, v float64) {
	var f, i float64
	r := float64(R) / 255
	g := float64(G) / 255
	b := float64(B) / 255
	x := math.Min(math.Min(r, g), b)
	v = math.Max(math.Max(r, g), b)
	if x == v {
		return // gray; hue arbitrarily reported as zero
	}
	if r == x {
		f = g - b
		i = 3
	} else if g == x {
		f = b - r
		i = 5
	} else {
		f = r - g
		i = 1
	}
	h = math.Mod((i-(f/(v-x)))*60, 360)
	s = (v - x) / v
	return
}

// ScaleBrightness changes color brightness.
//
func ScaleBrightness(r, g, b uint8, scale float64) (uint8, uint8, uint8) {
	h, s, v := RGB2HSV(r, g, b)
	val := math.Max(0, math.Min(scale*v, 1))
	return HSV2RGB(h, s, val)
}

// MixRGB mixs rgb1 with rgb2. 0 gives all rgb1, 1 gives rbg2, .5 mixes them 50/50.
//
func MixRGB(r1, g1, b1 uint8, r2, g2, b2 uint8, fraction float64) (r, g, b uint8) {
	if fraction <= 0 {
		return r1, g1, b1
	}
	if fraction >= 1 {
		return r2, g2, b2
	}
	r = uint8(fraction*float64(r2) + (1.0-fraction)*float64(r1))
	g = uint8(fraction*float64(g2) + (1.0-fraction)*float64(g1))
	b = uint8(fraction*float64(b2) + (1.0-fraction)*float64(b1))
	return
}

// Random returns a random color.
//
func Random() (uint8, uint8, uint8) {
	h := 360 * rand.Float64()
	s := 0.7 + (0.3 * rand.Float64())
	v := 0.6 + (0.4 * rand.Float64())
	return HSV2RGB(h, s, v)
}
