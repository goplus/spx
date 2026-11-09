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

package ffi

/*
#include "gdextension_spx_interface.h"
#include <stdint.h>
#include <stdlib.h>

static inline GdArray cgo_create_array_info(int array_type, int size, int *size_overflow) {
	if (size < 0) {
		return NULL;
	}
	size_t element_size = 0;
	if (size > 0) {
		switch (array_type) {
			case GD_ARRAY_TYPE_INT64:
				element_size = sizeof(int64_t);
				break;
			case GD_ARRAY_TYPE_FLOAT:
				element_size = sizeof(float);
				break;
			case GD_ARRAY_TYPE_STRING:
				element_size = sizeof(char *);
				break;
			case GD_ARRAY_TYPE_BOOL:
			case GD_ARRAY_TYPE_BYTE:
				element_size = sizeof(uint8_t);
				break;
			case GD_ARRAY_TYPE_GDOBJ:
				element_size = sizeof(GdObj);
				break;
			default:
				return NULL;
		}

		if ((size_t)size > SIZE_MAX / element_size) {
			*size_overflow = 1;
			return NULL;
		}
	}

	GdArray array = (GdArray)malloc(sizeof(GdArrayInfo));
	if (!array) {
		return NULL;
	}
	array->size = size;
	array->type = array_type;

	if (size == 0) {
		array->data = NULL;
		return array;
	}

	array->data = malloc((size_t)size * element_size);
	if (!array->data) {
		free(array);
		return NULL;
	}

	return array;
}

static inline void cgo_free_array_info(GdArray array_info) {
	if (array_info == NULL) {
		return;
	}
	if (array_info->type == GD_ARRAY_TYPE_STRING && array_info->data != NULL) {
		char **strings = (char **)array_info->data;
		for (int i = 0; i < array_info->size; i++) {
			free(strings[i]);
		}
	}
	free(array_info->data);
	free(array_info);
}
*/
import "C"

import (
	"fmt"
	"math"
	"unsafe"
)

// Array type constants
const (
	ArrayTypeInt64  = C.GD_ARRAY_TYPE_INT64
	ArrayTypeFloat  = C.GD_ARRAY_TYPE_FLOAT
	ArrayTypeBool   = C.GD_ARRAY_TYPE_BOOL
	ArrayTypeString = C.GD_ARRAY_TYPE_STRING
	ArrayTypeByte   = C.GD_ARRAY_TYPE_BYTE
	ArrayTypeGdObj  = C.GD_ARRAY_TYPE_GDOBJ
)

// ArrayInfo owns a temporary native array buffer. Call Free after use.
type ArrayInfo struct {
	gdArray C.GdArray
}

func ToGdArrayInfo(slice any) *ArrayInfo {
	switch v := slice.(type) {
	case []int64:
		return createGdArrayFromInt64s(v)
	case []float32:
		return createGdArrayFromFloats(v)
	case []float64:
		return createGdArrayFromFloats(v)
	case []bool:
		return createGdArrayFromBools(v)
	case []string:
		return createGdArrayFromStrings(v)
	case []GdObj:
		return createGdArrayFromObjectIDs(v)
	case []byte:
		return createGdArrayFromBytes(v)
	default:
		panic(fmt.Sprintf("unsupported array type: %T", slice))
	}
}

// ToArray copies an owned ABI return into Go slices and releases its native buffer.
func ToArray(arrayInfo GdArray) any {
	if arrayInfo == nil {
		return nil
	}
	info := ArrayInfo{gdArray: C.GdArray(arrayInfo)}
	defer info.Free()
	switch info.Type() {
	case ArrayTypeInt64:
		return info.ToInt64s()
	case ArrayTypeFloat:
		return info.ToFloat32s()
	case ArrayTypeBool:
		return info.ToBools()
	case ArrayTypeString:
		return info.ToStrings()
	case ArrayTypeGdObj:
		return info.ToObjectIDs()
	case ArrayTypeByte:
		return info.ToBytes()
	default:
		return nil
	}
}

func (a *ArrayInfo) Raw() GdArray {
	if a == nil {
		return nil
	}
	return GdArray(a.gdArray)
}

func (a *ArrayInfo) Size() int64 {
	if a.gdArray == nil {
		return 0
	}
	return int64(a.gdArray.size)
}

func (a *ArrayInfo) Type() int64 {
	if a.gdArray == nil {
		return 0
	}
	// cgo exposes the C field "type" as "_type" because "type" is a Go keyword.
	return int64(a.gdArray._type)
}

func (a *ArrayInfo) Free() {
	if a.gdArray != nil {
		C.cgo_free_array_info(a.gdArray)
		a.gdArray = nil
	}
}

func (a *ArrayInfo) ToInt64s() []int64 {
	if a.gdArray == nil || a.Type() != ArrayTypeInt64 {
		return nil
	}
	size := a.Size()
	slice := unsafe.Slice((*C.int64_t)(a.gdArray.data), size)
	result := make([]int64, size)
	for i, v := range slice {
		result[i] = int64(v)
	}
	return result
}

