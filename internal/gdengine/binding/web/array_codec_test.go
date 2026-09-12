package webffi

import (
	"bytes"
	"reflect"
	"testing"
)

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
