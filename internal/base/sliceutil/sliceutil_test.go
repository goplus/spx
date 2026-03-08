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
