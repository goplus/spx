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
	GdArrayTypeUnknown = 0
	GdArrayTypeInt64   = 1
	GdArrayTypeFloat   = 2
	GdArrayTypeBool    = 3
	GdArrayTypeString  = 4
	GdArrayTypeByte    = 5
	GdArrayTypeGdObj   = 6
	// Keep malformed bridge counts from requesting multi-GB allocations.
	maxGdArrayElements = 16 * 1024 * 1024
	maxGdArrayBytes    = 256 * 1024 * 1024
)

type GdArrayInfo struct {
	Size int32
	Type int32
	Data any
}

const (
	maxInt32 = int(^uint32(0) >> 1)
)

func validateGdArraySize(info *GdArrayInfo) error {
	if info == nil {
		return fmt.Errorf("nil GdArrayInfo")
	}
	if info.Data == nil {
		if info.Size != 0 {
			return fmt.Errorf("array data is nil for non-empty array")
		}
		switch info.Type {
		case GdArrayTypeInt64, GdArrayTypeFloat, GdArrayTypeBool,
			GdArrayTypeString, GdArrayTypeByte, GdArrayTypeGdObj:
			return nil
		default:
			return fmt.Errorf("array type is not supported: %d", info.Type)
		}
	}

	var length int
	switch info.Type {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		arr, ok := info.Data.([]int64)
		if !ok {
			return fmt.Errorf("array type is not supported: %T", info.Data)
		}
		length = len(arr)
	case GdArrayTypeFloat:
		switch arr := info.Data.(type) {
		case []float32:
			length = len(arr)
		case []float64:
			length = len(arr)
		default:
			return fmt.Errorf("array type is not supported: %T", info.Data)
		}
	case GdArrayTypeBool:
		arr, ok := info.Data.([]bool)
		if !ok {
			return fmt.Errorf("array type is not supported: %T", info.Data)
		}
		length = len(arr)
	case GdArrayTypeByte:
		arr, ok := info.Data.([]byte)
		if !ok {
			return fmt.Errorf("array type is not supported: %T", info.Data)
		}
		length = len(arr)
	case GdArrayTypeString:
		arr, ok := info.Data.([]string)
		if !ok {
			return fmt.Errorf("array type is not supported: %T", info.Data)
		}
		length = len(arr)
	default:
		return fmt.Errorf("array type is not supported: %d", info.Type)
	}
	if length > maxGdArrayElements || int64(length) != int64(info.Size) {
		return fmt.Errorf("array size does not match payload")
	}
	return nil
}

func serializeGdArray(info *GdArrayInfo) ([]byte, error) {
	if info == nil || info.Size < 0 || info.Size > maxGdArrayElements {
		return nil, fmt.Errorf("nil GdArrayInfo")
	}
	if err := validateGdArraySize(info); err != nil {
		return nil, err
	}

	dataBytes, err := serializeDataByType(info.Type, info.Data)
	if err != nil {
		return nil, err
	}

	if int64(len(dataBytes)) > int64(maxInt32)-8 {
		return nil, fmt.Errorf("serialized array is too large")
	}
	if len(dataBytes) > maxGdArrayBytes-8 {
		return nil, fmt.Errorf("serialized array is too large")
	}
	totalSize := 8 + len(dataBytes)
	result := make([]byte, totalSize)

	binary.LittleEndian.PutUint32(result[0:4], uint32(info.Size))
	binary.LittleEndian.PutUint32(result[4:8], uint32(info.Type))

	copy(result[8:], dataBytes)

	return result, nil
}

func deserializeGdArray(data []byte) (*GdArrayInfo, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data length is not enough")
	}
	if len(data) > maxGdArrayBytes {
		return nil, fmt.Errorf("array data is too large")
	}

	encodedSize := binary.LittleEndian.Uint32(data[0:4])
	if encodedSize > uint32(maxInt32) || encodedSize > maxGdArrayElements {
		return nil, fmt.Errorf("array size is invalid")
	}
	size := int32(encodedSize)
	arrayType := int32(binary.LittleEndian.Uint32(data[4:8]))

	arrayData, err := deserializeDataByType(arrayType, data[8:], size)
	if err != nil {
		return nil, err
	}

	return &GdArrayInfo{
		Size: size,
		Type: arrayType,
		Data: arrayData,
	}, nil
}

