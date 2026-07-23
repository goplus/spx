package spx

import (
	"testing"

	"github.com/goplus/spx/v3/internal/engine"
)

func TestBaseObjGetCostumeAssetPath(t *testing.T) {
	obj := baseObj{
		costumes: []*costume{{
			path: "sprites/Eat/costume1.svg",
		}},
	}

	got := obj.getCostumeAssetPath()
	want := engine.ToAssetPath("sprites/Eat/costume1.svg")
	if got != want {
		t.Fatalf("getCostumeAssetPath() = %q, want %q", got, want)
	}
}
