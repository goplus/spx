//go:build js && wasm

package webffi

import (
	"bytes"
	"math"
	"reflect"
	"syscall/js"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestNativeArrayOutputCopiesIntoCallerStorage(t *testing.T) {
	for _, test := range []struct {
		arrayType    int32
		target, want any
		data         []byte
	}{
		{GdArrayTypeFloat, []float32{0}, []float32{1.5}, []byte{0, 0, 192, 63}},
		{GdArrayTypeInt64, []int64{0}, []int64{-1}, []byte{255, 255, 255, 255, 255, 255, 255, 255}},
		{GdArrayTypeGdObj, []int64{0}, []int64{-1 << 63}, []byte{0, 0, 0, 0, 0, 0, 0, 128}},
		{GdArrayTypeGdObj, []uint64{0}, []uint64{1 << 63}, []byte{0, 0, 0, 0, 0, 0, 0, 128}},
		{GdArrayTypeByte, []byte{0}, []byte{23}, []byte{23}},
	} {
		bytes := jsUint8Array.New(len(test.data))
		js.CopyBytesToJS(bytes, test.data)
		wrapper := js.ValueOf(map[string]any{
			arrayTag: true, "type": test.arrayType, "count": 1, "data": bytes,
		})
		CopyNativeArrayOutput(test.target, wrapper)
		if !reflect.DeepEqual(test.target, test.want) {
			t.Fatalf("copy into %T: got %v, want %v", test.target, test.target, test.want)
		}
		wrapper.Set("count", 2)
		func() {
			defer func() {
				if recover() == nil {
					t.Error("mismatched output shape was accepted")
				}
			}()
			CopyNativeArrayOutput(test.target, wrapper)
		}()
		if !reflect.DeepEqual(test.target, test.want) {
			t.Fatal("invalid output changed caller storage")
		}
	}
}

func TestNativeArrayRejectsInvalidMetadata(t *testing.T) {
	for _, test := range []struct {
		arrayType, count any
		length           int
	}{
		{99, 0, 0},
		{GdArrayTypeInt64, -1, 0},
		{GdArrayTypeInt64, 0.5, 0},
		{GdArrayTypeInt64, math.NaN(), 0},
		{GdArrayTypeInt64, math.Inf(1), 0},
		{GdArrayTypeInt64, maxGdArrayElements + 1, 0},
		{GdArrayTypeInt64, "1", 8},
		{GdArrayTypeInt64, 1, 4},
		{GdArrayTypeByte, 0, 1},
		{GdArrayTypeString, 1, 8},
	} {
		value := js.ValueOf(map[string]any{
			arrayTag: true,
			"type":   test.arrayType,
			"count":  test.count,
			"data":   jsUint8Array.New(test.length),
		})
		if got := JsToGdArray(value); got != nil {
			t.Fatalf("accepted invalid metadata %#v: %#v", test, got)
		}
	}
}

func TestNativeArrayBridgeRoundtrip(t *testing.T) {
	previous := js.Global().Get("GdspxBorrowNativeArray")
	borrow := js.FuncOf(func(_ js.Value, args []js.Value) any {
		return map[string]any{
			arrayTag: true,
			"type":   args[0].Int(),
			"count":  args[1].Int(),
			"data":   jsUint8Array.New(args[2].Int()),
		}
	})
	js.Global().Set("GdspxBorrowNativeArray", borrow)
	t.Cleanup(func() {
		js.Global().Set("GdspxBorrowNativeArray", previous)
		borrow.Release()
	})
	for _, test := range []struct {
		input, want any
	}{
		{[]int64{-1, 1 << 40}, []int64{-1, 1 << 40}},
		{[]uint64{1 << 63, 7}, []int64{-1 << 63, 7}},
		{[]float32{1.5, -2}, []float32{1.5, -2}},
		{[]float64{1.5, -2}, []float32{1.5, -2}},
		{[]bool{true, false}, []bool{true, false}},
		{[]string{"", "你好", "a\x00b"}, []string{"", "你好", "a\x00b"}},
		{[]byte{0, 255}, []byte{0, 255}},
		{[]string(nil), []string{}},
		{[]bool(nil), []bool{}},
	} {
		value := JsFromGdArray(test.input)
		if got := JsToGdArray(value); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("roundtrip %T: got %#v, want %#v", test.input, got, test.want)
		}
	}
	if got := JsToGdArray(JsFromNativeArray([]int64{-1, 1<<54 + 3}, GdArrayTypeGdObj)); !reflect.DeepEqual(got, []int64{-1, 1<<54 + 3}) {
		t.Fatalf("declared object array=%v", got)
	}
	for _, call := range []func(){func() { JsFromGdArray(nil) }, func() { JsFromNativeArray([]byte{1}, GdArrayTypeFloat) }, func() { CopyNativeArrayOutput([]bool{true}, js.Undefined()) }, func() { CopyNativeArrayOutput([]byte{1}, js.Undefined()) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid native array request accepted")
				}
			}()
			call()
		}()
	}
	if !JsFromGdArray(42).IsNull() {
		t.Fatal("unsupported storage returned an array")
	}
}