func f64Tof32(slice []float64) []float32 {
	if slice == nil {
		return []float32{}
	}
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func serializeDataByType(arrayType int32, data any) ([]byte, error) {
	if data == nil {
		return []byte{}, nil
	}

	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		arr, ok := data.([]int64)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		if len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeInt64Array(arr)
	case GdArrayTypeFloat:
		val, ok := data.([]float32)
		if !ok {
			slice, ok := data.([]float64)
			if !ok {
				return []byte{}, fmt.Errorf("array type is not supported: %T", data)
			}
			val = f64Tof32(slice)
		}
		if arr := val; len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeFloatArray(val)
	case GdArrayTypeBool:
		arr, ok := data.([]bool)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		if len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeBoolArray(arr)
	case GdArrayTypeByte:
		arr, ok := data.([]byte)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		if len(arr) == 0 {
			return []byte{}, nil
		}
		return arr, nil
	case GdArrayTypeString:
		arr, ok := data.([]string)
		if !ok {
			return nil, fmt.Errorf("array type is not supported: %T", data)
		}
		if len(arr) == 0 {
			return []byte{}, nil
		}
		return serializeStringArray(arr)
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

func deserializeDataByType(arrayType int32, data []byte, size int32) (any, error) {
	if size < 0 || size > maxGdArrayElements {
		return nil, fmt.Errorf("array size is invalid")
	}
	if len(data) > maxGdArrayBytes-8 {
		return nil, fmt.Errorf("array data is too large")
	}
	if len(data) == 0 || size == 0 {
		if size != 0 || len(data) != 0 {
			return nil, fmt.Errorf("array data length is not enough")
		}
		switch arrayType {
		case GdArrayTypeInt64, GdArrayTypeGdObj:
			return []int64{}, nil
		case GdArrayTypeFloat:
			return []float32{}, nil
		case GdArrayTypeBool:
			return []bool{}, nil
		case GdArrayTypeByte:
			return []byte{}, nil
		case GdArrayTypeString:
			return []string{}, nil
		default:
			return nil, fmt.Errorf("array type is not supported: %d", arrayType)
		}
	}

	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		return deserializeInt64Array(data, size)
	case GdArrayTypeFloat:
		return deserializeFloatArray(data, size)
	case GdArrayTypeBool:
		return deserializeBoolArray(data, size)
	case GdArrayTypeByte:
		if int64(len(data)) != int64(size) {
			return nil, fmt.Errorf("array data length is invalid")
		}
		return data, nil
	case GdArrayTypeString:
		return deserializeStringArray(data, size)
	default:
		return nil, fmt.Errorf("array type is not supported: %d", arrayType)
	}
}

func serializeInt64Array(data []int64) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	return bytesFromInt64Slice(data), nil
}

// Fixed-width array decoding retains the Web bridge's borrowed-storage semantics.
func deserializeInt64Array(data []byte, size int32) ([]int64, error) {
	if size < 0 {
		return nil, fmt.Errorf("array size is invalid")
	}
	requiredBytes := int64(size) * 8
	if int64(len(data)) != requiredBytes {
		return nil, fmt.Errorf("array data length is invalid")
	}
	if size == 0 {
		return []int64{}, nil
	}
	return unsafe.Slice((*int64)(unsafe.Pointer(unsafe.SliceData(data))), int(size)), nil
}

func serializeFloatArray(data []float32) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}
	return bytesFromFloat32Slice(data), nil
}

func deserializeFloatArray(data []byte, size int32) ([]float32, error) {
	if size < 0 {
		return nil, fmt.Errorf("array size is invalid")
	}
	requiredBytes := int64(size) * 4
	if int64(len(data)) != requiredBytes {
		return nil, fmt.Errorf("array data length is invalid")
	}
	if size == 0 {
		return []float32{}, nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(data))), int(size)), nil
}

func serializeBoolArray(data []bool) ([]byte, error) {
	result := make([]byte, len(data))
	for i, val := range data {
		if val {
			result[i] = 1
		} else {
			result[i] = 0
		}
	}
	return result, nil
}

func deserializeBoolArray(data []byte, size int32) ([]bool, error) {
	if size < 0 || int64(len(data)) != int64(size) {
		return nil, fmt.Errorf("array data length is invalid")
	}

	result := make([]bool, size)
	for i := 0; i < int(size); i++ {
		result[i] = data[i] != 0
	}
	return result, nil
}

func serializeStringArray(data []string) ([]byte, error) {
	if len(data) > maxGdArrayElements {
		return nil, fmt.Errorf("array size is invalid")
	}
	result := make([]byte, 0)
	for _, str := range data {
		strBytes := []byte(str)
		if uint64(len(strBytes)) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("string is too long")
		}
		if len(result) > maxGdArrayBytes-8-4-len(strBytes) {
			return nil, fmt.Errorf("serialized array is too large")
		}
		var lengthBytes [4]byte
		binary.LittleEndian.PutUint32(lengthBytes[:], uint32(len(strBytes)))

		result = append(result, lengthBytes[:]...)
		result = append(result, strBytes...)
	}
	return result, nil
}

func deserializeStringArray(data []byte, size int32) ([]string, error) {
	if size < 0 {
		return nil, fmt.Errorf("array size is invalid")
	}
	var result []string
	offset := 0

	for i := int32(0); i < size; i++ {
		if offset < 0 || offset > len(data) || len(data)-offset < 4 {
			return nil, fmt.Errorf("string data is not complete")
		}

		encodedLen := uint64(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4

		if encodedLen > uint64(len(data)-offset) {
			return nil, fmt.Errorf("string data is not complete")
		}
		strLen := int(encodedLen)

		str := string(data[offset : offset+strLen])
		result = append(result, str)
		offset += strLen
	}
	if offset != len(data) {
		return nil, fmt.Errorf("string data has trailing bytes")
	}

	return result, nil
}

// checkedGdArraySize keeps the length conversion and its allocation guard in
// one place. Callers must perform this check before copying/converting the
// backing slice (notably []float64 -> []float32).
func checkedGdArraySize(length int) (int32, bool) {
	if length < 0 || length > maxGdArrayElements || int64(length) > int64(maxInt32) {
		return 0, false
	}
	return int32(length), true
}

func bytesFromInt64Slice(data []int64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*8)
}

func bytesFromUint64Slice(data []uint64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*8)
}

func bytesFromFloat32Slice(data []float32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(data))), len(data)*4)
}

func fixedGdArrayElemSize(arrayType int32) (int, bool) {
	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj:
		return 8, true
	case GdArrayTypeFloat:
		return 4, true
	case GdArrayTypeBool, GdArrayTypeByte:
		return 1, true
	default:
		return 0, false
	}
}
