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
	"encoding/json"
	"math"
	"strings"
	"syscall/js"

	. "github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v3/internal/log"
	. "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

const (
	maxInt = int(^uint(0) >> 1)
	// JS numbers are exact only through Number.MAX_SAFE_INTEGER.
	maxSafeJSInt = 1<<53 - 1

	fastGdArrayFlagKey  = "__gdspx_fast_array"
	fastGdArrayTypeKey  = "type"
	fastGdArrayCountKey = "count"
	fastGdArrayDataKey  = "data"
)

var (
	jsObject     = js.Global().Get("Object")
	jsUint8Array = js.Global().Get("Uint8Array")

	fastGdArrayScratchByType = map[int32]*fastGdArrayScratch{}
)

type fastGdArrayScratch struct {
	bytes   js.Value
	wrapper js.Value
	byteCap int
}

func PrintJs(rect js.Value) {
	rectMap := jsValue2Go(rect)
	jsonData, err := json.Marshal(rectMap)
	if err != nil {
		spxlog.Error("Error converting to JSON: %v", err)
		return
	}
	spxlog.Debug("%s", string(jsonData))
}

func JsFromGdObj(val Object) js.Value {
	return JsFromGdInt(int64(val))
}

func JsSplitGdObj(val Object) (uint32, uint32) {
	return JsSplitGdInt(int64(val))
}

// JsSplitGdInt encodes a web bridge GdInt/GdObj as (low, high) uint32 parts.
// JS-facing wrappers keep this order, while the wasm-side constructors rebuild
// the 64-bit value according to the platform ABI.
func JsSplitGdInt(val int64) (uint32, uint32) {
	low := uint32(val & 0xFFFFFFFF)
	high := uint32((val >> 32) & 0xFFFFFFFF)
	return low, high
}

func JsFromGdInt(val int64) js.Value {
	intJs := jsObject.New()
	low, high := JsSplitGdInt(val)
	intJs.Set("low", low)
	intJs.Set("high", high)
	return intJs
}

func JsToGdObject(val js.Value) Object {
	return Object(JsToGdInt(val))
}

func JsToGdObj(val js.Value) int64 {
	return JsToGdInt(val)
}

func JsToGdInt(val js.Value) int64 {
	low := uint32(val.Get("low").Int())
	high := uint32(val.Get("high").Int())
	return gdIntFromParts(low, high)
}

func JsFromGdString(object string) js.Value {
	return js.ValueOf(object)
}

func JsFromGdVec2(vec Vec2) js.Value {
	vec2Js := jsObject.New()
	vec2Js.Set("x", float32(vec.X))
	vec2Js.Set("y", float32(vec.Y))
	return vec2Js
}

func JsFromGdVec3(vec Vec3) js.Value {
	vec3Js := jsObject.New()
	vec3Js.Set("x", float32(vec.X))
	vec3Js.Set("y", float32(vec.Y))
	vec3Js.Set("z", float32(vec.Z))
	return vec3Js
}

func JsFromGdVec4(vec Vec4) js.Value {
	vec4Js := jsObject.New()
	vec4Js.Set("x", float32(vec.X))
	vec4Js.Set("y", float32(vec.Y))
	vec4Js.Set("z", float32(vec.Z))
	vec4Js.Set("w", float32(vec.W))
	return vec4Js
}

func JsFromGdColor(color Color) js.Value {
	colorJs := jsObject.New()
	colorJs.Set("r", float32(color.R))
	colorJs.Set("g", float32(color.G))
	colorJs.Set("b", float32(color.B))
	colorJs.Set("a", float32(color.A))
	return colorJs
}

func JsFromGdRect2(rect Rect2) js.Value {
	rectJs := jsObject.New()
	rectJs.Set("position", JsFromGdVec2(rect.Position))
	rectJs.Set("size", JsFromGdVec2(rect.Size))
	return rectJs
}

func JsFromGdBool(val bool) js.Value {
	return js.ValueOf(val)
}

