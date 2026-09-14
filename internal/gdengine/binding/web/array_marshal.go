//go:build js && wasm

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
	"math"
	"syscall/js"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

var jsUint8Array = js.Global().Get("Uint8Array")

func JsFromGdArray(array engine.Array) js.Value {
	return jsFromArray(array, 0)
}

// JsFromNativeArray preserves the declared ABI type, including signed object IDs.
func JsFromNativeArray(array engine.Array, arrayType int32) js.Value {
	return jsFromArray(array, arrayType)
}

// JsAllocNativeArray borrows output storage without copying or clearing its bytes.
func JsAllocNativeArray(arrayType int32, count int) js.Value {
	switch arrayType {
	case GdArrayTypeInt64, GdArrayTypeGdObj, GdArrayTypeFloat, GdArrayTypeByte:
	default:
		panic("unsupported native output array type")
	}
	n, ok := checkedArrayCount(count)
	if !ok {
		panic("invalid native output array count")
	}
	size, _ := arrayElementSize(arrayType)
	if count > maxGdArrayBytes/size {
		panic("native output array exceeds byte limit")
	}
	wrapper := borrowNativeArray(arrayType, n, count*size)
	if wrapper.IsNull() {
		panic("failed to borrow native output array")
	}
	return wrapper
}

func JsToGdArray(value js.Value) engine.Array {
	if value.Type() != js.TypeObject {
		return nil
	}
	tag := value.Get(arrayTag)
	if tag.Type() != js.TypeBoolean || !tag.Bool() {
		return nil
	}
	data := value.Get("data")
	if !isByteArray(data) {
		return nil
	}
	arrayType, typeOK := arrayInteger(value.Get("type"), GdArrayTypeGdObj)
	count, countOK := arrayInteger(value.Get("count"), maxGdArrayElements)
	if !typeOK || !countOK || validateArrayShape(arrayType, count, data.Length()) != nil {
		return nil
	}
	bytes := make([]byte, data.Length())
	js.CopyBytesToGo(bytes, data)
	array, err := decodeArrayData(arrayType, bytes, count)
	if err != nil {
		return nil
	}
	return array
}

// CopyNativeArrayOutput copies Wasm output directly into the caller's Go storage,
// avoiding the temporary byte slice and decoded result used by GdArray returns.
func CopyNativeArrayOutput(target engine.Array, wrapper js.Value) {
	switch target.(type) {
	case []int64, []uint64, []float32, []byte:
	default:
		panic("native array output requires a fixed-width Go slice")
	}
	array := describeArray(target)
	if array == nil || wrapper.Type() != js.TypeObject {
		panic("invalid native array output")
	}
	data, err := encodeArrayData(array.Type, array.Data)
	if err != nil {
		panic(err)
	}
	bytes := wrapper.Get("data")
	arrayType, typeOK := arrayInteger(wrapper.Get("type"), GdArrayTypeGdObj)
	if !isByteArray(bytes) || bytes.Length() != len(data) ||
		!typeOK || !nativeArrayTypeMatches(array.Type, arrayType) ||
		!wrapper.Get("count").Equal(js.ValueOf(array.Count)) {
		panic("native array output shape does not match caller storage")
	}
	js.CopyBytesToGo(data, bytes)
}

func jsFromArray(array engine.Array, arrayType int32) js.Value {
	if array == nil {
		panic("JsFromGdArray doesn't support nil array")
	}
	value := describeArray(array)
	if value == nil {
		return js.Null()
	}
	if arrayType != 0 {
		if !nativeArrayTypeMatches(value.Type, arrayType) {
			panic("native array element type does not match caller storage")
		}
		value.Type = arrayType
	}
	data, err := encodeArrayData(value.Type, value.Data)
	if err != nil {
		return js.Null()
	}
	wrapper := borrowNativeArray(value.Type, value.Count, len(data))
	if !wrapper.IsNull() {
		js.CopyBytesToJS(wrapper.Get("data"), data)
	}
	return wrapper
}

// Go stores object IDs as int64; both ABI types preserve the same 64 bits.
func nativeArrayTypeMatches(storageType, declaredType int32) bool {
	return storageType == declaredType || (storageType == GdArrayTypeInt64 && declaredType == GdArrayTypeGdObj)
}

func describeArray(array engine.Array) *arrayValue {
	switch data := array.(type) {
	case []int64:
		return newArrayValue(GdArrayTypeInt64, data)
	case []uint64:
		return newArrayValue(GdArrayTypeGdObj, data)
	case []float32:
		return newArrayValue(GdArrayTypeFloat, data)
	case []float64:
		return newArrayValue(GdArrayTypeFloat, data)
	case []bool:
		return newArrayValue(GdArrayTypeBool, data)
	case []string:
		return newArrayValue(GdArrayTypeString, data)
	case []byte:
		return newArrayValue(GdArrayTypeByte, data)
	default:
		return nil
	}
}

func isByteArray(value js.Value) bool {
	return value.Type() == js.TypeObject && value.InstanceOf(jsUint8Array)
}

func arrayInteger(value js.Value, maximum int32) (int32, bool) {
	if value.Type() != js.TypeNumber {
		return 0, false
	}
	n := value.Float()
	if !(n >= 0 && n <= float64(maximum) && math.Trunc(n) == n) {
		return 0, false
	}
	return int32(n), true
}

func borrowNativeArray(arrayType, count int32, byteLength int) js.Value {
	borrow := js.Global().Get("GdspxBorrowNativeArray")
	if borrow.Type() != js.TypeFunction {
		return js.Null()
	}
	wrapper := borrow.Invoke(arrayType, count, byteLength)
	if wrapper.Type() != js.TypeObject {
		return js.Null()
	}
	bytes := wrapper.Get("data")
	if !isByteArray(bytes) || bytes.Length() != byteLength ||
		!wrapper.Get(arrayTag).Equal(js.ValueOf(true)) ||
		!wrapper.Get("type").Equal(js.ValueOf(arrayType)) ||
		!wrapper.Get("count").Equal(js.ValueOf(count)) {
		return js.Null()
	}
	return wrapper
}
