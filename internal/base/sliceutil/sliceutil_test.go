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
	got := MoveToIndex([]int{1, 2, 3, 4}, 3, 1)
	want := []int{1, 4, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MoveToIndex = %v, want %v", got, want)
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
