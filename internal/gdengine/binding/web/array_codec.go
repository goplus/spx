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

package webffi

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

const (
	// Keep malformed bridge counts from requesting multi-GB allocations.
	maxGdArrayElements = 16 * 1024 * 1024
	maxGdArrayBytes    = 256 * 1024 * 1024
)

type arrayValue struct {
	Count int32
	Type  int32
	Data  any
}

func newArrayValue[T any](arrayType int32, data []T) *arrayValue {
	count, ok := checkedArrayCount(len(data))
	if !ok {
		return nil
	}
	return &arrayValue{Type: arrayType, Count: count, Data: data}
}

func float32Slice(slice []float64) []float32 {
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func encodeArrayData(arrayType int32, data any) ([]byte, error) {
	if data == nil {
		return []byte{}, nil
	}

	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		switch arr := data.(type) {
		case []int64:
			return sliceBytes(arr), nil
		case []uint64:
			return sliceBytes(arr), nil
		default:
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
	case GdArrayTypeFloat:
		val, ok := data.([]float32)
		if !ok {
			slice, ok := data.([]float64)
			if !ok {
				return []byte{}, fmt.Errorf("array type is not supported: %T", data)
			}
			val = float32Slice(slice)
		}
		return sliceBytes(val), nil
	case GdArrayTypeBool:
		arr, ok := data.([]bool)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		return encodeBoolArray(arr), nil
	case GdArrayTypeByte:
		arr, ok := data.([]byte)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		return arr, nil
	case GdArrayTypeString:
		arr, ok := data.([]string)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		return encodeStringArray(arr)
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

// validateArrayShape bounds allocation and checks fixed-width payload lengths.
// String offsets and terminators are checked by decodeStringArray.
func validateArrayShape(arrayType int32, count int32, byteLength int) error {
	if count < 0 || count > maxGdArrayElements || byteLength < 0 || byteLength > maxGdArrayBytes {
		return fmt.Errorf("array size is invalid")
	}
	if arrayType == GdArrayTypeString {
		if count == 0 && byteLength != 0 || int64(byteLength) < int64(count)*9 {
			return fmt.Errorf("string array size is invalid")
		}
		return nil
	}
	size, ok := arrayElementSize(arrayType)
	if !ok || int64(byteLength) != int64(count)*int64(size) {
		return fmt.Errorf("array type or data length is invalid")
	}
	return nil
}

func decodeArrayData(arrayType int32, data []byte, count int32) (any, error) {
	if err := validateArrayShape(arrayType, count, len(data)); err != nil {
		return nil, err
	}
	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		return sliceFromBytes[int64](data), nil
	case GdArrayTypeFloat:
		return sliceFromBytes[float32](data), nil
	case GdArrayTypeBool:
		result := make([]bool, len(data))
		for i, value := range data {
			result[i] = value != 0
		}
		return result, nil
	case GdArrayTypeByte:
		if data == nil {
			data = []byte{}
		}
		return data, nil
	default: // Shape validation leaves only strings.
		return decodeStringArray(data, count)
	}
}

func encodeBoolArray(data []bool) []byte {
	result := make([]byte, len(data))
	for i, value := range data {
		if value {
			result[i] = 1
		}
	}
	return result
}

// Strings use [offset, byte length] pairs followed by NUL-terminated UTF-8 data.
// Offsets are relative to the buffer, so the same layout works across memories.
func encodeStringArray(data []string) ([]byte, error) {
	if len(data) > maxGdArrayElements || len(data) > maxGdArrayBytes/8 {
		return nil, fmt.Errorf("array size is invalid")
	}
	size := len(data) * 8
	for _, str := range data {
		if len(str) >= maxGdArrayBytes-size {
			return nil, fmt.Errorf("array data is too large")
		}
		size += len(str) + 1
	}
	result := make([]byte, size)
	offset := len(data) * 8
	for i, str := range data {
		binary.LittleEndian.PutUint32(result[i*8:], uint32(offset))
		binary.LittleEndian.PutUint32(result[i*8+4:], uint32(len(str)))
		copy(result[offset:], str)
		offset += len(str) + 1
	}
	return result, nil
}

func validateStringArray(data []byte, size int32) error {
	if size < 0 || size > maxGdArrayElements || int64(size)*8 > int64(len(data)) {
		return fmt.Errorf("string array table is invalid")
	}
	offset := uint64(size) * 8
	for i := 0; i < int(size); i++ {
		start := uint64(binary.LittleEndian.Uint32(data[i*8:]))
		length := uint64(binary.LittleEndian.Uint32(data[i*8+4:]))
		if start != offset || start+length >= uint64(len(data)) || data[start+length] != 0 {
			return fmt.Errorf("string array range is invalid")
		}
		offset = start + length + 1
	}
	if offset != uint64(len(data)) {
		return fmt.Errorf("string array has trailing bytes")
	}
	return nil
}

func decodeStringArray(data []byte, size int32) ([]string, error) {
	if err := validateStringArray(data, size); err != nil {
		return nil, err
	}
	result := make([]string, size)
	for i := range result {
		start := binary.LittleEndian.Uint32(data[i*8:])
		length := binary.LittleEndian.Uint32(data[i*8+4:])
		result[i] = string(data[start : start+length])
	}
	return result, nil
}

// Check before allocating or converting the source slice.
func checkedArrayCount(length int) (int32, bool) {
	if length < 0 || length > maxGdArrayElements {
		return 0, false
	}
	return int32(length), true
}

func sliceBytes[T int64 | uint64 | float32](data []T) []byte {
	var element T
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*int(unsafe.Sizeof(element)))
}

// The result shares storage with data; callers validate its byte length first.
func sliceFromBytes[T int64 | float32](data []byte) []T {
	if len(data) == 0 {
		return []T{}
	}
	var element T
	return unsafe.Slice((*T)(unsafe.Pointer(unsafe.SliceData(data))), len(data)/int(unsafe.Sizeof(element)))
}
