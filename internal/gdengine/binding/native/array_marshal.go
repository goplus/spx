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
		return createNumericArray[C.int64_t](ArrayTypeInt64, v)
	case []float32:
		return createNumericArray[C.float](ArrayTypeFloat, v)
	case []float64:
		return createNumericArray[C.float](ArrayTypeFloat, v)
	case []bool:
		return createGdArrayFromBools(v)
	case []string:
		return createGdArrayFromStrings(v)
	case []GdObj:
		return createNumericArray[C.GdObj](ArrayTypeGdObj, v)
	case []byte:
		return createNumericArray[C.uchar](ArrayTypeByte, v)
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
	return readNumericArray[C.int64_t, int64](a, ArrayTypeInt64)
}

func (a *ArrayInfo) ToFloat32s() []float32 {
	return readNumericArray[C.float, float32](a, ArrayTypeFloat)
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
	return readNumericArray[C.uchar, byte](a, ArrayTypeByte)
}

func (a *ArrayInfo) ToObjectIDs() []int64 {
	return readNumericArray[C.GdObj, int64](a, ArrayTypeGdObj)
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

type arrayNumber interface {
	~int64 | ~float32 | ~float64 | ~uint8
}

// Keep the ABI element type explicit while copying between native and Go storage.
func createNumericArray[Native, Go arrayNumber](arrayType int64, values []Go) *ArrayInfo {
	info := allocateGdArrayInfo(arrayType, len(values), values == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	data := unsafe.Slice((*Native)(info.gdArray.data), len(values))
	for i, value := range values {
		data[i] = Native(value)
	}
	return info
}

func readNumericArray[Native, Go arrayNumber](info *ArrayInfo, arrayType int64) []Go {
	if info.gdArray == nil || info.Type() != arrayType {
		return nil
	}
	data := unsafe.Slice((*Native)(info.gdArray.data), info.Size())
	result := make([]Go, len(data))
	for i, value := range data {
		result[i] = Go(value)
	}
	return result
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
