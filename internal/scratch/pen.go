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
	"strings"

	"github.com/goplus/spbase/mathf"
)

// Pen color parameters retain their SPX values.
const (
	PenHue = iota
	PenSaturation
	PenBrightness
	PenTransparency
	PenNone
)

// PenColorParamFromString accepts color, saturation, brightness, and
// transparency, ignoring case. Unknown names return PenNone.
func PenColorParamFromString(name string) int {
	switch strings.ToLower(name) {
	case "color":
		return PenHue
	case "saturation":
		return PenSaturation
	case "brightness":
		return PenBrightness
	case "transparency":
		return PenTransparency
	default:
		return PenNone
	}
}

// LegacyPenColor converts Scratch 2's hue/shade pair and transparency to RGBA.
// Hue uses a 100-unit cycle; shade mirrors around 100 in its 200-unit cycle.
func LegacyPenColor(hue, shade, transparency float64) mathf.Color {
	color := mathf.NewColorHSV((hue/100)*360, 1, 1)
	if shade > 100 {
		shade = 200 - shade
	}
	if shade < 50 {
		color = mathf.ColorBlack().Lerp(color, (10+shade)/60)
	} else {
		color = color.Lerp(mathf.ColorWhite(), (shade-50)/60)
	}
	color.A = 1 - transparency/100
	return color
}

// WrapLegacyPenShade wraps a Scratch 2 pen shade using its 200-unit cycle.
func WrapLegacyPenShade(shade float64) float64 {
	shade = math.Mod(shade, 200)
	if shade < 0 {
		shade += 200
	}
	return shade
}

// WrapPenHue wraps a pen hue percentage using its 100-unit cycle.
func WrapPenHue(hue float64) float64 {
	hue = math.Mod(hue, 100)
	if hue < 0 {
		hue += 100
	}
	return hue
}
