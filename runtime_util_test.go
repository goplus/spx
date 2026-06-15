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

func TestRand0SwapsReversedBounds(t *testing.T) {
	const (
		from = 93
		to   = -220
	)

	sawNonUpperBound := false
	for i := 0; i < 100; i++ {
		got := Rand__0(from, to)
		if got < to || got > from {
			t.Fatalf("Rand__0(%d, %d) = %v, want value in [%d, %d]", from, to, got, to, from)
		}
		if got != float64(from) {
			sawNonUpperBound = true
		}
	}

	if !sawNonUpperBound {
		t.Fatalf("Rand__0(%d, %d) only returned the upper bound; reversed bounds should produce values across the range", from, to)
	}
}

func TestCharAt(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		idx      int
		expected string
	}{
		{
			name:     "first character",
			s:        "hello",
			idx:      0,
			expected: "h",
		},
		{
			name:     "last character",
			s:        "hello",
			idx:      4,
			expected: "o",
		},
		{
			name:     "middle character",
			s:        "hello",
			idx:      2,
			expected: "l",
		},
		{
			name:     "index equals length returns empty string",
			s:        "hello",
			idx:      5,
			expected: "",
		},
		{
			name:     "negative index returns empty string",
			s:        "hello",
			idx:      -1,
			expected: "",
		},
		{
			name:     "index exceeds length returns empty string",
			s:        "hello",
			idx:      6,
			expected: "",
		},
		{
			name:     "empty string returns empty string",
			s:        "",
			idx:      0,
			expected: "",
		},
		{
			name:     "unicode multibyte character",
			s:        "你好世界",
			idx:      0,
			expected: "你",
		},
		{
			name:     "unicode last character",
			s:        "你好世界",
			idx:      3,
			expected: "界",
		},
		{
			name:     "unicode index out of range",
			s:        "你好世界",
			idx:      4,
			expected: "",
		},
		{
			name:     "single character string",
			s:        "a",
			idx:      0,
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CharAt(tt.s, tt.idx)
			if got != tt.expected {
				t.Errorf("CharAt(%q, %d) = %q, want %q", tt.s, tt.idx, got, tt.expected)
			}
		})
	}
}

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
