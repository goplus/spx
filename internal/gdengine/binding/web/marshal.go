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
	"strings"
	"syscall/js"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

var jsObject = js.Global().Get("Object")

func JSToGdBool(val js.Value) bool {
	switch val.Type() {
	case js.TypeNumber:
		return val.Int() != 0
	case js.TypeBoolean:
		return val.Bool()
	default:
		panic("unsupported JavaScript boolean value")
	}
}

func JSFromGdInt(val int64) js.Value {
	intJS := jsObject.New()
	low, high := JSSplitGdInt(val)
	intJS.Set("low", low)
	intJS.Set("high", high)
	return intJS
}

func JSToGdInt(val js.Value) int64 {
	low := uint32(val.Get("low").Int())
	high := uint32(val.Get("high").Int())
	return gdIntFromParts(low, high)
}

// JSSplitGdInt encodes a web bridge GdInt/GdObj as (low, high) uint32 parts.
// JS wrappers rebuild a signed BigInt in this order for by-value Wasm inputs.
func JSSplitGdInt(val int64) (uint32, uint32) {
	return uint32(val), uint32(val >> 32)
}

// gdIntFromParts reconstructs the 64-bit GdInt/GdObj value from the web
// bridge's JS-visible (low, high) representation.
func gdIntFromParts(low, high uint32) int64 {
	return int64(uint64(high)<<32 | uint64(low))
}

func JSFromGdObj(val engine.Object) js.Value {
	return JSFromGdInt(int64(val))
}

func JSToGdObj(val js.Value) int64 {
	return JSToGdInt(val)
}

func JSToGdObject(val js.Value) engine.Object {
	return engine.Object(JSToGdInt(val))
}

func JSSplitGdObj(val engine.Object) (uint32, uint32) {
	return JSSplitGdInt(int64(val))
}

func JSToGdFloat(val js.Value) float64 {
	return val.Float()
}

func JSFromGdString(value string) js.Value {
	return js.ValueOf(value)
}

func JSToGdString(value js.Value) string {
	s := value.String()
	// Strip null terminators from C/FFI layer to ensure proper string comparison
	return strings.TrimRight(s, "\x00")
}

func JSFromGdVec2(vec mathf.Vec2) js.Value {
	vec2JS := jsObject.New()
	vec2JS.Set("x", float32(vec.X))
	vec2JS.Set("y", float32(vec.Y))
	return vec2JS
}

func JSToGdVec2(vec js.Value) mathf.Vec2 {
	return mathf.Vec2{
		X: vec.Get("x").Float(),
		Y: vec.Get("y").Float(),
	}
}

func JSFromGdVec3(vec mathf.Vec3) js.Value {
	vec3JS := jsObject.New()
	vec3JS.Set("x", float32(vec.X))
	vec3JS.Set("y", float32(vec.Y))
	vec3JS.Set("z", float32(vec.Z))
	return vec3JS
}

func JSToGdVec3(vec js.Value) mathf.Vec3 {
	return mathf.Vec3{
		X: vec.Get("x").Float(),
		Y: vec.Get("y").Float(),
		Z: vec.Get("z").Float(),
	}
}

func JSFromGdVec4(vec mathf.Vec4) js.Value {
	vec4JS := jsObject.New()
	vec4JS.Set("x", float32(vec.X))
	vec4JS.Set("y", float32(vec.Y))
	vec4JS.Set("z", float32(vec.Z))
	vec4JS.Set("w", float32(vec.W))
	return vec4JS
}

func JSToGdVec4(vec js.Value) mathf.Vec4 {
	return mathf.Vec4{
		X: vec.Get("x").Float(),
		Y: vec.Get("y").Float(),
		Z: vec.Get("z").Float(),
		W: vec.Get("w").Float(),
	}
}

func JSFromGdColor(color mathf.Color) js.Value {
	colorJS := jsObject.New()
	colorJS.Set("r", float32(color.R))
	colorJS.Set("g", float32(color.G))
	colorJS.Set("b", float32(color.B))
	colorJS.Set("a", float32(color.A))
	return colorJS
}

func JSToGdColor(color js.Value) mathf.Color {
	return mathf.Color{
		R: color.Get("r").Float(),
		G: color.Get("g").Float(),
		B: color.Get("b").Float(),
		A: color.Get("a").Float(),
	}
}

func JSFromGdRect2(rect mathf.Rect2) js.Value {
	rectJS := jsObject.New()
	rectJS.Set("position", JSFromGdVec2(rect.Position))
	rectJS.Set("size", JSFromGdVec2(rect.Size))
	return rectJS
}

func JSToGdRect2(rect js.Value) mathf.Rect2 {
	return mathf.Rect2{
		Position: JSToGdVec2(rect.Get("position")),
		Size:     JSToGdVec2(rect.Get("size")),
	}
}
