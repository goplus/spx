package spx

import (
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
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

func TestInitSpriteCostumesConsumesPreparedLayout(t *testing.T) {
	assetPath := costumeAssetPath("sprites/hero.png")
	costumeSizeCache.Store(assetPath, mathf.NewVec2(20, 10))
	t.Cleanup(func() { costumeSizeCache.Delete(assetPath) })

	config := &coreproject.SpriteConfig{
		CostumeMPSet: &coreproject.CostumeMPSet{
			Path:             "sprites/hero.png",
			FaceRight:        90,
			BitmapResolution: 2,
			Parts: []coreproject.CostumeSetPart{
				{Nx: 1, Rect: coreproject.CostumeSetRect{W: 10, H: 10}},
				{
					Nx:    2,
					Rect:  coreproject.CostumeSetRect{W: 20, H: 10},
					Items: []coreproject.CostumeSetItem{{NamePrefix: "run", N: 2}},
				},
			},
		},
		CostumeIndex: 2,
	}
	layout, err := coreproject.PrepareCostumeLayout(config)
	if err != nil {
		t.Fatal(err)
	}

	var obj baseObj
	obj.initSpriteCostumes(config, layout)

	names := make([]string, len(obj.costumes))
	setIndexes := make([]int, len(obj.costumes))
	for i, costume := range obj.costumes {
		names[i] = costume.name
		setIndexes[i] = costume.setIndex
	}
	if want := []string{"0", "run0", "run1"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("costume names = %v, want %v", names, want)
	}
	if want := []int{0, 0, 1}; !reflect.DeepEqual(setIndexes, want) {
		t.Fatalf("costume set indexes = %v, want %v", setIndexes, want)
	}
	if obj.costumeIndex != 2 {
		t.Fatalf("costume index = %d, want 2", obj.costumeIndex)
	}
}
