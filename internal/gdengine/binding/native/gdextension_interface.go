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
#include <stdlib.h>

static inline GdArray createArrayInfo(int type, int size){
	if (size < 0) {
		return NULL;
	}
	GdArray array = (GdArray)malloc(sizeof(GdArrayInfo));
	if (!array) {
		return NULL;
	}
	array->size = size;
	array->type = type;

	if (size == 0) {
		array->data = NULL;
		return array;
	}

	size_t element_size = 0;
	switch (type) {
		case GD_ARRAY_TYPE_INT64 :
			element_size = sizeof(int64_t);
			break;
		case GD_ARRAY_TYPE_FLOAT :
			element_size = sizeof(float);
			break;
		case GD_ARRAY_TYPE_BOOL :
			element_size = sizeof(uint8_t); // Store as uint8_t for alignment
			break;
		case GD_ARRAY_TYPE_STRING :
			element_size = sizeof(char*);
			break;
		case GD_ARRAY_TYPE_BYTE :
			element_size = sizeof(uint8_t);
			break;
		case GD_ARRAY_TYPE_GDOBJ :
			element_size = sizeof(GdObj);
			break;
		default:
			free(array);
			return NULL;
	}

	array->data = malloc(size * element_size);
	if (!array->data && size > 0) {
		free(array);
		return NULL;
	}

	return array;
}
static inline void freeArrayInfo(GdArray arrayInfo) {
    if (arrayInfo == NULL) return;
    if (arrayInfo->type == 4  && arrayInfo->data != NULL) {//ARRAY_TYPE_STRING
        char** stringData = (char**)arrayInfo->data;
        for (int i = 0; i < arrayInfo->size; i++) {
            free(stringData[i]);
        }
    }
	if (arrayInfo->data != NULL) {
		free(arrayInfo->data);
	}
    free(arrayInfo);
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type Uint64T C.uint64_t

type Uint32T C.uint32_t

type Uint16T C.uint16_t

type Uint8T C.uint8_t

type Int32T C.int32_t

type Int16T C.int16_t

type Int8T C.int8_t

type Char C.char

type WcharT C.wchar_t

type Char32T C.char32_t

type Char16T C.char16_t

type GdString C.GdString

type GdInt C.GdInt

type GdBool C.GdBool

type GdFloat C.GdFloat

type GdVec4 C.GdVec4

type GdVec3 C.GdVec3

type GdVec2 C.GdVec2

type GdColor C.GdColor

type GdRect2 C.GdRect2

type GdObj C.GdObj

type GdArray C.GdArray

// Array type constants
const (
	ArayTypeUnknown = 0
	ArrayTypeInt64  = 1
	ArrayTypeFloat  = 2
	ArrayTypeBool   = 3
	ArrayTypeString = 4
	ArrayTypeByte   = 5
	ArrayTypeGdObj  = 6
)

// IArrayInfo interface for unified array operations
type IArrayInfo interface {
	Size() int64
	Type() int64
	ToInt64s() []int64
	ToFloats() []float32
	ToBools() []bool
	ToBytes() []byte
	ToObjects() []GdObj
	ToStrings() []string
	Free()
}

// Go wrapper for GdArray
type ArrayInfoImpl struct {
	gdArray   C.GdArray
	needsFree bool
}

type GDExtensionSpxCallbackInfoPtr C.GDExtensionSpxCallbackInfoPtr

type SpxCallbackInfo C.SpxCallbackInfo

type GDExtensionVariantPtr C.GDExtensionVariantPtr

type GDExtensionConstVariantPtr C.GDExtensionConstVariantPtr

type GDExtensionUninitializedVariantPtr C.GDExtensionUninitializedVariantPtr

type GDExtensionStringNamePtr C.GDExtensionStringNamePtr

type GDExtensionConstStringNamePtr C.GDExtensionConstStringNamePtr

type GDExtensionUninitializedStringNamePtr C.GDExtensionUninitializedStringNamePtr

type GDExtensionStringPtr C.GDExtensionStringPtr

type GDExtensionConstStringPtr C.GDExtensionConstStringPtr

type GDExtensionUninitializedStringPtr C.GDExtensionUninitializedStringPtr

type GDExtensionObjectPtr C.GDExtensionObjectPtr

type GDExtensionConstObjectPtr C.GDExtensionConstObjectPtr

type GDExtensionUninitializedObjectPtr C.GDExtensionUninitializedObjectPtr

type GDExtensionTypePtr C.GDExtensionTypePtr

type GDExtensionConstTypePtr C.GDExtensionConstTypePtr

type GDExtensionUninitializedTypePtr C.GDExtensionUninitializedTypePtr

type GDExtensionMethodBindPtr C.GDExtensionMethodBindPtr

type GDExtensionInt C.GDExtensionInt

type GDExtensionBool C.GDExtensionBool

type GDObjectInstanceID C.GDObjectInstanceID

type GDExtensionRefPtr C.GDExtensionRefPtr

type GDExtensionConstRefPtr C.GDExtensionConstRefPtr

type GDExtensionPtrConstructor C.GDExtensionPtrConstructor

type GDExtensionPtrDestructor C.GDExtensionPtrDestructor

type GDExtensionVariantType C.GDExtensionVariantType

const (
	GDEXTENSION_VARIANT_TYPE_NIL GDExtensionVariantType = iota
	GDEXTENSION_VARIANT_TYPE_BOOL
	GDEXTENSION_VARIANT_TYPE_INT
	GDEXTENSION_VARIANT_TYPE_FLOAT
	GDEXTENSION_VARIANT_TYPE_STRING
	GDEXTENSION_VARIANT_TYPE_VECTOR2
	GDEXTENSION_VARIANT_TYPE_VECTOR2I
	GDEXTENSION_VARIANT_TYPE_RECT2
	GDEXTENSION_VARIANT_TYPE_RECT2I
	GDEXTENSION_VARIANT_TYPE_VECTOR3
	GDEXTENSION_VARIANT_TYPE_VECTOR3I
	GDEXTENSION_VARIANT_TYPE_TRANSFORM2D
	GDEXTENSION_VARIANT_TYPE_VECTOR4
	GDEXTENSION_VARIANT_TYPE_VECTOR4I
	GDEXTENSION_VARIANT_TYPE_PLANE
	GDEXTENSION_VARIANT_TYPE_QUATERNION
	GDEXTENSION_VARIANT_TYPE_AABB
	GDEXTENSION_VARIANT_TYPE_BASIS
	GDEXTENSION_VARIANT_TYPE_TRANSFORM3D
	GDEXTENSION_VARIANT_TYPE_PROJECTION
	GDEXTENSION_VARIANT_TYPE_COLOR
	GDEXTENSION_VARIANT_TYPE_STRING_NAME
	GDEXTENSION_VARIANT_TYPE_NODE_PATH
	GDEXTENSION_VARIANT_TYPE_RID
	GDEXTENSION_VARIANT_TYPE_OBJECT
	GDEXTENSION_VARIANT_TYPE_CALLABLE
	GDEXTENSION_VARIANT_TYPE_SIGNAL
	GDEXTENSION_VARIANT_TYPE_DICTIONARY
	GDEXTENSION_VARIANT_TYPE_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_BYTE_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_INT32_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_INT64_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_FLOAT32_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_FLOAT64_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_STRING_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_VECTOR2_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_VECTOR3_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_COLOR_ARRAY
	GDEXTENSION_VARIANT_TYPE_PACKED_VECTOR4_ARRAY
	GDEXTENSION_VARIANT_TYPE_VARIANT_MAX
)

type GDExtensionInitializationLevel int64

const (
	GDExtensionInitializationLevelCore    GDExtensionInitializationLevel = 0
	GDExtensionInitializationLevelServers GDExtensionInitializationLevel = 1
	GDExtensionInitializationLevelScene   GDExtensionInitializationLevel = 2
	GDExtensionInitializationLevelEditor  GDExtensionInitializationLevel = 3
)

type initialization = C.GDExtensionInitialization

type initializationLevel = C.GDExtensionInitializationLevel

func ToGdBool(val bool) GdBool {
	if val {
		return GdBool(1)
	} else {
		return GdBool(0)
	}
}

func ToBool(val GdBool) bool {
	return val != 0
}

func ToGdVec2(val mathf.Vec2) GdVec2 {
	return GdVec2{C.GdFloat(val.X), C.GdFloat(val.Y)}
}

func ToVec2(val GdVec2) mathf.Vec2 {
	return mathf.NewVec2(float64(val.X), float64(val.Y))
}

func ToGdVec4(val mathf.Vec4) GdVec4 {
	return GdVec4{C.GdFloat(val.X), C.GdFloat(val.Y), C.GdFloat(val.Z), C.GdFloat(val.W)}
}

func ToVec4(val GdVec4) mathf.Vec4 {
	return mathf.NewVec4(float64(val.X), float64(val.Y), float64(val.Z), float64(val.W))
}

func ToGdColor(val mathf.Color) GdColor {
	return GdColor{C.float(val.R), C.float(val.G), C.float(val.B), C.float(val.A)}
}

func ToColor(val GdColor) mathf.Color {
	return mathf.NewColor(float64(val.R), float64(val.G), float64(val.B), float64(val.A))
}

func ToGdRect2(val mathf.Rect2) GdRect2 {
	position := ToGdVec2(val.Position)
	size := ToGdVec2(val.Size)
	ret := GdRect2{}
	ret.Position = C.GdVec2(position)
	ret.Size = C.GdVec2(size)
	return ret
}

func ToRect2(val GdRect2) mathf.Rect2 {
	ret := mathf.Rect2{}
	ret.Position = ToVec2(GdVec2(val.Position))
	ret.Size = ToVec2(GdVec2(val.Size))
	return ret
}

func ToGdObj(val engine.Object) GdObj {
	return GdObj(val)
}

func ToObject(val GdObj) engine.Object {
	return engine.Object(val)
}

func ToGdInt(val int64) GdInt {
	return GdInt(val)
}

func ToInt(val GdInt) int64 {
	return int64(val)
}

func ToInt64(val GdInt) int64 {
	return int64(val)
}

func ToGdFloat(val float64) GdFloat {
	return GdFloat(val)
}

func ToFloat64(val GdFloat) float64 {
	return float64(val)
}

func ToFloat32(val GdFloat) float64 {
	return float64(val)
}

func ToFloat(val GdFloat) float64 {
	return float64(val)
}

// ToString copies an owned ABI return and releases it through the runtime allocator.
func ToString(val GdString) string {
	if val == nil {
		return ""
	}
	str := copyString(val)
	freeReturnedString(val)
	return str
}

func GDExtensionInterfaceObjectMethodBindPtrcall(
	p_method_bind GDExtensionMethodBindPtr,
	p_instance GDExtensionObjectPtr,
	p_args *GDExtensionConstTypePtr,
	r_ret GDExtensionTypePtr,
) {
}

func ToGdArrayInfo(slice interface{}) *ArrayInfoImpl {
	var info *ArrayInfoImpl = nil
	switch v := slice.(type) {
	case []int64:
		info = createGdArrayFromInt64s(v)
	case []float32:
		info = createGdArrayFromFloats(v)
	case []float64:
		if v == nil {
			info = createGdArrayFromFloats(nil)
			break
		}
		floats := make([]float32, len(v))
		for i, f := range v {
			floats[i] = float32(f)
		}
		info = createGdArrayFromFloats(floats)
	case []bool:
		info = createGdArrayFromBools(v)
	case []string:
		info = createGdArrayFromStrings(v)
	case []GdObj:
		info = createGdArrayFromObjects(v)
	case []byte:
		info = createGdArrayFromBytes(v)
	default:
		panic(fmt.Sprintf("unsupported array type: %T", slice))
	}
	return info
}

func ToGdArray(slice interface{}) GdArray {
	info := ToGdArrayInfo(slice)
	if info == nil {
		return nil
	}
	return GdArray(info.gdArray)
}

// Conversion functions
func ToArray(arrayInfo GdArray) any {
	if arrayInfo == nil {
		return nil
	}
	// Returned GdArray values own a temporary bridge buffer and must be freed
	// after converting them into Go slices.
	info := ArrayInfoImpl{gdArray: C.GdArray(arrayInfo), needsFree: true}
	defer info.Free()
	switch info.Type() {
	case ArrayTypeInt64:
		return info.ToInt64s()
	case ArrayTypeFloat:
		return info.ToFloats()
	case ArrayTypeBool:
		return info.ToBools()
	case ArrayTypeString:
		return info.ToStrings()
	case ArrayTypeGdObj:
		return info.ToObjects()
	case ArrayTypeByte:
		return info.ToBytes()
	default:
		return nil
	}
}

func (a *ArrayInfoImpl) Raw() GdArray {
	if a == nil {
		return nil
	}
	return GdArray(a.gdArray)
}

// GdArray implementation

// ArrayInfoImpl methods
func (a *ArrayInfoImpl) Size() int64 {
	if a.gdArray == nil {
		return 0
	}
	return int64(a.gdArray.size)
}

func (a *ArrayInfoImpl) Type() int64 {
	if a.gdArray == nil {
		return 0
	}
	// In Go, we need to use a different way to access reserved words in C structs
	return int64(a.gdArray._type)
}

func (a *ArrayInfoImpl) Free() {
	if a.gdArray != nil && a.needsFree {
		C.freeArrayInfo(a.gdArray)
		a.gdArray = nil
		a.needsFree = false
	}
}

func (a *ArrayInfoImpl) ToInt64s() []int64 {
	if a.gdArray == nil || a.Type() != ArrayTypeInt64 {
		return nil
	}
	size := a.Size()
	if size == 0 {
		return []int64{}
	}
	slice := (*[1 << 27]C.int64_t)(unsafe.Pointer(a.gdArray.data))[:size:size]
	result := make([]int64, size)
	for i, v := range slice {
		result[i] = int64(v)
	}
	return result
}

func (a *ArrayInfoImpl) ToFloats() []float32 {
	if a.gdArray == nil || a.Type() != ArrayTypeFloat {
		return nil
	}
	size := a.Size()
	if size == 0 {
		return []float32{}
	}
	slice := (*[1 << 27]C.float)(unsafe.Pointer(a.gdArray.data))[:size:size]
	result := make([]float32, size)
	for i, v := range slice {
		result[i] = float32(v)
	}
	return result
}

func (a *ArrayInfoImpl) ToBools() []bool {
	if a.gdArray == nil || a.Type() != ArrayTypeBool {
		return nil
	}
	size := a.Size()
	if size == 0 {
		return []bool{}
	}
	slice := (*[1 << 27]C.uint8_t)(unsafe.Pointer(a.gdArray.data))[:size:size]
	result := make([]bool, size)
	for i, v := range slice {
		result[i] = int64(v) != 0
	}
	return result
}

func (a *ArrayInfoImpl) ToBytes() []byte {
	if a.gdArray == nil || a.Type() != ArrayTypeByte {
		return nil
	}
	size := a.Size()
	if size == 0 {
		return []byte{}
	}
	slice := (*[1 << 27]C.uchar)(unsafe.Pointer(a.gdArray.data))[:size:size]
	result := make([]byte, size)
	for i, v := range slice {
		result[i] = byte(v)
	}
	return result
}

func (a *ArrayInfoImpl) ToObjects() []int64 {
	if a.gdArray == nil || a.Type() != ArrayTypeGdObj {
		return nil
	}
	size := a.Size()
	if size == 0 {
		return []int64{}
	}
	slice := (*[1 << 27]C.GdObj)(unsafe.Pointer(a.gdArray.data))[:size:size]
	result := make([]int64, size)
	for i, v := range slice {
		result[i] = int64(v)
	}
	return result
}

func (a *ArrayInfoImpl) ToStrings() []string {
	if a.gdArray == nil || a.Type() != ArrayTypeString {
		return nil
	}
	size := a.Size()
	if size == 0 {
		return []string{}
	}
	slice := (*[1 << 27]*C.char)(unsafe.Pointer(a.gdArray.data))[:size:size]
	result := make([]string, size)
	for i, cStr := range slice {
		result[i] = C.GoString(cStr)
	}
	return result
}

// copyString copies a borrowed string without releasing the caller's memory.
func copyString(val GdString) string {
	if val == nil {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(val)))
}

