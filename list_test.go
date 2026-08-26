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
	"fmt"
	"strconv"
	"testing"
)

func TestScratchStyleListEquality(t *testing.T) {
	list := NewList(1, "02", "Apple")

	tests := []struct {
		name     string
		value    any
		contains bool
		index    Pos
	}{
		{name: "integer and float", value: 1.0, contains: true, index: 0},
		{name: "numeric strings", value: 2, contains: true, index: 1},
		{name: "case insensitive strings", value: "apple", contains: true, index: 2},
		{name: "missing value", value: "banana", contains: false, index: Invalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := list.Contains(tt.value); got != tt.contains {
				t.Errorf("List.Contains(%v) = %v, want %v", tt.value, got, tt.contains)
			}
			if got := list.IndexOf(tt.value); got != tt.index {
				t.Errorf("List.IndexOf(%v) = %v, want %v", tt.value, got, tt.index)
			}
		})
	}
}

func TestValueEqualUsesScratchSemantics(t *testing.T) {
	if !NewValue(1).Equal(1.0) {
		t.Fatal("Value.Equal should treat integer and float values equally")
	}
	if !NewValue("Apple").Equal("apple") {
		t.Fatal("Value.Equal should compare strings case-insensitively")
	}
}

// TestValueBoolToInt tests bool to int conversion in Value.Int()
func TestValueBoolToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected int
	}{
		{
			name:     "true converts to 1",
			input:    true,
			expected: 1,
		},
		{
			name:     "false converts to 0",
			input:    false,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValue(tt.input)
			result := v.Int()
			if result != tt.expected {
				t.Errorf("NewValue(%v).Int() = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestValueBoolToFloat tests bool to float64 conversion in Value.Float()
func TestValueBoolToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected float64
	}{
		{
			name:     "true converts to 1.0",
			input:    true,
			expected: 1.0,
		},
		{
			name:     "false converts to 0.0",
			input:    false,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValue(tt.input)
			result := v.Float()
			if result != tt.expected {
				t.Errorf("NewValue(%v).Float() = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValueConversionFailureReturnsZero(t *testing.T) {
	tests := []struct {
		name  string
		value Value
	}{
		{name: "invalid string", value: NewValue("not a number")},
		{name: "unsupported type", value: NewValue([]int{1})},
		{name: "zero value", value: Value{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.Int(); got != 0 {
				t.Errorf("Value.Int() = %d, want 0", got)
			}
			if got := tt.value.Float(); got != 0 {
				t.Errorf("Value.Float() = %f, want 0", got)
			}
		})
	}
}

// TestToIntAnyBool tests toIntAny function with bool values
func TestToIntAnyBool(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
		shouldOk bool
	}{
		{
			name:     "true",
			input:    true,
			expected: 1,
			shouldOk: true,
		},
		{
			name:     "false",
			input:    false,
			expected: 0,
			shouldOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toIntAny(tt.input)
			if ok != tt.shouldOk {
				t.Errorf("toIntAny(%v) ok = %v, want %v", tt.input, ok, tt.shouldOk)
			}
			if result != tt.expected {
				t.Errorf("toIntAny(%v) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestToFloat64AnyBool tests toFloat64Any function with bool values
func TestToFloat64AnyBool(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
		shouldOk bool
	}{
		{
			name:     "true",
			input:    true,
			expected: 1.0,
			shouldOk: true,
		},
		{
			name:     "false",
			input:    false,
			expected: 0.0,
			shouldOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toFloat64Any(tt.input)
			if ok != tt.shouldOk {
				t.Errorf("toFloat64Any(%v) ok = %v, want %v", tt.input, ok, tt.shouldOk)
			}
			if result != tt.expected {
				t.Errorf("toFloat64Any(%v) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToFloat64AnyCompoundSignString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "double minus becomes positive",
			input:    "--5.5",
			expected: 5.5,
		},
		{
			name:     "plus minus becomes negative",
			input:    "+-12",
			expected: -12,
		},
		{
			name:     "generated negation pattern on negative value",
			input:    "-" + fmt.Sprintf("%v", -18.25),
			expected: 18.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toFloat64Any(tt.input)
			if !ok {
				t.Fatalf("toFloat64Any(%q) ok = false, want true", tt.input)
			}
			if result != tt.expected {
				t.Fatalf("toFloat64Any(%q) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeNumericStringSigns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		shouldOk bool
	}{
		{
			name:     "plain number unchanged",
			input:    "12.5",
			expected: "12.5",
			shouldOk: true,
		},
		{
			name:     "double minus becomes positive",
			input:    "--7",
			expected: "7",
			shouldOk: true,
		},
		{
			name:     "plus minus becomes negative",
			input:    "+-12",
			expected: "-12",
			shouldOk: true,
		},
		{
			name:     "trim surrounding spaces",
			input:    "  --18.25  ",
			expected: "18.25",
			shouldOk: true,
		},
		{
			name:     "signs only invalid",
			input:    "--",
			expected: "",
			shouldOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := normalizeNumericStringSigns(tt.input)
			if ok != tt.shouldOk {
				t.Fatalf("normalizeNumericStringSigns(%q) ok = %v, want %v", tt.input, ok, tt.shouldOk)
			}
			if result != tt.expected {
				t.Fatalf("normalizeNumericStringSigns(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseIntString(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name     string
		input    string
		expected int
		shouldOk bool
	}{
		{
			name:     "compound sign integer",
			input:    "--7",
			expected: 7,
			shouldOk: true,
		},
		{
			name:     "float string truncates like int conversion",
			input:    "--7.9",
			expected: 7,
			shouldOk: true,
		},
		{
			name:     "max int preserved",
			input:    strconv.Itoa(maxInt),
			expected: maxInt,
			shouldOk: true,
		},
		{
			name:     "signs only invalid",
			input:    "--",
			expected: 0,
			shouldOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := parseIntString(tt.input)
			if ok != tt.shouldOk {
				t.Fatalf("parseIntString(%q) ok = %v, want %v", tt.input, ok, tt.shouldOk)
			}
			if result != tt.expected {
				t.Fatalf("parseIntString(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToIntAnyCompoundSignString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "double minus becomes positive",
			input:    "--7",
			expected: 7,
		},
		{
			name:     "generated negation pattern on negative integer",
			input:    "-" + fmt.Sprintf("%v", -24),
			expected: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toIntAny(tt.input)
			if !ok {
				t.Fatalf("toIntAny(%q) ok = false, want true", tt.input)
			}
			if result != tt.expected {
				t.Fatalf("toIntAny(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToIntAnyPreservesLargeIntegerStringPrecision(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "max int string",
			input:    strconv.Itoa(maxInt),
			expected: maxInt,
		},
	}

	if int64(maxInt) > int64(1<<53) {
		beyondFloatExact := int64(1<<53) + 1
		tests = append(tests, struct {
			name     string
			input    string
			expected int
		}{
			name:     "integer above float64 exact range",
			input:    strconv.FormatInt(beyondFloatExact, 10),
			expected: int(beyondFloatExact),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toIntAny(tt.input)
			if !ok {
				t.Fatalf("toIntAny(%q) ok = false, want true", tt.input)
			}
			if result != tt.expected {
				t.Fatalf("toIntAny(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}
