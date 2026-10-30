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
	"slices"
	"strconv"
	"testing"
	"time"

	coreruntime "github.com/goplus/spx/v4/internal/core/runtime"
	"github.com/goplus/spx/v4/internal/coroutine"
)

func TestListInitFromOwnsElementStorage(t *testing.T) {
	var source, copied List
	source.Init("first", "second")
	copied.InitFrom(&source)
	source.Set(0, "source")
	copied.Set(1, "copy")
	if source.String() != "source second" || copied.String() != "first copy" {
		t.Fatalf("InitFrom shares element storage: source=%q, copy=%q", source.String(), copied.String())
	}
	copied.InitFrom(&copied)
	if copied.String() != "first copy" {
		t.Fatalf("InitFrom self lost items: %q", copied.String())
	}
	var empty List
	copied.InitFrom(&empty)
	if copied.Len() != 0 {
		t.Fatalf("InitFrom empty left %d items", copied.Len())
	}
}

func TestListInsertPositions(t *testing.T) {
	for _, test := range []struct {
		name  string
		index Pos
		want  []any
	}{
		{"first", 0, []any{"new", "a", "b"}},
		{"middle", 1, []any{"a", "new", "b"}},
		{"end", 2, []any{"a", "b", "new"}},
		{"past end", 10, []any{"a", "b", "new"}},
		{"last", Last, []any{"a", "b", "new"}},
		{"invalid", Invalid, []any{"a", "b"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := NewList("a", "b")
			list.Insert(test.index, NewValue("new"))
			if !slices.Equal(list.data, test.want) {
				t.Fatalf("Insert(%d) = %v, want %v", test.index, list.data, test.want)
			}
		})
	}
	var empty List
	empty.Insert(Last, "first")
	if empty.Len() != 1 || empty.At(Last).String() != "first" {
		t.Fatalf("Insert(Last) into empty list = %v", empty.data)
	}
}

func TestListDeleteAndReadPositions(t *testing.T) {
	for _, test := range []struct {
		name  string
		index Pos
		want  []any
	}{
		{"first", 0, []any{"b", "c"}},
		{"middle", 1, []any{"a", "c"}},
		{"last", Last, []any{"a", "b"}},
		{"all", All, []any{}},
		{"invalid", Invalid, []any{"a", "b", "c"}},
		{"past end", 3, []any{"a", "b", "c"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := NewList("a", "b", "c")
			list.Delete(test.index)
			if !slices.Equal(list.data, test.want) {
				t.Fatalf("Delete(%d) = %v, want %v", test.index, list.data, test.want)
			}
			for _, invalid := range []Pos{Invalid, All, Pos(list.Len())} {
				if got := list.At(invalid); got.data != nil {
					t.Errorf("At(%d) = %v, want empty value", invalid, got)
				}
			}
			list.Clear()
			list.Delete(Last)
			if list.Len() != 0 || list.At(Last).data != nil {
				t.Fatal("empty list retained elements")
			}
		})
	}
}

func TestStyleListEquality(t *testing.T) {
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

func TestListSetOutOfBoundsIsNoOp(t *testing.T) {
	tests := []struct {
		name  string
		index Pos
	}{
		{name: "invalid", index: Invalid},
		{name: "all", index: All},
		{name: "other negative", index: -100},
		{name: "past end", index: 3},
		{name: "far past end", index: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := NewList(1, 2, 3)
			list.Set(tt.index, 99)
			if got := list.String(); got != "123" {
				t.Fatalf("List.Set(%d, 99) changed list to %q, want %q", tt.index, got, "123")
			}
		})
	}

	empty := NewList()
	empty.Set(Last, 99)
	if empty.Len() != 0 {
		t.Fatalf("List.Set(Last, 99) changed an empty list, length = %d", empty.Len())
	}
}

func TestListSetLast(t *testing.T) {
	list := NewList(1, 2, 3)
	list.Set(Last, 9)
	if got := list.String(); got != "129" {
		t.Fatalf("List.Set(Last, 9) produced %q, want %q", got, "129")
	}
}

func TestValueEqualUsesCompatibilitySemantics(t *testing.T) {
	if !NewValue(1).Equal(1.0) {
		t.Fatal("Value.Equal should treat integer and float values equally")
	}
	if !NewValue("Apple").Equal("apple") {
		t.Fatal("Value.Equal should compare strings case-insensitively")
	}
}