func doInitialization(init *initialization) {
	stringInitConstructorBindings()
	C.initialization(init)
}

func getProcAddress(handle uintptr, name string) unsafe.Pointer {
	name = name + "\000"
	char := C.CString(name)
	defer C.free(unsafe.Pointer(char))
	return C.get_proc_address(C.pointer(handle), char)
}

func registerEngineCallback() {
	spx_global_register_callbacks := resolveCFunc("spx_global_register_callbacks")
	C.spx_global_register_callbacks(
		C.pointer(uintptr(spx_global_register_callbacks)),
	)
}

//export initialize
func initialize(_ unsafe.Pointer, level initializationLevel) {
	if level == 2 {
		main()
	}
}

//export deinitialize
func deinitialize(_ unsafe.Pointer, level initializationLevel) {
	if level == 0 {
	}
}

func allocateGdArrayInfo(arrayType int64, size int, isNil bool) *ArrayInfoImpl {
	if isNil {
		return &ArrayInfoImpl{gdArray: nil, needsFree: false}
	}
	arrayInfo := C.createArrayInfo(C.int(arrayType), C.int(size))
	if arrayInfo == nil {
		return nil
	}
	return &ArrayInfoImpl{gdArray: arrayInfo, needsFree: true}
}

func createGdArrayFromInt64s(ints []int64) *ArrayInfoImpl {
	info := allocateGdArrayInfo(ArrayTypeInt64, len(ints), ints == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	if len(ints) > 0 {
		cIntSlice := (*[1 << 27]C.int64_t)(unsafe.Pointer(arrayInfo.data))[:len(ints):len(ints)]
		for i, v := range ints {
			cIntSlice[i] = C.int64_t(v)
		}
	}
	return info
}

func createGdArrayFromFloats(floats []float32) *ArrayInfoImpl {
	info := allocateGdArrayInfo(ArrayTypeFloat, len(floats), floats == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	if len(floats) > 0 {
		cFloatSlice := (*[1 << 27]C.float)(unsafe.Pointer(arrayInfo.data))[:len(floats):len(floats)]
		for i, v := range floats {
			cFloatSlice[i] = C.float(v)
		}
	}
	return info
}

func createGdArrayFromBools(bools []bool) *ArrayInfoImpl {
	info := allocateGdArrayInfo(ArrayTypeBool, len(bools), bools == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	if len(bools) > 0 {
		cBoolSlice := (*[1 << 27]C.uint8_t)(unsafe.Pointer(arrayInfo.data))[:len(bools):len(bools)]
		for i, v := range bools {
			if v {
				cBoolSlice[i] = 1
			} else {
				cBoolSlice[i] = 0
			}
		}
	}
	return info
}

func createGdArrayFromBytes(bytes []byte) *ArrayInfoImpl {
	info := allocateGdArrayInfo(ArrayTypeByte, len(bytes), bytes == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	if len(bytes) > 0 {
		cByteSlice := (*[1 << 27]C.uchar)(unsafe.Pointer(arrayInfo.data))[:len(bytes):len(bytes)]
		for i, v := range bytes {
			cByteSlice[i] = C.uchar(v)
		}
	}
	return info
}

func createGdArrayFromObjects(objects []GdObj) *ArrayInfoImpl {
	info := allocateGdArrayInfo(ArrayTypeGdObj, len(objects), objects == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	if len(objects) > 0 {
		cObjSlice := (*[1 << 27]C.GdObj)(unsafe.Pointer(arrayInfo.data))[:len(objects):len(objects)]
		for i, v := range objects {
			cObjSlice[i] = C.GdObj(v)
		}
	}
	return info
}

func createGdArrayFromStrings(strings []string) *ArrayInfoImpl {
	info := allocateGdArrayInfo(ArrayTypeString, len(strings), strings == nil)
	if info == nil || info.gdArray == nil {
		return info
	}
	arrayInfo := info.gdArray
	if len(strings) > 0 {
		cStrSlice := (*[1 << 27]*C.char)(unsafe.Pointer(arrayInfo.data))[:len(strings):len(strings)]
		for i, v := range strings {
			cStr := C.CString(v)
			cStrSlice[i] = cStr
		}
	}
	return info
}
