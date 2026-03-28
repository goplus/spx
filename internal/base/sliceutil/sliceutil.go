package sliceutil

func InsertAt[T any](slice []T, idx int, item T) []T {
	var zero T
	slice = append(slice, zero)
	copy(slice[idx+1:], slice[idx:])
	slice[idx] = item
	return slice
}

func DeleteAt[T any](slice []T, idx int) []T {
	last := len(slice) - 1
	copy(slice[idx:], slice[idx+1:])
	var zero T
	slice[last] = zero
	return slice[:last]
}

func MoveToEnd[T any](slice []T, idx int) []T {
	n := len(slice)
	item := slice[idx]
	copy(slice[idx:n-1], slice[idx+1:])
	slice[n-1] = item
	return slice
}

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

func CopyInto[T any](dst, src []T, minCap int) []T {
	requiredCap := max(minCap, len(src))
	if cap(dst) < requiredCap {
		dst = make([]T, len(src), requiredCap)
	} else {
		dst = dst[:len(src)]
	}
	copy(dst, src)
	return dst
}
