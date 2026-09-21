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
	"reflect"
	"testing"
	"unsafe"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestToGdArrayInfoPreservesTypedEmptyArrays(t *testing.T) {
	tests := []struct {
		name     string
		empty    any
		nilValue any
		wantType int64
		toSlice  func(*ArrayInfoImpl) any
	}{
		{
			name:     "int64",
			empty:    []int64{},
			nilValue: []int64(nil),
			wantType: ArrayTypeInt64,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToInt64s() },
		},
		{
			name:     "float32",
			empty:    []float32{},
			nilValue: []float32(nil),
			wantType: ArrayTypeFloat,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToFloats() },
		},
		{
			name:     "float64",
			empty:    []float64{},
			nilValue: []float64(nil),
			wantType: ArrayTypeFloat,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToFloats() },
		},
		{
			name:     "bool",
			empty:    []bool{},
			nilValue: []bool(nil),
			wantType: ArrayTypeBool,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToBools() },
		},
		{
			name:     "string",
			empty:    []string{},
			nilValue: []string(nil),
			wantType: ArrayTypeString,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToStrings() },
		},
		{
			name:     "object",
			empty:    []GdObj{},
			nilValue: []GdObj(nil),
			wantType: ArrayTypeGdObj,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToObjects() },
		},
		{
			name:     "byte",
			empty:    []byte{},
			nilValue: []byte(nil),
			wantType: ArrayTypeByte,
			toSlice:  func(info *ArrayInfoImpl) any { return info.ToBytes() },
		},
	}

	for _, test := range tests {
		t.Run(test.name+"/empty", func(t *testing.T) {
			info := ToGdArrayInfo(test.empty)
			if info == nil {
				t.Fatal("ToGdArrayInfo() returned nil")
			}
			defer info.Free()

			if info.Raw() == nil {
				t.Fatal("ToGdArrayInfo().Raw() returned nil for a typed empty array")
			}
			if got := info.Size(); got != 0 {
				t.Fatalf("ToGdArrayInfo().Size() = %d, want 0", got)
			}
			if got := info.Type(); got != test.wantType {
				t.Fatalf("ToGdArrayInfo().Type() = %d, want %d", got, test.wantType)
			}

			converted := reflect.ValueOf(test.toSlice(info))
			if converted.Kind() != reflect.Slice || converted.IsNil() || converted.Len() != 0 {
				t.Fatalf("converted value = %#v, want a non-nil empty slice", converted.Interface())
			}
		})

		t.Run(test.name+"/nil", func(t *testing.T) {
			info := ToGdArrayInfo(test.nilValue)
			if info == nil {
				t.Fatal("ToGdArrayInfo() returned nil")
			}
			defer info.Free()

			if info.Raw() != nil {
				t.Fatal("ToGdArrayInfo().Raw() returned non-nil for a typed nil slice")
			}
		})
	}
}

func TestStringCallbackBorrowsCallerMemory(t *testing.T) {
	previous := callbacks
	t.Cleanup(func() { BindCallback(previous) })
	input := ToGdArrayInfo([]string{"borrowed"})
	defer input.Free()
	ptr := *(*unsafe.Pointer)(input.gdArray.data)
	var got string
	BindCallback(engine.CallbackInfo{OnActionPressed: func(value string) { got = value }})
	invokeStringCallback(func_on_action_pressed, ptr)
	if got != "borrowed" || input.ToStrings()[0] != "borrowed" {
		t.Fatalf("callback = %q, caller input = %q", got, input.ToStrings()[0])
	}
	unsafe.Slice((*byte)(ptr), len(got))[0] = 'B'
	if got != "borrowed" || input.ToStrings()[0] != "Borrowed" {
		t.Fatalf("callback did not retain an independent copy: callback = %q, input = %q", got, input.ToStrings()[0])
	}
	BindCallback(engine.CallbackInfo{})
	invokeStringCallback(func_on_action_pressed, ptr)
	if input.ToStrings()[0] != "Borrowed" {
		t.Fatal("a missing handler changed the caller input")
	}
}

// Infer the cgo pointer type without adding an import-C production test hook.
func invokeStringCallback[T ~unsafe.Pointer](callback func(T), input unsafe.Pointer) {
	callback(T(input))
}
