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

import "testing"

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
		index int
		want  []any
	}{
		{"first", 0, []any{"new", "a", "b"}},
		{"middle", 1, []any{"a", "new", "b"}},
		{"end", 2, []any{"a", "b", "new"}},
		{"past end", 10, []any{"a", "b", "new"}},
		{"last", ListLast, []any{"a", "b", "new"}},
		{"invalid", ListInvalid, []any{"a", "b"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := NewList("a", "b")
			list.Insert(test.index, NewValue("new"))
			assertListItems(t, &list, test.want)
		})
	}
	var empty List
	empty.Insert(ListLast, "first")
	if empty.Len() != 1 || empty.At(ListLast).String() != "first" {
		t.Fatalf("Insert(ListLast) into empty list = %q", empty.String())
	}
}

func TestListDeleteAndReadPositions(t *testing.T) {
	for _, test := range []struct {
		name  string
		index int
		want  []any
	}{
		{"first", 0, []any{"b", "c"}},
		{"middle", 1, []any{"a", "c"}},
		{"last", ListLast, []any{"a", "b"}},
		{"all", ListAll, []any{}},
		{"invalid", ListInvalid, []any{"a", "b", "c"}},
		{"past end", 3, []any{"a", "b", "c"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := NewList("a", "b", "c")
			list.Delete(test.index)
			assertListItems(t, &list, test.want)
			for _, invalid := range []int{ListInvalid, ListAll, list.Len()} {
				if got := list.At(invalid); got.Raw() != nil {
					t.Errorf("At(%d) = %v, want empty value", invalid, got)
				}
			}
			list.Clear()
			list.Delete(ListLast)
			if list.Len() != 0 || list.At(ListLast).Raw() != nil {
				t.Fatal("empty list retained elements")
			}
		})
	}
}

func assertListItems(t *testing.T, list *List, want []any) {
	t.Helper()
	if list.Len() != len(want) {
		t.Fatalf("list length = %d, want %d", list.Len(), len(want))
	}
	for i, item := range want {
		if got := list.At(i).Raw(); got != item {
			t.Errorf("list item %d = %T(%v), want %T(%v)", i, got, got, item, item)
		}
	}
}

func TestListEquality(t *testing.T) {
	list := NewList(1, "02", "Apple")

	tests := []struct {
		name     string
		value    any
		contains bool
		index    int
	}{
		{name: "integer and float", value: 1.0, contains: true, index: 0},
		{name: "numeric strings", value: 2, contains: true, index: 1},
		{name: "case insensitive strings", value: "apple", contains: true, index: 2},
		{name: "missing value", value: "banana", contains: false, index: ListInvalid},
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

func TestFusedListCallsMatchOriginalSemantics(t *testing.T) {
	source := NewList(1, "02", "Apple")
	category := NewList(2, "apple")

	for _, test := range []struct {
		name     string
		index    int
		expected any
	}{
		{name: "first", index: 0, expected: 1.0},
		{name: "numeric string", index: 1, expected: 2},
		{name: "last", index: ListLast, expected: "apple"},
		{name: "invalid", index: ListInvalid, expected: ""},
		{name: "past end", index: 100, expected: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, want := source.EqualAt(test.index, test.expected), Equal(source.At(test.index), test.expected); got != want {
				t.Errorf("EqualAt(%d, %v) = %v, want %v", test.index, test.expected, got, want)
			}
			if got, want := category.ContainsAt(&source, test.index), category.Contains(source.At(test.index)); got != want {
				t.Errorf("ContainsAt(source, %d) = %v, want %v", test.index, got, want)
			}
		})
	}

	// Init can retain a Value. Fusion must not unwrap it more times than the
	// original At expression does.
	var nested List
	nested.Init(NewValue(2))
	if got, want := source.ContainsAt(&nested, 0), source.Contains(nested.At(0)); got != want {
		t.Fatalf("ContainsAt nested value = %v, want %v", got, want)
	}
	if got, want := nested.EqualAt(0, 2), Equal(nested.At(0), 2); got != want {
		t.Fatalf("EqualAt nested value = %v, want %v", got, want)
	}
}

func TestFusedListMutationAndTypedAccessMatchOriginalSemantics(t *testing.T) {
	source := NewList(1, "2", 3.5)
	destination := NewList(9, 8, 7)
	var value Value

	value.SetAt(&source, 1)
	if got, want := value.String(), source.At(1).String(); got != want {
		t.Fatalf("Value.SetAt = %q, want %q", got, want)
	}
	destination.SetAt(0, &source, 2)
	if got, want := destination.At(0).Float(), source.At(2).Float(); got != want {
		t.Fatalf("List.SetAt = %v, want %v", got, want)
	}
	if got, want := source.IntAt(1), source.At(1).Int(); got != want {
		t.Fatalf("List.IntAt = %d, want %d", got, want)
	}
	if got, want := source.FloatAt(2), source.At(2).Float(); got != want {
		t.Fatalf("List.FloatAt = %v, want %v", got, want)
	}
	if got, want := source.StringAt(0), source.At(0).String(); got != want {
		t.Fatalf("List.StringAt = %q, want %q", got, want)
	}
}

func TestListSetOutOfBoundsIsNoOp(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{name: "invalid", index: ListInvalid},
		{name: "all", index: ListAll},
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
	empty.Set(ListLast, 99)
	if empty.Len() != 0 {
		t.Fatalf("List.Set(ListLast, 99) changed an empty list, length = %d", empty.Len())
	}
}

func TestListSetLast(t *testing.T) {
	list := NewList(1, 2, 3)
	list.Set(ListLast, 9)
	if got := list.String(); got != "129" {
		t.Fatalf("List.Set(ListLast, 9) produced %q, want %q", got, "129")
	}
}
