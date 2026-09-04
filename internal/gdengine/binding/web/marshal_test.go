//go:build js && wasm

package webffi

import (
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

func TestArrayToGdArrayInfoConvertsFloat64(t *testing.T) {
	data := []float64{1.25, -2.5}
	got := arrayToGdArrayInfo(data)
	if got == nil || got.Type != GdArrayTypeFloat || got.Size != int32(len(data)) {
		t.Fatalf("arrayToGdArrayInfo(%v) = %#v", data, got)
	}
	converted, ok := got.Data.([]float32)
	if !ok || len(converted) != len(data) || converted[0] != 1.25 || converted[1] != -2.5 {
		t.Fatalf("converted float64 payload = %#v", got.Data)
	}
}
