package webffi

import (
	"bytes"
	"reflect"
	"testing"
)

func TestCheckedGdArraySize(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int32
		ok    bool
	}{
		{name: "zero", input: 0, want: 0, ok: true},
		{name: "maximum", input: maxGdArrayElements, want: maxGdArrayElements, ok: true},
		{name: "over maximum", input: maxGdArrayElements + 1, ok: false},
		{name: "over int32", input: int(maxInt32) + 1, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := checkedGdArraySize(test.input)
			if ok != test.ok || (ok && got != test.want) {
				t.Fatalf("checkedGdArraySize(%d) = (%d, %t), want (%d, %t)", test.input, got, ok, test.want, test.ok)
			}
		})
	}
}

// Wire fixtures exercise the codec without requiring a JavaScript runtime.
func TestArrayCodecWireFormat(t *testing.T) {
	tests := []struct {
		name string
		info GdArrayInfo
		wire []byte
	}{
		{"int64", GdArrayInfo{1, GdArrayTypeInt64, []int64{-1}}, []byte{1, 0, 0, 0, 1, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255}},
		{"object", GdArrayInfo{1, GdArrayTypeGdObj, []int64{1 << 40}}, []byte{1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0}},
		{"float", GdArrayInfo{1, GdArrayTypeFloat, []float32{1.5}}, []byte{1, 0, 0, 0, 2, 0, 0, 0, 0, 0, 192, 63}},
		{"bool", GdArrayInfo{2, GdArrayTypeBool, []bool{false, true}}, []byte{2, 0, 0, 0, 3, 0, 0, 0, 0, 1}},
		{"string", GdArrayInfo{1, GdArrayTypeString, []string{"hi"}}, []byte{1, 0, 0, 0, 4, 0, 0, 0, 2, 0, 0, 0, 104, 105}},
		{"byte", GdArrayInfo{2, GdArrayTypeByte, []byte{7, 9}}, []byte{2, 0, 0, 0, 5, 0, 0, 0, 7, 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire, err := serializeGdArray(&tt.info)
			if err != nil || !bytes.Equal(wire, tt.wire) {
				t.Fatalf("encode = %v, %v; want %v", wire, err, tt.wire)
			}
			got, err := deserializeGdArray(tt.wire)
			if err != nil || !reflect.DeepEqual(got, &tt.info) {
				t.Fatalf("decode = %#v, %v; want %#v", got, err, tt.info)
			}
		})
	}
}

func TestArrayCodecPreservesEmptyTypes(t *testing.T) {
	for _, typ := range []int32{GdArrayTypeInt64, GdArrayTypeFloat, GdArrayTypeBool, GdArrayTypeString, GdArrayTypeByte, GdArrayTypeGdObj} {
		wire, err := serializeGdArray(&GdArrayInfo{Type: typ})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := deserializeGdArray(wire)
		if err != nil || decoded.Type != typ || decoded.Size != 0 || decoded.Data == nil {
			t.Fatalf("empty type %d: %#v, %v", typ, decoded, err)
		}
		value := reflect.ValueOf(decoded.Data)
		if value.Kind() != reflect.Slice || value.IsNil() || value.Len() != 0 {
			t.Fatalf("empty type %d lost its non-nil slice: %#v", typ, decoded.Data)
		}
	}
	if f64Tof32(nil) == nil {
		t.Fatal("Web conversion must preserve its non-nil empty result")
	}
}

func TestArrayCodecRejectsMalformedPayload(t *testing.T) {
	for _, wire := range [][]byte{
		{}, {0, 0, 0, 0},
		{255, 255, 255, 255, 1, 0, 0, 0},
		{0, 0, 0, 0, 99, 0, 0, 0},
		{1, 0, 0, 0, 1, 0, 0, 0, 1},
		{1, 0, 0, 0, 4, 0, 0, 0, 9, 0, 0, 0, 1},
		{0, 0, 0, 0, 5, 0, 0, 0, 1},
	} {
		if _, err := deserializeGdArray(wire); err == nil {
			t.Fatalf("accepted malformed payload %v", wire)
		}
	}
}

func FuzzArrayCodec(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{1, 0, 0, 0, 4, 0, 0, 0, 2, 0, 0, 0, 'h', 'i'})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, wire []byte) {
		if len(wire) > 65536 {
			t.Skip()
		}
		decoded, err := deserializeGdArray(wire)
		if err != nil {
			return
		}
		encoded, err := serializeGdArray(decoded)
		if err != nil {
			t.Fatalf("decoded value cannot be encoded: %v", err)
		}
		again, err := deserializeGdArray(encoded)
		if err != nil || again.Type != decoded.Type || again.Size != decoded.Size {
			t.Fatalf("roundtrip header changed: %#v, %v", again, err)
		}
	})
}
