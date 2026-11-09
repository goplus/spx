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
	"reflect"
	"strconv"
	"testing"
)

func TestToGdArrayInfoPreservesTypedEmptyArrays(t *testing.T) {
	tests := []struct {
		name     string
		empty    any
		nilValue any
		wantType int64
		toSlice  func(*ArrayInfo) any
	}{
		{
			name:     "int64",
			empty:    []int64{},
			nilValue: []int64(nil),
			wantType: ArrayTypeInt64,
			toSlice:  func(info *ArrayInfo) any { return info.ToInt64s() },
		},
		{
			name:     "float32",
			empty:    []float32{},
			nilValue: []float32(nil),
			wantType: ArrayTypeFloat,
			toSlice:  func(info *ArrayInfo) any { return info.ToFloat32s() },
		},
		{
			name:     "float64",
			empty:    []float64{},
			nilValue: []float64(nil),
			wantType: ArrayTypeFloat,
			toSlice:  func(info *ArrayInfo) any { return info.ToFloat32s() },
		},
		{
			name:     "bool",
			empty:    []bool{},
			nilValue: []bool(nil),
			wantType: ArrayTypeBool,
			toSlice:  func(info *ArrayInfo) any { return info.ToBools() },
		},
		{
			name:     "string",
			empty:    []string{},
			nilValue: []string(nil),
			wantType: ArrayTypeString,
			toSlice:  func(info *ArrayInfo) any { return info.ToStrings() },
		},
		{
			name:     "object",
			empty:    []GdObj{},
			nilValue: []GdObj(nil),
			wantType: ArrayTypeGdObj,
			toSlice:  func(info *ArrayInfo) any { return info.ToObjectIDs() },
		},
		{
			name:     "byte",
			empty:    []byte{},
			nilValue: []byte(nil),
			wantType: ArrayTypeByte,
			toSlice:  func(info *ArrayInfo) any { return info.ToBytes() },
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

func TestNativeArrayValuesAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		name        string
		input, want any
		kind        int64
	}{
		{"integers", []int64{math.MinInt64, 0, math.MaxInt64}, []int64{math.MinInt64, 0, math.MaxInt64}, ArrayTypeInt64},
		{"float32", []float32{-1.5, 0, 2.25}, []float32{-1.5, 0, 2.25}, ArrayTypeFloat},
		{"float64", []float64{-1.5, 0, 2.25}, []float32{-1.5, 0, 2.25}, ArrayTypeFloat},
		{"booleans", []bool{true, false, true}, []bool{true, false, true}, ArrayTypeBool},
		{"bytes", []byte{0, 127, 255}, []byte{0, 127, 255}, ArrayTypeByte},
		{"objects", []GdObj{-1, 0, 1<<54 + 3}, []int64{-1, 0, 1<<54 + 3}, ArrayTypeGdObj},
		{"strings", []string{"", "héllo", "two words"}, []string{"", "héllo", "two words"}, ArrayTypeString},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := ToGdArrayInfo(tc.input)
			defer info.Free()
			if info.Size() != 3 || info.Type() != tc.kind {
				t.Fatalf("array metadata = %d/%d", info.Size(), info.Type())
			}
			readers := map[int64]func() any{
				ArrayTypeInt64: func() any { return info.ToInt64s() }, ArrayTypeFloat: func() any { return info.ToFloat32s() },
				ArrayTypeBool: func() any { return info.ToBools() }, ArrayTypeByte: func() any { return info.ToBytes() },
				ArrayTypeGdObj: func() any { return info.ToObjectIDs() }, ArrayTypeString: func() any { return info.ToStrings() },
			}
			for kind, read := range readers {
				got := read()
				if kind == tc.kind {
					if !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("decoded = %#v, want %#v", got, tc.want)
					}
				} else if !reflect.ValueOf(got).IsNil() {
					t.Fatalf("wrong type %d decoded as %#v", kind, got)
				}
			}
			// Each read is a Go-owned copy; changing it must not alter native storage.
			reflect.ValueOf(readers[tc.kind]()).Clear()
			if got := readers[tc.kind](); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("native buffer aliased Go copy: %#v", got)
			}
			info.Free()
			info.Free()
			if info.Raw() != nil || info.Size() != 0 || info.Type() != 0 {
				t.Fatal("freed array retained native storage")
			}
			for _, read := range readers {
				if !reflect.ValueOf(read()).IsNil() {
					t.Fatal("read from freed array returned data")
				}
			}
			// ToArray consumes the owned return buffer; its result survives that free.
			if got := ToArray(ToGdArrayInfo(tc.input).Raw()); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("owned return = %#v, want %#v", got, tc.want)
			}
		})
	}
	if (*ArrayInfo)(nil).Raw() != nil || ToArray(nil) != nil || ToGdArrayInfo([]byte(nil)).Raw() != nil {
		t.Fatal("nil array conversion was not nil")
	}
	for _, input := range []any{nil, 42, []int{1}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("unsupported array %T accepted", input)
				}
			}()
			ToGdArrayInfo(input)
		}()
	}
}

func TestToArrayRejectsUnknownNativeType(t *testing.T) {
	info := allocateGdArrayInfo(-1, 0, false)
	if info == nil || info.Raw() == nil {
		t.Fatal("failed to allocate empty native array")
	}
	// ToArray takes ownership of this buffer; do not also call info.Free.
	if got := ToArray(info.Raw()); got != nil {
		t.Fatalf("unknown native array type decoded as %#v, want nil", got)
	}
}

func TestNativeArrayAllocationRejectsInvalidMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind int64
		size int
	}{
		{"negative size", ArrayTypeByte, -1},
		{"unknown nonempty type", -1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if info := allocateGdArrayInfo(tc.kind, tc.size, false); info != nil {
				info.Free()
				t.Fatal("invalid native array metadata allocated storage")
			}
		})
	}
}

func TestNativeArrayAllocationRejectsLengthOverflow(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("Go lengths cannot exceed int32 on a 32-bit target")
	}
	for _, size := range []int64{math.MaxInt32 + 1, 1 << 32, 1<<32 + 1} {
		t.Run(strconv.FormatInt(size, 10), func(t *testing.T) {
			defer func() {
				if got := recover(); got != "gdengine: native array length exceeds int32" {
					t.Fatalf("oversized array panic = %v", got)
				}
			}()
			if info := allocateGdArrayInfo(ArrayTypeByte, int(size), false); info != nil {
				info.Free()
			}
		})
	}
}

func TestNativeArrayAllocationRejectsNegativeLengthBeforeNarrowing(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("requires a 64-bit Go length")
	}
	for _, size := range []int64{-1 << 32, -1<<32 + 1, math.MinInt64} {
		if info := allocateGdArrayInfo(ArrayTypeByte, int(size), false); info != nil {
			info.Free()
			t.Errorf("negative length %d allocated native storage", size)
		}
	}
}

func TestNativeArrayAllocationRejectsByteSizeOverflow(t *testing.T) {
	if strconv.IntSize != 32 {
		t.Skip("int32 element counts cannot overflow size_t on a 64-bit target")
	}
	// The element count fits int32, but count * sizeof(int64_t) exceeds size_t.
	defer func() {
		if got := recover(); got != "gdengine: native array byte size overflows size_t" {
			t.Fatalf("overflowing allocation panic = %v", got)
		}
	}()
	if info := allocateGdArrayInfo(ArrayTypeInt64, 1<<29, false); info != nil {
		info.Free()
	}
}