func JsFromGdFloat(val float64) js.Value {
	return js.ValueOf(float32(val))
}

func JsToGdString(object js.Value) string {
	s := object.String()
	// Strip null terminators from C/FFI layer to ensure proper string comparison
	return strings.TrimRight(s, "\x00")
}

func JsToGdVec2(vec js.Value) Vec2 {
	return Vec2{
		X: float64(vec.Get("x").Float()),
		Y: float64(vec.Get("y").Float()),
	}
}

func JsToGdVec3(vec js.Value) Vec3 {
	return Vec3{
		X: float64(vec.Get("x").Float()),
		Y: float64(vec.Get("y").Float()),
		Z: float64(vec.Get("z").Float()),
	}
}

func JsToGdVec4(vec js.Value) Vec4 {
	return Vec4{
		X: float64(vec.Get("x").Float()),
		Y: float64(vec.Get("y").Float()),
		Z: float64(vec.Get("z").Float()),
		W: float64(vec.Get("w").Float()),
	}
}

func JsToGdColor(color js.Value) Color {
	return Color{
		R: float64(color.Get("r").Float()),
		G: float64(color.Get("g").Float()),
		B: float64(color.Get("b").Float()),
		A: float64(color.Get("a").Float()),
	}
}

func JsToGdRect2(rect js.Value) Rect2 {
	return Rect2{
		Position: JsToGdVec2(rect.Get("position")),
		Size:     JsToGdVec2(rect.Get("size")),
	}
}

func JsToGdBool(val js.Value) bool {
	switch val.Type() {
	case js.TypeNumber:
		return val.Int() != 0
	case js.TypeBoolean:
		return val.Bool()
	default:
		panic("unknow type")
	}
}

func JsToGdFloat(val js.Value) float64 {
	return float64(val.Float())
}

func JsToGdFloat32(val js.Value) float32 {
	return float32(val.Float())
}

func JsToGdInt64(val js.Value) int64 {
	return int64(val.Int())
}

func JsFromGdArray(arrayPtr Array) js.Value {
	if arrayPtr == nil {
		panic("JsFromGdArray doesn't support nil array")
	}

	if fastValue, ok := arrayToFastGdArrayValue(arrayPtr); ok {
		return fastValue
	}

	info := arrayToGdArrayInfo(arrayPtr)
	if info == nil {
		return js.ValueOf(nil)
	}

	serializedBytes, err := serializeGdArray(info)
	if err != nil {
		return js.ValueOf(nil)
	}

	jsBytes := jsUint8Array.New(len(serializedBytes))
	js.CopyBytesToJS(jsBytes, serializedBytes)
	return jsBytes
}

func JsToGdArray(val js.Value) Array {
	if val.IsNull() || val.IsUndefined() {
		return nil
	}

	if val.Type() != js.TypeObject {
		return nil
	}

	if fastArray, ok := fastGdArrayToGo(val); ok {
		return fastArray
	}

	lengthVal := val.Get("length")
	if lengthVal.Type() != js.TypeNumber {
		return nil
	}
	lengthFloat := lengthVal.Float()
	if !isSafeNonnegativeJSInt(lengthFloat) || lengthFloat < 8 ||
		lengthFloat > float64(maxInt) || lengthFloat > maxGdArrayBytes {
		return nil
	}
	if !val.InstanceOf(jsUint8Array) {
		return nil
	}
	length := int(lengthFloat)
	if length == 0 {
		return nil
	}

	serializedBytes := make([]byte, length)
	js.CopyBytesToGo(serializedBytes, val)

	info, err := deserializeGdArray(serializedBytes)
	if err != nil {
		return nil
	}
	return info.Data
}