func TestAllocateNativeOutputKeepsBorrowedBytes(t *testing.T) {
	previous := js.Global().Get("GdspxBorrowNativeArray")
	calls := 0
	borrow := js.FuncOf(func(_ js.Value, args []js.Value) any {
		calls++
		data := jsUint8Array.New(args[2].Int())
		data.Call("fill", 0xa5)
		return map[string]any{arrayTag: true, "type": args[0].Int(), "count": args[1].Int(), "data": data}
	})
	js.Global().Set("GdspxBorrowNativeArray", borrow)
	t.Cleanup(func() { js.Global().Set("GdspxBorrowNativeArray", previous); borrow.Release() })
	for _, typ := range []int32{GdArrayTypeFloat, GdArrayTypeInt64, GdArrayTypeGdObj, GdArrayTypeByte} {
		for _, count := range []int{0, 3} {
			wrapper := JsAllocNativeArray(typ, count)
			size, _ := arrayElementSize(typ)
			data := make([]byte, size*count)
			js.CopyBytesToGo(data, wrapper.Get("data"))
			for _, value := range data {
				if value != 0xa5 {
					t.Fatal("output allocation modified borrowed storage")
				}
			}
		}
	}
	previousCalls := calls
	for _, tc := range []struct {
		typ   int32
		count int
	}{
		{GdArrayTypeFloat, -1}, {GdArrayTypeFloat, maxGdArrayElements + 1},
		{GdArrayTypeString, 3}, {GdArrayTypeBool, 3}, {99, 3},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid output allocation was accepted")
				}
			}()
			JsAllocNativeArray(tc.typ, tc.count)
		}()
	}
	if calls != previousCalls {
		t.Fatal("invalid output allocation reached JavaScript")
	}
	js.Global().Set("GdspxBorrowNativeArray", js.Undefined())
	defer func() {
		if recover() == nil {
			t.Error("missing allocator did not fail")
		}
	}()
	JsAllocNativeArray(GdArrayTypeFloat, 3)
}

func TestWebScalarMarshallingPreservesValues(t *testing.T) {
	for _, value := range []int64{math.MinInt64, -1, 0, 1<<54 + 3, math.MaxInt64} {
		if got := JsToGdInt(JsFromGdInt(value)); got != value {
			t.Errorf("integer round trip=%d, want %d", got, value)
		}
		object := engine.Object(value)
		if JsToGdObject(JsFromGdObj(object)) != object || JsToGdObj(JsFromGdObj(object)) != value {
			t.Errorf("object precision lost: %d", value)
		}
		low, high := JsSplitGdObj(object)
		if gdIntFromParts(low, high) != value {
			t.Errorf("split object precision lost: %d", value)
		}
	}
	for _, tc := range []struct {
		input any
		want  bool
	}{{true, true}, {false, false}, {2, true}, {0, false}, {-1, true}} {
		if got := JsToGdBool(js.ValueOf(tc.input)); got != tc.want {
			t.Errorf("boolean %v=%v", tc.input, got)
		}
	}
	for _, value := range []js.Value{js.Null(), js.Undefined(), js.ValueOf("true")} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid boolean accepted")
				}
			}()
			JsToGdBool(value)
		}()
	}
	if got := JsToGdString(JsFromGdString("héllo\x00inside\x00\x00")); got != "héllo\x00inside" {
		t.Fatalf("string terminators=%q", got)
	}
	vec2, vec3, vec4 := mathf.NewVec2(1.5, -2.25), mathf.NewVec3(1, 2, 3), mathf.NewVec4(1, 2, 3, 4)
	color := mathf.NewColor(.25, .5, .75, 1)
	rect := mathf.Rect2{Position: vec2, Size: mathf.NewVec2(40, 50)}
	if JsToGdVec2(JsFromGdVec2(vec2)) != vec2 || JsToGdVec3(JsFromGdVec3(vec3)) != vec3 || JsToGdVec4(JsFromGdVec4(vec4)) != vec4 || JsToGdColor(JsFromGdColor(color)) != color || JsToGdRect2(JsFromGdRect2(rect)) != rect {
		t.Fatal("structured value round trip changed components")
	}
	if JsToGdFloat(js.ValueOf(-1.5)) != -1.5 || JsToGdFloat32(js.ValueOf(2.25)) != 2.25 || JsToGdInt64(js.ValueOf(-42)) != -42 {
		t.Fatal("scalar conversion lost value")
	}
}

