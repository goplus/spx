//go:build js && wasm

package webffi

import (
	"math"
	"syscall/js"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestScalarMarshalingPreservesValues(t *testing.T) {
	for _, value := range []int64{math.MinInt64, -1, 0, 1<<54 + 3, math.MaxInt64} {
		if got := JSToGdInt(JSFromGdInt(value)); got != value {
			t.Errorf("integer round trip=%d, want %d", got, value)
		}
		object := engine.Object(value)
		if JSToGdObject(JSFromGdObj(object)) != object || JSToGdObj(JSFromGdObj(object)) != value {
			t.Errorf("object precision lost: %d", value)
		}
		low, high := JSSplitGdObj(object)
		if gdIntFromParts(low, high) != value {
			t.Errorf("split object precision lost: %d", value)
		}
	}
	for _, tc := range []struct {
		input any
		want  bool
	}{{true, true}, {false, false}, {2, true}, {0, false}, {-1, true}} {
		if got := JSToGdBool(js.ValueOf(tc.input)); got != tc.want {
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
			JSToGdBool(value)
		}()
	}
	if got := JSToGdString(JSFromGdString("héllo\x00inside\x00\x00")); got != "héllo\x00inside" {
		t.Fatalf("string terminators=%q", got)
	}
	vec2, vec3, vec4 := mathf.NewVec2(1.5, -2.25), mathf.NewVec3(1, 2, 3), mathf.NewVec4(1, 2, 3, 4)
	color := mathf.NewColor(.25, .5, .75, 1)
	rect := mathf.Rect2{Position: vec2, Size: mathf.NewVec2(40, 50)}
	if JSToGdVec2(JSFromGdVec2(vec2)) != vec2 || JSToGdVec3(JSFromGdVec3(vec3)) != vec3 || JSToGdVec4(JSFromGdVec4(vec4)) != vec4 || JSToGdColor(JSFromGdColor(color)) != color || JSToGdRect2(JSFromGdRect2(rect)) != rect {
		t.Fatal("structured value round trip changed components")
	}
	if JSToGdFloat(js.ValueOf(-1.5)) != -1.5 {
		t.Fatal("scalar conversion lost value")
	}
}
