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

package sliceutil

import (
	"reflect"
	"testing"
)

func TestResetNil(t *testing.T) {
	for _, tc := range []struct {
		name       string
		initialCap int
		wantCap    int
	}{
		{"negative capacity", -1, 0},
		{"zero capacity", 0, 0},
		{"allocate capacity", 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Reset[*int](nil, tc.initialCap)
			if len(got) != 0 || cap(got) != tc.wantCap || (got == nil) != (tc.wantCap == 0) {
				t.Fatalf("Reset(nil, %d) len/cap/nil = %d/%d/%v, want 0/%d/%v", tc.initialCap, len(got), cap(got), got == nil, tc.wantCap, tc.wantCap == 0)
			}
			for i, value := range got[:cap(got)] {
				if value != nil {
					t.Fatalf("allocated element %d = %v, want nil", i, value)
				}
			}
		})
	}
}

func TestResetClearsAndReusesStorage(t *testing.T) {
	backing := make([]*int, 3, 5)
	for i := range backing {
		backing[i] = new(int)
	}
	// A non-nil slice ignores initialCap, even when it asks for more storage.
	got := Reset(backing, 10)
	if len(got) != 0 || cap(got) != cap(backing) || &got[:cap(got)][0] != &backing[0] {
		t.Fatalf("Reset did not preserve storage: len=%d cap=%d, want 0/%d", len(got), cap(got), cap(backing))
	}
	for i, value := range backing {
		if value != nil {
			t.Fatalf("Reset retained element %d: %v", i, value)
		}
	}
	// Resetting an already-empty buffer must remain non-nil and reusable.
	again := Reset(got, 10)
	if again == nil || len(again) != 0 || cap(again) != cap(backing) || &again[:cap(again)][0] != &backing[0] {
		t.Fatal("Reset lost the empty buffer's storage")
	}
	value := new(int)
	again = append(again, value)
	if backing[0] != value {
		t.Fatal("append after Reset did not reuse the backing array")
	}
	if empty := Reset(make([]int, 0), 10); empty == nil || len(empty) != 0 || cap(empty) != 0 {
		t.Fatalf("Reset allocated for a non-nil zero-capacity slice: len/cap/nil = %d/%d/%v", len(empty), cap(empty), empty == nil)
	}
}

func TestInsertAt(t *testing.T) {
	got := InsertAt([]int{1, 3}, 1, 2)
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InsertAt = %v, want %v", got, want)
	}
}

func TestDeleteAt(t *testing.T) {
	got := DeleteAt([]int{1, 2, 3}, 1)
	want := []int{1, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeleteAt = %v, want %v", got, want)
	}
}

func TestMoveToEnd(t *testing.T) {
	got := MoveToEnd([]int{1, 2, 3}, 0)
	want := []int{2, 3, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MoveToEnd = %v, want %v", got, want)
	}
}

func TestMoveToIndex(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to int
		want     []int
	}{
		{"backward", 3, 1, []int{1, 4, 2, 3}},
		{"forward", 0, 2, []int{2, 3, 1, 4}},
		{"same index", 1, 1, []int{1, 2, 3, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []int{1, 2, 3, 4}
			got := MoveToIndex(input, tc.from, tc.to)
			if !reflect.DeepEqual(got, tc.want) || &got[0] != &input[0] {
				t.Fatalf("MoveToIndex = %v, want in-place %v", got, tc.want)
			}
		})
	}
}

func TestCopyInto(t *testing.T) {
	dst := make([]int, 0, 8)
	got := CopyInto(dst, []int{1, 2, 3}, 4)
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CopyInto = %v, want %v", got, want)
	}
	if cap(got) != cap(dst) {
		t.Fatalf("CopyInto changed reusable capacity: got cap=%d want cap=%d", cap(got), cap(dst))
	}
}

func TestCopyIntoHonorsMinCapForReusableSlices(t *testing.T) {
	dst := make([]int, 0, 2)
	got := CopyInto(dst, []int{1}, 4)
	if cap(got) < 4 {
		t.Fatalf("CopyInto cap = %d, want at least 4", cap(got))
	}
}

func TestCopyIntoClearsTailAfterShortCopy(t *testing.T) {
	a, b, c := new(int), new(int), new(int)
	dst := []*int{a, b, c}
	got := CopyInto(dst, []*int{a}, 0)
	if len(got) != 1 || got[0] != a || cap(got) != cap(dst) {
		t.Fatalf("CopyInto = %v, cap %d", got, cap(got))
	}
	if tail := got[:len(dst)][1:]; tail[0] != nil || tail[1] != nil {
		t.Fatalf("CopyInto retained old tail: %v", tail)
	}
}

func TestCopyIntoOverlappingSource(t *testing.T) {
	a, b, c, d := new(int), new(int), new(int), new(int)
	dst := []*int{a, b, c, d}
	src := dst[1:3]
	got := CopyInto(dst, src, 0)
	if len(got) != 2 || got[0] != b || got[1] != c {
		t.Fatalf("CopyInto overlapping = %v", got)
	}
	if src[0] != b || src[1] != c || !reflect.DeepEqual(dst, []*int{a, b, c, d}) {
		t.Fatal("CopyInto changed overlapping source")
	}
	if &got[0] == &dst[0] {
		t.Fatal("CopyInto reused overlapping storage")
	}
}

func TestCopyIntoOverlappingEmptyDestination(t *testing.T) {
	backing := []int{1, 2, 3}
	got := CopyInto(backing[:0], backing[1:], 0)
	if !reflect.DeepEqual(got, []int{2, 3}) || !reflect.DeepEqual(backing, []int{1, 2, 3}) {
		t.Fatalf("CopyInto = %v, backing = %v", got, backing)
	}
}

func TestCopyIntoEmptyAndZeroSizedElements(t *testing.T) {
	if got := CopyInto[int](nil, nil, 0); got != nil {
		t.Fatalf("empty nil copy allocated storage: %v", got)
	}
	value := 7
	backing := []*int{&value}
	got := CopyInto(backing, nil, 0)
	if len(got) != 0 || cap(got) != 1 || backing[0] != nil {
		t.Fatalf("empty copy did not clear and retain storage: %v, cap=%d", backing, cap(got))
	}
	zeroSized := make([]struct{}, 2, 8)
	copied := CopyInto(zeroSized, zeroSized[:1], 0)
	if len(copied) != 1 || cap(copied) != cap(zeroSized) {
		t.Fatalf("zero-sized copy len/cap = %d/%d, want 1/%d", len(copied), cap(copied), cap(zeroSized))
	}
}
