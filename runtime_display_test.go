package spx

import (
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestGetCostumeRenderOffsetUsesPivot(t *testing.T) {
	costume := &costume{
		width:            200,
		height:           120,
		bitmapResolution: 2,
		center:           mathf.NewVec2(100, 60),
		pivot:            mathf.NewVec2(10, -5),
	}

	x, y := getCostumeRenderOffset(costume, costume.pivot, 1, 1)
	if x != -10 || y != 5 {
		t.Fatalf("getCostumeRenderOffset = (%v, %v), want (-10, 5)", x, y)
	}
}
