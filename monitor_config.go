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

import (
	"math"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/tools"
	"github.com/goplus/spx/v3/internal/ui"
)

func parseMonitorAppearance(v coreproject.StageShape) ui.MonitorAppearance {
	if v["mode"] == "slider" || v["mode"] == float64(monitorModeSlider) {
		return ui.MonitorAppearanceSlider
	}
	if v["mode"] == "list" || v["mode"] == float64(monitorModeList) {
		return ui.MonitorAppearanceList
	}
	mode := int(v["mode"].(float64))
	style, _ := coreproject.ShapeValue(v, "style", "default").(string)
	if style == monitorStyleScratch {
		if mode == monitorModeLarge {
			return ui.MonitorAppearanceScratchLarge
		}
		return ui.MonitorAppearanceScratch
	}
	if mode == monitorModeDefault {
		return ui.MonitorAppearanceDefault
	}
	return ui.MonitorAppearanceDefaultLarge
}

func parseMonitorColor(v coreproject.StageShape, appearance ui.MonitorAppearance) mathf.Color {
	if color, err := mathf.NewColorAny(coreproject.ShapeValue(v, "color")); err == nil {
		return color
	}
	switch {
	case appearance == ui.MonitorAppearanceList:
		return mathf.NewColorRGBAi(0xff, 0x66, 0x1a, 0xff)
	case appearance.IsScratch():
		return mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)
	default:
		return mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)
	}
}

func parseListMonitorDimensions(v coreproject.StageShape) mathf.Vec2 {
	dimension := func(key string, fallback, minimum float64) float64 {
		value := monitorNumber(v, key, fallback)
		if value <= 0 {
			return fallback
		}
		return math.Max(value, minimum)
	}
	return mathf.NewVec2(dimension("width", 100, 100), dimension("height", 200, 60))
}

func parseMonitorSlider(v coreproject.StageShape) ui.MonitorSlider {
	min, max := monitorNumber(v, "sliderMin", 0), monitorNumber(v, "sliderMax", 100)
	if min > max {
		min, max = max, min
	}
	step := 1.0
	if discrete, ok := v["isDiscrete"].(bool); ok && !discrete {
		step = 0.01
	}
	if math.IsInf((max-min)/step, 0) || math.IsInf(min/step, 0) || math.IsInf(max/step, 0) {
		min, max = 0, 100
	}
	return ui.MonitorSlider{Min: min, Max: max, Step: step}
}

// monitorNumber reads finite dimensions and range limits. Monitor scale retains
// its existing parsing rules and deliberately does not use this helper.
func monitorNumber(v coreproject.StageShape, key string, fallback float64) float64 {
	value, ok := tools.GetFloat(v[key])
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
}