func arrayToGdArrayInfo(arrayPtr Array) *GdArrayInfo {
	arraySize := checkedGdArraySize
	switch data := arrayPtr.(type) {
	case []int64:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		return &GdArrayInfo{Size: size, Type: GdArrayTypeInt64, Data: data}
	case []float32:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		return &GdArrayInfo{Size: size, Type: GdArrayTypeFloat, Data: data}
	case []float64:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		val := f64Tof32(data)
		return &GdArrayInfo{Size: size, Type: GdArrayTypeFloat, Data: val}
	case []bool:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		return &GdArrayInfo{Size: size, Type: GdArrayTypeBool, Data: data}
	case []string:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		return &GdArrayInfo{Size: size, Type: GdArrayTypeString, Data: data}
	case []byte:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		return &GdArrayInfo{Size: size, Type: GdArrayTypeByte, Data: data}
	case []uint64:
		size, ok := arraySize(len(data))
		if !ok {
			return nil
		}
		int64Data := make([]int64, len(data))
		for i, v := range data {
			int64Data[i] = int64(v)
		}
		return &GdArrayInfo{Size: size, Type: GdArrayTypeGdObj, Data: int64Data}
	default:
		return nil
	}
}

func jsValue2Go(value js.Value) any {
	switch value.Type() {
	case js.TypeObject:
		obj := make(map[string]any)
		keys := jsObject.Call("keys", value)
		for i := 0; i < keys.Length(); i++ {
			key := keys.Index(i).String()
			obj[key] = jsValue2Go(value.Get(key)) // Recursively process nested objects
		}
		return obj
	case js.TypeString:
		return value.String()
	case js.TypeNumber:
		return value.Float()
	case js.TypeBoolean:
		return value.Bool()
	default:
		return nil
	}
}

// gdIntFromParts reconstructs the 64-bit GdInt/GdObj value from the web
// bridge's JS-visible (low, high) representation.
func gdIntFromParts(low, high uint32) int64 {
	return int64(uint64(high)<<32 | uint64(low))
}

func getFastGdArrayScratch(arrayType int32) *fastGdArrayScratch {
	if scratch, ok := fastGdArrayScratchByType[arrayType]; ok {
		return scratch
	}
	scratch := &fastGdArrayScratch{
		wrapper: jsObject.New(),
	}
	scratch.wrapper.Set(fastGdArrayFlagKey, true)
	scratch.wrapper.Set(fastGdArrayTypeKey, arrayType)
	fastGdArrayScratchByType[arrayType] = scratch
	return scratch
}

func newFastGdArrayValue(arrayType int32, count int, data []byte) js.Value {
	if fastValue, ok := newBorrowedFastGdArrayValue(arrayType, count, data); ok {
		return fastValue
	}

	scratch := getFastGdArrayScratch(arrayType)
	if scratch.byteCap < len(data) || scratch.bytes.Type() == js.TypeUndefined {
		scratch.bytes = jsUint8Array.New(len(data))
		scratch.byteCap = len(data)
	}
	if len(data) > 0 {
		js.CopyBytesToJS(scratch.bytes, data)
	}
	// Expose only the valid prefix so JS consumers that rely on byteLength/length
	// do not read stale capacity bytes from previous larger buffers.
	scratch.wrapper.Set(fastGdArrayDataKey, scratch.bytes.Call("subarray", 0, len(data)))
	scratch.wrapper.Set(fastGdArrayCountKey, count)
	return scratch.wrapper
}

func newBorrowedFastGdArrayValue(arrayType int32, count int, data []byte) (js.Value, bool) {
	if count < 0 || count > maxGdArrayElements {
		return js.Value{}, false
	}
	if elemSize, ok := fixedGdArrayElemSize(arrayType); !ok || count > maxInt/elemSize || len(data) != count*elemSize {
		return js.Value{}, false
	}
	borrow := js.Global().Get("GdspxBorrowFastArray")
	if borrow.Type() != js.TypeFunction {
		return js.Value{}, false
	}

	wrapper := borrow.Invoke(arrayType, count, len(data))
	if wrapper.IsUndefined() || wrapper.IsNull() || wrapper.Type() != js.TypeObject {
		return js.Value{}, false
	}

	bytes := wrapper.Get(fastGdArrayDataKey)
	if bytes.IsUndefined() || bytes.IsNull() || bytes.Type() != js.TypeObject ||
		!bytes.InstanceOf(jsUint8Array) {
		return js.Value{}, false
	}
	byteLength := bytes.Get("length")
	if byteLength.Type() != js.TypeNumber {
		return js.Value{}, false
	}
	byteLengthFloat := byteLength.Float()
	if !isSafeNonnegativeJSInt(byteLengthFloat) || byteLengthFloat > float64(maxInt) ||
		int(byteLengthFloat) != len(data) {
		return js.Value{}, false
	}

	if len(data) > 0 {
		js.CopyBytesToJS(bytes, data)
	}
	return wrapper, true
}