func TestBorrowNativeArrayRejectsMalformedHostBuffers(t *testing.T) {
	previous := js.Global().Get("GdspxBorrowNativeArray")
	t.Cleanup(func() { js.Global().Set("GdspxBorrowNativeArray", previous) })
	for _, result := range []any{42, map[string]any{arrayTag: true, "type": GdArrayTypeByte, "count": 1, "data": jsUint8Array.New(0)}, map[string]any{arrayTag: false, "type": GdArrayTypeByte, "count": 1, "data": jsUint8Array.New(1)}} {
		func() {
			borrow := js.FuncOf(func(js.Value, []js.Value) any { return result })
			defer borrow.Release()
			js.Global().Set("GdspxBorrowNativeArray", borrow)
			if !JsFromGdArray([]byte{1}).IsNull() {
				t.Errorf("malformed host buffer accepted: %v", result)
			}
		}()
	}
	for _, value := range []js.Value{js.ValueOf(42), js.ValueOf(map[string]any{arrayTag: false}), js.ValueOf(map[string]any{arrayTag: true, "data": "not bytes"})} {
		if JsToGdArray(value) != nil {
			t.Error("malformed JS array accepted")
		}
	}
}

func TestCheckedArrayCount(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int32
		ok    bool
	}{
		{name: "zero", input: 0, want: 0, ok: true},
		{name: "maximum", input: maxGdArrayElements, want: maxGdArrayElements, ok: true},
		{name: "over maximum", input: maxGdArrayElements + 1, ok: false},
		{name: "over int32", input: 1 << 31, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := checkedArrayCount(test.input)
			if ok != test.ok || (ok && got != test.want) {
				t.Fatalf("checkedArrayCount(%d) = (%d, %t), want (%d, %t)", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

// The payload has no array header; type and count live in the native descriptor.
func TestNativeArrayLayout(t *testing.T) {
	tests := []struct {
		name   string
		typeID int32
		count  int32
		value  any
		data   []byte
	}{
		{"int64", GdArrayTypeInt64, 1, []int64{-1}, []byte{255, 255, 255, 255, 255, 255, 255, 255}},
		{"object", GdArrayTypeGdObj, 1, []int64{1 << 40}, []byte{0, 0, 0, 0, 0, 1, 0, 0}},
		{"float", GdArrayTypeFloat, 1, []float32{1.5}, []byte{0, 0, 192, 63}},
		{"bool", GdArrayTypeBool, 2, []bool{false, true}, []byte{0, 1}},
		{"string", GdArrayTypeString, 1, []string{"hi"}, []byte{8, 0, 0, 0, 2, 0, 0, 0, 'h', 'i', 0}},
		{"byte", GdArrayTypeByte, 2, []byte{7, 9}, []byte{7, 9}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := encodeArrayData(test.typeID, test.value)
			if err != nil || !bytes.Equal(encoded, test.data) {
				t.Fatalf("encode = %v, %v; want %v", encoded, err, test.data)
			}
			decoded, err := decodeArrayData(test.typeID, test.data, test.count)
			if err != nil || !reflect.DeepEqual(decoded, test.value) {
				t.Fatalf("decode = %#v, %v; want %#v", decoded, err, test.value)
			}
		})
	}
}

func TestNativeArrayEmptyTypes(t *testing.T) {
	for _, typ := range []int32{GdArrayTypeInt64, GdArrayTypeFloat, GdArrayTypeBool, GdArrayTypeString, GdArrayTypeByte, GdArrayTypeGdObj} {
		decoded, err := decodeArrayData(typ, nil, 0)
		if err != nil || decoded == nil {
			t.Fatalf("empty type %d: %#v, %v", typ, decoded, err)
		}
		value := reflect.ValueOf(decoded)
		if value.Kind() != reflect.Slice || value.IsNil() || value.Len() != 0 {
			t.Fatalf("empty type %d lost its non-nil slice: %#v", typ, decoded)
		}
	}
	if float32Slice(nil) == nil {
		t.Fatal("Web conversion must preserve its non-nil empty result")
	}
}

func TestNativeStringArrayRanges(t *testing.T) {
	values := []string{"", "你好", "a\x00b"}
	data, err := encodeArrayData(GdArrayTypeString, values)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeArrayData(GdArrayTypeString, data, int32(len(values)))
	if err != nil || !reflect.DeepEqual(got, values) {
		t.Fatalf("%v, %v", got, err)
	}
	for _, change := range []func([]byte){
		func(b []byte) { b[0]++ },
		func(b []byte) { b[4] = 255 },
		func(b []byte) { b[len(b)-1] = 1 },
	} {
		invalid := bytes.Clone(data)
		change(invalid)
		if _, err := decodeArrayData(GdArrayTypeString, invalid, int32(len(values))); err == nil {
			t.Fatal("accepted invalid string range")
		}
	}
}

func TestNativeArrayRejectsMalformedPayload(t *testing.T) {
	for _, test := range []struct {
		typ, count int32
		data       []byte
	}{
		{GdArrayTypeInt64, -1, nil},
		{99, 0, nil},
		{GdArrayTypeInt64, 1, []byte{1}},
		{GdArrayTypeString, 1, []byte{8, 0, 0, 0, 0, 0, 0, 0}},
		{GdArrayTypeByte, 0, []byte{1}},
	} {
		if _, err := decodeArrayData(test.typ, test.data, test.count); err == nil {
			t.Fatalf("accepted malformed payload %#v", test)
		}
	}
}

func FuzzNativeArrayLayout(f *testing.F) {
	f.Add(int32(1), int32(1), []byte{1, 0, 0, 0, 0, 0, 0, 0})
	f.Add(int32(4), int32(1), []byte{8, 0, 0, 0, 2, 0, 0, 0, 'h', 'i', 0})
	f.Add(int32(4), int32(0), []byte{})
	f.Fuzz(func(t *testing.T, typ, count int32, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		decoded, err := decodeArrayData(typ, data, count)
		if err != nil {
			return
		}
		encoded, err := encodeArrayData(typ, decoded)
		if err != nil {
			t.Fatal(err)
		}
		again, err := decodeArrayData(typ, encoded, count)
		if err != nil || reflect.TypeOf(again) != reflect.TypeOf(decoded) {
			t.Fatalf("roundtrip type changed: %#v, %v", again, err)
		}
	})
}

func TestArrayEncoderRejectsMismatchedStorage(t *testing.T) {
	for _, typ := range []int32{GdArrayTypeInt64, GdArrayTypeGdObj, GdArrayTypeFloat, GdArrayTypeBool, GdArrayTypeByte, GdArrayTypeString, 99} {
		if _, err := encodeArrayData(typ, 42); err == nil {
			t.Errorf("type %d accepted integer as array storage", typ)
		}
	}
	if data, err := encodeArrayData(GdArrayTypeByte, nil); err != nil || len(data) != 0 {
		t.Fatalf("nil bytes=%v,%v", data, err)
	}
	if err := validateStringArray(nil, -1); err == nil {
		t.Fatal("negative string table count accepted")
	}
	data, err := encodeStringArray([]string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeStringArray(append(data, 0), 1); err == nil {
		t.Fatal("trailing string bytes accepted")
	}
}
