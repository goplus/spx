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
	if array == nil {
		panic("JsFromGdArray doesn't support nil array")
	}
	value := describeArray(array)
	if value == nil {
		return js.Null()
	}
	data, err := encodeArrayData(value.Type, value.Data)
	if err != nil {
		return js.Null()
	}
	borrow := js.Global().Get("GdspxBorrowNativeArray")
	if borrow.Type() != js.TypeFunction {
		return js.Null()
	}
	wrapper := borrow.Invoke(value.Type, value.Count, len(data))
	if wrapper.Type() != js.TypeObject {
		return js.Null()
	}
	bytes := wrapper.Get("data")
	if !isByteArray(bytes) || bytes.Length() != len(data) {
		return js.Null()
	}
	js.CopyBytesToJS(bytes, data)
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
