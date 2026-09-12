//go:build js && wasm

package webffi

import (
	"math"
	"reflect"
	"syscall/js"
	"testing"
)

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
}
