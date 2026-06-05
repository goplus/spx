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
	"testing"
)

func TestPenColorParamFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PenColorParam
	}{
		{name: "color lowercase", input: "color", expected: PenHue},
		{name: "color uppercase", input: "COLOR", expected: PenHue},
		{name: "color mixed case", input: "Color", expected: PenHue},
		{name: "saturation", input: "saturation", expected: PenSaturation},
		{name: "saturation uppercase", input: "SATURATION", expected: PenSaturation},
		{name: "brightness", input: "brightness", expected: PenBrightness},
		{name: "brightness uppercase", input: "BRIGHTNESS", expected: PenBrightness},
		{name: "transparency", input: "transparency", expected: PenTransparency},
		{name: "transparency uppercase", input: "TRANSPARENCY", expected: PenTransparency},
		{name: "unknown string falls back to PenNone", input: "unknown", expected: PenNone},
		{name: "empty string falls back to PenNone", input: "", expected: PenNone},
		{name: "numeric string falls back to PenNone", input: "123", expected: PenNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PenColorParamFromString(tt.input)
			if got != tt.expected {
				t.Errorf("PenColorParamFromString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
