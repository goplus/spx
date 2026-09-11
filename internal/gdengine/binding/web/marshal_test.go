//go:build js && wasm

package webffi

import (
	"testing"
)

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
