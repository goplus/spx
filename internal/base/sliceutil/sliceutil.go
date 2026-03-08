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

// InsertAt inserts an item at the specified index.
func InsertAt[T any](slice []T, idx int, item T) []T {
	n := len(slice)
	newSlice := make([]T, n+1)
	copy(newSlice[:idx], slice[:idx])
	newSlice[idx] = item
	copy(newSlice[idx+1:], slice[idx:])
	return newSlice
}

// DeleteAt removes an item at the specified index.
func DeleteAt[T any](slice []T, idx int) []T {
	n := len(slice)
	newSlice := make([]T, n-1)
	copy(newSlice[:idx], slice[:idx])
	copy(newSlice[idx:], slice[idx+1:])
	return newSlice
}

// MoveToEnd moves an item from idx to the end of the slice.
func MoveToEnd[T any](slice []T, idx int) []T {
	n := len(slice)
	item := slice[idx]
	newSlice := make([]T, n)
	copy(newSlice[:idx], slice[:idx])
	copy(newSlice[idx:n-1], slice[idx+1:])
	newSlice[n-1] = item
	return newSlice
}

// MoveToIndex moves an item from oldIdx to newIdx.
func MoveToIndex[T any](slice []T, oldIdx, newIdx int) []T {
	if oldIdx == newIdx {
		return slice
	}

	n := len(slice)
	item := slice[oldIdx]
	newSlice := make([]T, n)

	if oldIdx < newIdx {
		copy(newSlice[:oldIdx], slice[:oldIdx])
		copy(newSlice[oldIdx:newIdx], slice[oldIdx+1:newIdx+1])
		newSlice[newIdx] = item
		copy(newSlice[newIdx+1:], slice[newIdx+1:])
	} else {
		copy(newSlice[:newIdx], slice[:newIdx])
		newSlice[newIdx] = item
		copy(newSlice[newIdx+1:oldIdx+1], slice[newIdx:oldIdx])
		copy(newSlice[oldIdx+1:], slice[oldIdx+1:])
	}

	return newSlice
}

// CopyInto copies src into dst, reusing dst's capacity when possible.
func CopyInto[T any](dst, src []T, minCap int) []T {
	if dst == nil {
		dst = make([]T, 0, minCap)
	}
	if cap(dst) < len(src) {
		dst = make([]T, len(src))
	} else {
		dst = dst[:len(src)]
	}
	copy(dst, src)
	return dst
}