func (a *ArrayInfo) ToFloat32s() []float32 {
	if a.gdArray == nil || a.Type() != ArrayTypeFloat {
		return nil
	}
	size := a.Size()
	slice := unsafe.Slice((*C.float)(a.gdArray.data), size)
	result := make([]float32, size)
	for i, v := range slice {
		result[i] = float32(v)
	}
	return result
}

func (a *ArrayInfo) ToBools() []bool {
	if a.gdArray == nil || a.Type() != ArrayTypeBool {
		return nil
	}
	size := a.Size()
	slice := unsafe.Slice((*C.uint8_t)(a.gdArray.data), size)
	result := make([]bool, size)
	for i, v := range slice {
		result[i] = v != 0
	}
	return result
}

func (a *ArrayInfo) ToBytes() []byte {
	if a.gdArray == nil || a.Type() != ArrayTypeByte {
		return nil
	}
	size := a.Size()
	slice := unsafe.Slice((*C.uchar)(a.gdArray.data), size)
	result := make([]byte, size)
	for i, v := range slice {
		result[i] = byte(v)
	}
	return result
}

func (a *ArrayInfo) ToObjectIDs() []int64 {
	if a.gdArray == nil || a.Type() != ArrayTypeGdObj {
		return nil
	}
	size := a.Size()
	slice := unsafe.Slice((*C.GdObj)(a.gdArray.data), size)
	result := make([]int64, size)
	for i, v := range slice {
		result[i] = int64(v)
	}
	return result
}

func (a *ArrayInfo) ToStrings() []string {
	if a.gdArray == nil || a.Type() != ArrayTypeString {
		return nil
	}
	size := a.Size()
	slice := unsafe.Slice((**C.char)(a.gdArray.data), size)
	result := make([]string, size)
	for i, cStr := range slice {
		result[i] = C.GoString(cStr)
	}
	return result
}

func checkNativeArrayLength(size int) {
	if size > math.MaxInt32 {
		panic("gdengine: native array length exceeds int32")
	}
}

func allocateGdArrayInfo(arrayType int64, size int, isNil bool) *ArrayInfo {
	if isNil {
		return &ArrayInfo{}
	}
	// GdArrayInfo.size is int32_t. Check before narrowing the Go length.
	if size < 0 {
		return nil
	}
	checkNativeArrayLength(size)
	var sizeOverflow C.int
	arrayInfo := C.cgo_create_array_info(C.int(arrayType), C.int(size), &sizeOverflow)
	if sizeOverflow != 0 {
		panic("gdengine: native array byte size overflows size_t")
	}
	if arrayInfo == nil {
		return nil
	}
	return &ArrayInfo{gdArray: arrayInfo}
}

func createGdArrayFromInt64s(ints []int64) *ArrayInfo {
	info := allocateGdArrayInfo(ArrayTypeInt64, len(ints), ints == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	cIntSlice := unsafe.Slice((*C.int64_t)(arrayInfo.data), len(ints))
	for i, v := range ints {
		cIntSlice[i] = C.int64_t(v)
	}
	return info
}

func createGdArrayFromFloats[T ~float32 | ~float64](floats []T) *ArrayInfo {
	info := allocateGdArrayInfo(ArrayTypeFloat, len(floats), floats == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	cFloatSlice := unsafe.Slice((*C.float)(arrayInfo.data), len(floats))
	for i, v := range floats {
		cFloatSlice[i] = C.float(v)
	}
	return info
}

func createGdArrayFromBools(bools []bool) *ArrayInfo {
	info := allocateGdArrayInfo(ArrayTypeBool, len(bools), bools == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	cBoolSlice := unsafe.Slice((*C.uint8_t)(arrayInfo.data), len(bools))
	for i, v := range bools {
		if v {
			cBoolSlice[i] = 1
		} else {
			cBoolSlice[i] = 0
		}
	}
	return info
}

func createGdArrayFromBytes(bytes []byte) *ArrayInfo {
	info := allocateGdArrayInfo(ArrayTypeByte, len(bytes), bytes == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	cByteSlice := unsafe.Slice((*C.uchar)(arrayInfo.data), len(bytes))
	for i, v := range bytes {
		cByteSlice[i] = C.uchar(v)
	}
	return info
}

func createGdArrayFromObjectIDs(objects []GdObj) *ArrayInfo {
	info := allocateGdArrayInfo(ArrayTypeGdObj, len(objects), objects == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	cObjSlice := unsafe.Slice((*C.GdObj)(arrayInfo.data), len(objects))
	for i, v := range objects {
		cObjSlice[i] = C.GdObj(v)
	}
	return info
}

func createGdArrayFromStrings(strings []string) *ArrayInfo {
	info := allocateGdArrayInfo(ArrayTypeString, len(strings), strings == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	cStrSlice := unsafe.Slice((**C.char)(arrayInfo.data), len(strings))
	for i, v := range strings {
		cStr := C.CString(v)
		cStrSlice[i] = cStr
	}
	return info
}