func TestValueNumericConversions(t *testing.T) {
	for _, test := range []struct {
		input any
		want  int
	}{
		{true, 1},
		{false, 0},
		{int(7), 7},
		{int8(7), 7},
		{int16(7), 7},
		{int32(7), 7},
		{int64(7), 7},
		{uint(7), 7},
		{uint8(7), 7},
		{uint16(7), 7},
		{uint32(7), 7},
		{uint64(7), 7},
		{float32(7), 7},
		{float64(7), 7},
	} {
		t.Run(fmt.Sprintf("%T(%v)", test.input, test.input), func(t *testing.T) {
			value := NewValue(test.input)
			if got := value.Int(); got != test.want {
				t.Errorf("Value.Int() = %d, want %d", got, test.want)
			}
			if got := value.Float(); got != float64(test.want) {
				t.Errorf("Value.Float() = %g, want %d", got, test.want)
			}
			if got, ok := toIntAny(test.input); !ok || got != test.want {
				t.Errorf("toIntAny() = %d, %v; want %d, true", got, ok, test.want)
			}
			if got, ok := toFloat64Any(test.input); !ok || got != float64(test.want) {
				t.Errorf("toFloat64Any() = %g, %v; want %d, true", got, ok, test.want)
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

func TestFloatStringConversion(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  float64
		ok    bool
	}{
		{"double minus becomes positive", "--5.5", 5.5, true},
		{"plus minus becomes negative", "+-12", -12, true},
		{"generated negation pattern on negative value", "-" + fmt.Sprintf("%v", -18.25), 18.25, true},
		{"empty after trimming", " ", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := parseFloatString(test.input); ok != test.ok || got != test.want {
				t.Errorf("parseFloatString(%q) = %g, %v; want %g, %v", test.input, got, ok, test.want, test.ok)
			}
			if got, ok := toFloat64Any(test.input); ok != test.ok || got != test.want {
				t.Errorf("toFloat64Any(%q) = %g, %v; want %g, %v", test.input, got, ok, test.want, test.ok)
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
	for _, test := range []struct {
		name  string
		input string
		want  int
		ok    bool
	}{
		{"compound sign integer", "--7", 7, true},
		{"float string truncates like int conversion", "--7.9", 7, true},
		{"max int preserved", strconv.Itoa(maxInt), maxInt, true},
		{"signs only invalid", "--", 0, false},
		{"empty", "", 0, false},
		{"empty after trimming", "  ", 0, false},
		{"plus only invalid", "+", 0, false},
		{"positive overflow", "1e30", 0, false},
		{"negative overflow", "-1e30", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, ok := parseIntString(test.input); ok != test.ok || got != test.want {
				t.Fatalf("parseIntString(%q) = %d, %v; want %d, %v", test.input, got, ok, test.want, test.ok)
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

func TestValueSet(t *testing.T) {
	var absent *Value
	absent.Set("ignored")
	value := NewValue("old")
	value.Set(NewValue("new"))
	if value.String() != "new" {
		t.Fatalf("Set = %v", value)
	}
}

func TestToIntAnyOutOfRange(t *testing.T) {
	inputs := []any{^uint(0), ^uint64(0), float32(1e30), float64(1e30), float32(-1e30), float64(-1e30)}
	if strconv.IntSize == 32 {
		inputs = append(inputs, int64(1<<40), int64(-1<<31)-1, uint32(1<<31))
	}
	for _, input := range inputs {
		t.Run(fmt.Sprintf("%T(%v)", input, input), func(t *testing.T) {
			if got, ok := toIntAny(input); ok || got != 0 {
				t.Fatalf("toIntAny() = %d, %v; want 0, false", got, ok)
			}
		})
	}
}

func TestListRandomPositionUsesValidElement(t *testing.T) {
	previous := scriptRandom
	scriptRandom = newRandomState()
	t.Cleanup(func() { scriptRandom = previous })
	list := NewList("a", "b", "c")
	for range 20 {
		if item := list.At(Random); !list.Contains(item.data) {
			t.Fatalf("random item = %v", item)
		}
	}
	list.Clear()
	if list.At(Random).data != nil {
		t.Fatal("empty list has a random item")
	}
}

// Model ClearTT's 8-list and 3-list initialization loops without interpreter dispatch.
func BenchmarkChessListInitialization(b *testing.B) {
	for _, checkBudget := range []bool{false, true} {
		b.Run(fmt.Sprintf("warp-check=%t", checkBudget), func(b *testing.B) {
			co := coroutine.New(nil)
			b.Cleanup(func() { co.RunAfterStop(nil) })
			b.ReportAllocs()
			b.ResetTimer()
			var initialized int
			thread := co.Create("initialize", func(thread coroutine.Thread) {
				thread.SetWarp(true)
				yield := func() {}
				if checkBudget {
					yield = func() { thread.ShouldYield(time.Hour) }
				}
				for range b.N {
					var lists [11]List
					coreruntime.Repeat(262144, func() {
						for i := range 8 {
							lists[i].Append(0)
						}
					}, yield)
					coreruntime.Repeat(262144, func() {
						for i := 8; i < 11; i++ {
							lists[i].Append(0)
						}
					}, yield)
					initialized = lists[10].Len()
				}
			})
			co.Join(thread)
			if initialized != 262144 {
				b.Fatal("initialization lost list elements")
			}
		})
	}
}
