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
	var zero T
	slice = append(slice, zero)
	copy(slice[idx+1:], slice[idx:])
	slice[idx] = item
	return slice
}

// DeleteAt removes an item at the specified index.
func DeleteAt[T any](slice []T, idx int) []T {
	last := len(slice) - 1
	copy(slice[idx:], slice[idx+1:])
	var zero T
	slice[last] = zero
	return slice[:last]
}

// MoveToEnd moves an item from idx to the end of the slice.
func MoveToEnd[T any](slice []T, idx int) []T {
	n := len(slice)
	item := slice[idx]
	copy(slice[idx:n-1], slice[idx+1:])
	slice[n-1] = item
	return slice
}

// MoveToIndex moves an item from oldIdx to newIdx.
func MoveToIndex[T any](slice []T, oldIdx, newIdx int) []T {
	if oldIdx == newIdx {
		return slice
	}

	item := slice[oldIdx]
	if oldIdx < newIdx {
		copy(slice[oldIdx:newIdx], slice[oldIdx+1:newIdx+1])
		slice[newIdx] = item
	} else {
		copy(slice[newIdx+1:oldIdx+1], slice[newIdx:oldIdx])
		slice[newIdx] = item
	}

	return slice
}

// CopyInto copies src into dst, reusing dst's capacity when possible.
func CopyInto[T any](dst, src []T, minCap int) []T {
	requiredCap := len(src)
	if minCap > requiredCap {
		requiredCap = minCap
	}
	if cap(dst) < requiredCap {
		dst = make([]T, len(src), requiredCap)
	} else {
		dst = dst[:len(src)]
	}
	copy(dst, src)
	return dst
}