func arrayToFastGdArrayValue(arrayPtr Array) (js.Value, bool) {
	switch data := arrayPtr.(type) {
	case []float32:
		if len(data) > maxGdArrayElements || len(data) > maxGdArrayBytes/4 {
			return js.Value{}, false
		}
		return newFastGdArrayValue(GdArrayTypeFloat, len(data), bytesFromFloat32Slice(data)), true
	case []int64:
		if len(data) > maxGdArrayElements || len(data) > maxGdArrayBytes/8 {
			return js.Value{}, false
		}
		return newFastGdArrayValue(GdArrayTypeInt64, len(data), bytesFromInt64Slice(data)), true
	case []uint64:
		if len(data) > maxGdArrayElements || len(data) > maxGdArrayBytes/8 {
			return js.Value{}, false
		}
		return newFastGdArrayValue(GdArrayTypeGdObj, len(data), bytesFromUint64Slice(data)), true
	case []byte:
		if len(data) > maxGdArrayElements || len(data) > maxGdArrayBytes {
			return js.Value{}, false
		}
		return newFastGdArrayValue(GdArrayTypeByte, len(data), data), true
	default:
		return js.Value{}, false
	}
}

func fastGdArrayToGo(val js.Value) (Array, bool) {
	flag := val.Get(fastGdArrayFlagKey)
	if flag.Type() != js.TypeBoolean || !flag.Bool() {
		return nil, false
	}

	dataVal := val.Get(fastGdArrayDataKey)
	if dataVal.IsUndefined() || dataVal.IsNull() || dataVal.Type() != js.TypeObject ||
		!dataVal.InstanceOf(jsUint8Array) {
		return nil, true
	}

	lengthVal := dataVal.Get("length")
	if lengthVal.Type() != js.TypeNumber {
		return nil, true
	}
	lengthFloat := lengthVal.Float()
	if !isSafeNonnegativeJSInt(lengthFloat) || lengthFloat > float64(maxInt) {
		return nil, true
	}
	length := int(lengthFloat)
	arrayTypeVal := val.Get(fastGdArrayTypeKey)
	countVal := val.Get(fastGdArrayCountKey)
	if arrayTypeVal.Type() != js.TypeNumber || countVal.Type() != js.TypeNumber {
		return nil, true
	}
	arrayTypeFloat := arrayTypeVal.Float()
	countFloat := countVal.Float()
	if !isSafeNonnegativeJSInt(arrayTypeFloat) || !isSafeNonnegativeJSInt(countFloat) ||
		arrayTypeFloat > float64(maxInt32) || countFloat > float64(maxGdArrayElements) {
		return nil, true
	}
	arrayType := int32(arrayTypeFloat)
	count := int(countFloat)
	if elemSize, ok := fixedGdArrayElemSize(arrayType); ok {
		if count > maxInt/elemSize || length != count*elemSize {
			return nil, true
		}
	}
	if length > maxGdArrayBytes-8 {
		return nil, true
	}

	bytes := make([]byte, length)
	if length > 0 {
		js.CopyBytesToGo(bytes, dataVal)
	}

	data, err := deserializeDataByType(arrayType, bytes, int32(count))
	if err != nil {
		return nil, true
	}
	return data, true
}

func isSafeNonnegativeJSInt(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && math.Trunc(value) == value && value <= maxSafeJSInt
}
