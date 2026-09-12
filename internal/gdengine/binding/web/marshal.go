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
	"strings"
	"syscall/js"

	. "github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v3/internal/log"
	. "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

var jsObject = js.Global().Get("Object")

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
