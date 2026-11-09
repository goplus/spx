//go:build cgo

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

import (
	"math"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/gdengine/binding/native/testdata/stringallocator"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestNativeScalarConversions(t *testing.T) {
	for _, value := range []bool{false, true} {
		if ToBool(ToGdBool(value)) != value {
			t.Fatalf("boolean round trip %v", value)
		}
	}
	if !ToBool(2) || ToString(nil) != "" || copyString(nil) != "" {
		t.Fatal("noncanonical boolean or nil string conversion")
	}
	for _, value := range []int64{math.MinInt64, -1, 0, 1<<54 + 3, math.MaxInt64} {
		if ToInt64(ToGdInt(value)) != value || int64(ToObject(ToGdObj(engine.Object(value)))) != value {
			t.Fatalf("integer precision lost: %d", value)
		}
	}
	for _, value := range []float64{-1.5, 0, 2.25} {
		gd := ToGdFloat(value)
		if ToFloat64(gd) != value {
			t.Fatalf("float round trip %v", value)
		}
	}
	vec2, vec4 := mathf.NewVec2(-1.5, 2.25), mathf.NewVec4(1, 2, 3, 4)
	color := mathf.NewColor(.25, .5, .75, 1)
	rect := mathf.Rect2{Position: vec2, Size: mathf.NewVec2(40, 50)}
	if ToVec2(ToGdVec2(vec2)) != vec2 || ToVec4(ToGdVec4(vec4)) != vec4 || ToColor(ToGdColor(color)) != color || ToRect2(ToGdRect2(rect)) != rect {
		t.Fatal("structured value round trip changed components")
	}
}

func TestReturnedStringsUseRuntimeAllocator(t *testing.T) {
	previous := builtinAPI.SpxGlobalFreeString
	builtinAPI.SpxGlobalFreeString = GDExtensionSpxGlobalFreeString(stringallocator.FreeCallback())
	t.Cleanup(func() { builtinAPI.SpxGlobalFreeString = previous })
	for _, tt := range []struct{ name, value string }{
		{"empty", ""},
		{"unicode", "héllo 世界"},
		{"newlines", "line one\nline two"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stringallocator.Reset()
			t.Cleanup(stringallocator.Reset)
			owned := GdString(stringallocator.Allocate(tt.value))
			if owned == nil {
				t.Fatal("fixture failed to allocate a native string")
			}
			borrowed := copyString(owned)
			if borrowed != tt.value || stringallocator.ReleaseCount() != 0 {
				t.Fatalf("borrowed string = %q, release count = %d", borrowed, stringallocator.ReleaseCount())
			}
			got := ToString(owned)
			if got != tt.value || borrowed != tt.value {
				t.Fatalf("string copies after native release = %q, %q; want %q", got, borrowed, tt.value)
			}
			if stringallocator.ReleaseCount() != 1 || stringallocator.HasOutstandingAllocation() {
				t.Fatalf("runtime allocator release count = %d, outstanding allocation = %v", stringallocator.ReleaseCount(), stringallocator.HasOutstandingAllocation())
			}
			if got := ToString(nil); got != "" || stringallocator.ReleaseCount() != 1 {
				t.Fatalf("nil return = %q, release count = %d", got, stringallocator.ReleaseCount())
			}
		})
	}
}
