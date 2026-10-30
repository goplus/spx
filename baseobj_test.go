package spx

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
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

func TestBaseObjCostumeSelectionRejectsInvalidInputWithoutChangingState(t *testing.T) {
	obj := baseObj{costumes: []*costume{{name: "first"}, {name: "second"}}}
	for _, tt := range []struct {
		value any
		index int
		ok    bool
	}{
		{Prev, 1, true}, {Next, 0, true}, {1.9, 1, true},
		{-1, 1, false}, {2, 1, false}, {struct{}{}, 1, false},
		{"first", 0, true},
	} {
		if got := obj.goSetCostume(tt.value); got != tt.ok || obj.getCostumeIndex() != tt.index {
			t.Fatalf("select %v: ok=%v index=%d, want %v/%d", tt.value, got, obj.getCostumeIndex(), tt.ok, tt.index)
		}
		if obj.getCostumeName() != obj.costumes[tt.index].name {
			t.Fatal("selected costume name does not match its index")
		}
	}
}

func TestBaseObjInitialCostumeIndexFallsBackToFirst(t *testing.T) {
	for _, index := range []int{-1, 2} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			cfg := &coreproject.CostumeConfig{Name: "first", ImageWidth: 20, ImageHeight: 10}
			var sprite, stage baseObj
			sprite.init([]*coreproject.CostumeConfig{cfg}, index)
			stage.initBackdrops([]*coreproject.BackdropConfig{{CostumeConfig: *cfg}}, index)
			for _, obj := range []*baseObj{&sprite, &stage} {
				if obj.costumeIndex != 0 || obj.getCostumeName() != "first" || !obj.runtimeState.isCostumeDirty {
					t.Fatalf("initial costume = %d/%q, dirty=%v", obj.costumeIndex, obj.getCostumeName(), obj.runtimeState.isCostumeDirty)
				}
			}
		})
	}
}

func TestBaseObjInitializesSingleCostumeSetWithRect(t *testing.T) {
	config := &coreproject.SpriteConfig{
		CostumeIndex: 9,
		CostumeSet: &coreproject.CostumeSet{
			Path: "hero.png", ImageWidth: 100, ImageHeight: 40, Nx: 2,
			BitmapResolution: 2, FaceRight: 90,
			Rect: &coreproject.CostumeSetRect{X: 20, Y: 10, W: 40, H: 20},
		},
	}
	layout, err := coreproject.PrepareCostumeLayout(config)
	if err != nil {
		t.Fatal(err)
	}
	var obj baseObj
	obj.initSpriteCostumes(config, layout)
	if !obj.runtimeState.isCostumeSet || len(obj.costumes) != 2 || obj.costumeIndex != 0 {
		t.Fatalf("costume set = %v, frames = %d, initial = %d", obj.runtimeState.isCostumeSet, len(obj.costumes), obj.costumeIndex)
	}
	for i, costume := range obj.costumes {
		if costume.frame.PosX != 20+i*20 || costume.frame.PosY != 10 || costume.frame.Width != 20 || costume.frame.Height != 20 || costume.frame.BitmapResolution != 2 || costume.faceRight != 90 {
			t.Fatalf("frame %d = %+v", i, costume)
		}
	}
}

func TestBaseObjGraphicEffectsAccumulateBeforeProxyExists(t *testing.T) {
	var obj baseObj
	obj.changeGraphicEffect(ColorEffect, 25)
	obj.changeGraphicEffect(ColorEffect, -75)
	if obj.greffUniforms[ColorEffect] != -50 {
		t.Fatalf("accumulated color effect = %v, want -50", obj.greffUniforms[ColorEffect])
	}
	if got := normalizeEffectValue(ColorEffect, -50); got != 0.75 {
		t.Fatalf("wrapped color effect = %v, want 0.75", got)
	}
	if got := normalizeEffectValue(EffectKind(999), 42); got != 42 {
		t.Fatalf("unknown effect = %v, want unchanged 42", got)
	}
	obj.setMaterialParams("color", 0.75, true)
	obj.setMaterialParamsVec4("remap", mathf.Vec4{X: 1}, false)
	if obj.runtimeState.hasShader {
		t.Fatal("unbound material update created a shader")
	}
}

func TestBaseObjGraphicEffectLimits(t *testing.T) {
	for _, tt := range []struct {
		name                string
		kind                EffectKind
		value, delta        float64
		wantSet, wantChange float64
	}{
		{"brightness below minimum", BrightnessEffect, -125, 5, -100, -95},
		{"brightness above maximum", BrightnessEffect, 125, -5, 100, 95},
		{"brightness crosses minimum", BrightnessEffect, -95, -10, -95, -100},
		{"brightness crosses maximum", BrightnessEffect, 95, 10, 95, 100},
		{"brightness fractional", BrightnessEffect, -12.5, -0.5, -12.5, -13},
		{"ghost below minimum", GhostEffect, -25, 5, 0, 5},
		{"ghost above maximum", GhostEffect, 125, -5, 100, 95},
		{"ghost crosses minimum", GhostEffect, 5, -10, 5, 0},
		{"ghost crosses maximum", GhostEffect, 95, 10, 95, 100},
		{"ghost fractional", GhostEffect, 12.5, 0.5, 12.5, 13},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var obj baseObj
			obj.setGraphicEffect(tt.kind, tt.value)
			if got := obj.greffUniforms[tt.kind]; got != tt.wantSet {
				t.Errorf("effect after set = %v, want %v", got, tt.wantSet)
			}
			obj.changeGraphicEffect(tt.kind, tt.delta)
			if got := obj.greffUniforms[tt.kind]; got != tt.wantChange {
				t.Errorf("effect after change = %v, want %v", got, tt.wantChange)
			}
		})
	}
}

func TestBaseObjOtherGraphicEffectsRemainUnbounded(t *testing.T) {
	for _, kind := range []EffectKind{ColorEffect, FishEyeEffect, WhirlEffect, PixelateEffect, MosaicEffect} {
		t.Run(kind.String(), func(t *testing.T) {
			var obj baseObj
			obj.setGraphicEffect(kind, 1000)
			if got := obj.greffUniforms[kind]; got != 1000 {
				t.Fatalf("effect after set = %v, want 1000", got)
			}
			obj.changeGraphicEffect(kind, -2000)
			if got := obj.greffUniforms[kind]; got != -1000 {
				t.Fatalf("effect after change = %v, want -1000", got)
			}
		})
	}
}

func TestInitSpriteCostumesConsumesPreparedLayout(t *testing.T) {
	assetPath := costumeAssetPath("sprites/hero.png")
	costumeSizeCache.Store(costumeSizeKey{path: assetPath}, mathf.NewVec2(20, 10))
	t.Cleanup(func() { costumeSizeCache.Delete(costumeSizeKey{path: assetPath}) })

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
	positions := make([]int, len(obj.costumes))
	for i, costume := range obj.costumes {
		names[i] = costume.name
		positions[i] = costume.frame.PosX
		if !costume.isAtlas() {
			t.Fatalf("costume %d is not an atlas frame", i)
		}
	}
	if want := []string{"0", "run0", "run1"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("costume names = %v, want %v", names, want)
	}
	if want := []int{0, 0, 10}; !reflect.DeepEqual(positions, want) {
		t.Fatalf("costume frame positions = %v, want %v", positions, want)
	}
	if obj.costumeIndex != 2 {
		t.Fatalf("costume index = %d, want 2", obj.costumeIndex)
	}
}
