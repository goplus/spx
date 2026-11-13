package spx

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/collision"
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

func TestToBitmapResolution(t *testing.T) {
	if got := normalizeBitmapResolution(0); got != defaultBitmapResolution {
		t.Fatalf("normalizeBitmapResolution(0) = %d, want %d", got, defaultBitmapResolution)
	}
	if got := normalizeBitmapResolution(2); got != 2 {
		t.Fatalf("normalizeBitmapResolution(2) = %d, want 2", got)
	}
}

func TestDefaultAtlasUV(t *testing.T) {
	if got := defaultAtlasUV(); got != mathf.NewVec4(0, 0, fullUVRange, fullUVRange) {
		t.Fatalf("defaultAtlasUV() = %v, want full UV range", got)
	}
}

func TestCalculateAtlasUV(t *testing.T) {
	got := calculateAtlasUV(10, 20, 30, 40, mathf.NewVec2(100, 200))
	want := mathf.NewVec4(0.1, 0.1, 0.3, 0.2)
	if got != want {
		t.Fatalf("calculateAtlasUV() = %v, want %v", got, want)
	}
}

func TestResolveImageSize(t *testing.T) {
	got := resolveCostumeImageSize(32, 16, "ignored", func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called when config size is present")
		return mathf.Vec2{}
	})
	if got != (mathf.Vec2{X: 32, Y: 16}) {
		t.Fatalf("resolveCostumeImageSize configured = %v, want {32 16}", got)
	}

	fallbackCalled := false
	got = resolveCostumeImageSize(0, 0, "path", func(path string) mathf.Vec2 {
		fallbackCalled = true
		if path != "path" {
			t.Fatalf("fallback path = %q, want %q", path, "path")
		}
		return mathf.NewVec2(8, 4)
	})
	if !fallbackCalled {
		t.Fatal("fallback was not called")
	}
	if got != mathf.NewVec2(8, 4) {
		t.Fatalf("resolveCostumeImageSize fallback = %v, want {8 4}", got)
	}
}

func TestNewSizedFrame(t *testing.T) {
	got := newSizedCostumeFrame(10, 20)
	if got.Width != 10 || got.Height != 20 {
		t.Fatalf("newSizedCostumeFrame size = (%d, %d), want (10, 20)", got.Width, got.Height)
	}
	if got.Center != mathf.NewVec2(5, 10) {
		t.Fatalf("newSizedCostumeFrame center = %v, want {5 10}", got.Center)
	}
}

func TestNewAtlasFrame(t *testing.T) {
	tests := []struct {
		name       string
		nx, index  int
		rect       [4]float64
		width      int
		height     int
		posX, posY int
	}{
		{"full image", 5, 2, [4]float64{}, 20, 20, 40, 0},
		{"zero columns", 0, 0, [4]float64{}, 100, 20, 0, 0},
		{"negative columns", -2, 0, [4]float64{}, 100, 20, 0, 0},
		{"fractional region", 3, 1, [4]float64{7.5, 3.5, 31.5, 11.5}, 10, 11, 17, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			img := &costumeSetImage{
				width: 100, height: 20, path: "path", nx: test.nx,
				rc: coreproject.CostumeSetRect{X: test.rect[0], Y: test.rect[1], W: test.rect[2], H: test.rect[3]},
			}
			got := newAtlasCostumeFrame(img, test.index, 2, func(string) mathf.Vec2 {
				t.Fatal("fallback should not be called")
				return mathf.Vec2{}
			})
			want := costumeFrame{
				Width: test.width, Height: test.height, BitmapResolution: 2,
				ImageSize: mathf.NewVec2(100, 20),
				Center:    mathf.NewVec2(float64(test.width)/2, float64(test.height)/2),
				PosX:      test.posX, PosY: test.posY,
				AtlasUVRect: mathf.NewVec4(float64(test.posX)/100, float64(test.posY)/20, float64(test.width)/100, float64(test.height)/20),
			}
			if got != want {
				t.Fatalf("newAtlasCostumeFrame() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestNewStandaloneFrame(t *testing.T) {
	got := newStandaloneCostumeFrame(&coreproject.CostumeConfig{ImageWidth: 32, ImageHeight: 16, Path: "path"}, func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called")
		return mathf.Vec2{}
	})
	if got.Width != 32 || got.Height != 16 {
		t.Fatalf("newStandaloneCostumeFrame size = (%d, %d), want (32, 16)", got.Width, got.Height)
	}
	if got.BitmapResolution != defaultBitmapResolution {
		t.Fatalf("newStandaloneCostumeFrame bitmap resolution = %d, want %d", got.BitmapResolution, defaultBitmapResolution)
	}
	if got.Center != mathf.NewVec2(16, 8) {
		t.Fatalf("newStandaloneCostumeFrame center = %v, want {16 8}", got.Center)
	}
}

func TestCostumeFrameFallback(t *testing.T) {
	calls := 0
	fallback := func(path string) mathf.Vec2 {
		calls++
		if path != "image.svg" {
			t.Fatalf("fallback path = %q, want image.svg", path)
		}
		return mathf.NewVec2(31.5, 17.5)
	}
	img := costumeSetImage{path: "image.svg", nx: -1}
	atlas := newAtlasCostumeFrame(&img, 1, 0, fallback)
	if atlas.Width != 31 || atlas.Height != 17 || atlas.PosX != 31 || atlas.BitmapResolution != 0 || img.nx != -1 {
		t.Fatalf("atlas fallback = %+v, columns = %d", atlas, img.nx)
	}
	config := coreproject.CostumeConfig{Path: "image.svg"}
	standalone := newStandaloneCostumeFrame(&config, fallback)
	if standalone.ImageSize != mathf.NewVec2(31.5, 17.5) || standalone.Center != mathf.NewVec2(15.75, 8.75) || standalone.BitmapResolution != 1 || config.BitmapResolution != 0 {
		t.Fatalf("standalone fallback = %+v, configured resolution = %d", standalone, config.BitmapResolution)
	}
	if calls != 2 {
		t.Fatalf("fallback calls = %d, want 2", calls)
	}
}

func TestSpriteSetCostume(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		costume                 SpriteCostumeName
		defaultIndex, wantIndex int
		wantVersion             uint64
		wantDirty               bool
	}{
		{name: "missing name is no-op", costume: "missing", wantIndex: 1},
		{name: "valid name updates state", costume: "idle", defaultIndex: 1, wantVersion: 1, wantDirty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestRenderSprite()
			sprite.spriteState.defaultCostumeIndex = tt.defaultIndex

			sprite.setCostume(tt.costume)

			if got := sprite.costumeIndex; got != tt.wantIndex {
				t.Errorf("costumeIndex = %d, want %d", got, tt.wantIndex)
			}
			if got := sprite.spriteState.defaultCostumeIndex; got != 0 {
				t.Errorf("DefaultCostumeIndex = %d, want 0", got)
			}
			if got := uint64(sprite.spriteState.dirtyVersion); got != tt.wantVersion {
				t.Errorf("DirtyVersion = %d, want %d", got, tt.wantVersion)
			}
			if got := sprite.runtimeState.isCostumeDirty; got != tt.wantDirty {
				t.Errorf("IsCostumeDirty = %v, want %v", got, tt.wantDirty)
			}
		})
	}
}

func TestResolveCostumeIndex(t *testing.T) {
	sprite := newTestRenderSprite()

	tests := []struct {
		name    string
		costume string
		want    int
	}{
		{name: "name", costume: "run", want: 1},
		{name: "zero", costume: "0", want: -1},
		{name: "in range", costume: "2", want: 1},
		{name: "wrap", costume: "3", want: 0},
		{name: "wrap to zero", costume: "4", want: 1},
		{name: "negative whole cycle", costume: "-1", want: 0},
		{name: "negative remainder", costume: "-2", want: -1},
		{name: "explicit positive sign", costume: "+2", want: 1},
		{name: "minimum integer wraps subtraction", costume: strconv.Itoa(math.MinInt), want: 1},
		{name: "maximum integer", costume: strconv.Itoa(math.MaxInt), want: 0},
		{name: "integer overflow", costume: strconv.Itoa(math.MaxInt) + "0", want: -1},
		{name: "fraction is not an integer", costume: "2.5", want: -1},
		{name: "whitespace is not trimmed", costume: " 2 ", want: -1},
		{name: "compound signs are not accepted", costume: "--2", want: -1},
		{name: "invalid", costume: "missing", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sprite.ResolveCostumeIndex(tt.costume); got != tt.want {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want %d", tt.costume, got, tt.want)
			}
		})
	}
}

func TestResolveCostumeIndexPrefersNames(t *testing.T) {
	for _, name := range []string{"1", "0", "-2", " 2 ", strconv.Itoa(math.MaxInt) + "0"} {
		t.Run(name, func(t *testing.T) {
			sprite := newTestRenderSprite()
			sprite.costumes[1].name = name
			if got := sprite.ResolveCostumeIndex(name); got != 1 {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want named costume at 1", name, got)
			}
		})
	}
}

func TestResolveCostumeIndexWithNoCostumes(t *testing.T) {
	sprite := &SpriteImpl{}

	for _, tt := range []struct {
		costume string
		want    int
	}{
		{"3", 2},
		{"0", -1},
		{"-2", -3},
		{strconv.Itoa(math.MinInt), math.MaxInt},
		{"missing", -1},
	} {
		t.Run(tt.costume, func(t *testing.T) {
			if got := sprite.ResolveCostumeIndex(tt.costume); got != tt.want {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want %d", tt.costume, got, tt.want)
			}
		})
	}
}

func TestGetCostumeRenderOffsetUsesPivot(t *testing.T) {
	costume := newBackdropCostume(&coreproject.BackdropConfig{
		CostumeConfig: coreproject.CostumeConfig{
			ImageWidth: 200, ImageHeight: 120, BitmapResolution: 2,
			X: 100, Y: 60,
		},
		Pivot: mathf.NewVec2(10, -5),
	})

	x, y := getCostumeRenderOffset(costume, costume.pivot, 1, 1)
	if x != -10 || y != 5 {
		t.Fatalf("getCostumeRenderOffset = (%v, %v), want (-10, 5)", x, y)
	}
}

func TestCostumeRenderAnchorConvertsAssetCoordinatesToSPX(t *testing.T) {
	costume := newCostume(&coreproject.CostumeConfig{
		ImageWidth: 201.5, ImageHeight: 121.25, BitmapResolution: 2,
		X: 80, Y: 40,
	})

	want := mathf.NewVec2(-10.375, 10.3125)
	if got := costume.renderAnchorInSPX(); got != want {
		t.Fatalf("renderAnchorInSPX = %v, want %v", got, want)
	}
	if width, height := costume.sizeInSPX(); width != 100.75 || height != 60.625 {
		t.Fatalf("logical size = (%v, %v), want (100.75, 60.625)", width, height)
	}
	if costume.isAtlas() {
		t.Fatal("standalone costume is marked as an atlas")
	}
}

func TestCostumeCenterInAssetNormalizesStaleCenterOnEmptyFrame(t *testing.T) {
	// An empty vector costume is imported as a 1x1 frame, but the source project can
	// still declare a rotation center sized for its original canvas.
	empty := costumeFrame{Width: 1, Height: 1, BitmapResolution: 1}
	if got := costumeCenterInAsset(empty, mathf.NewVec2(240, 180)); got != (mathf.NewVec2(0.5, 0.5)) {
		t.Fatalf("empty frame center = %v, want {0.5 0.5}", got)
	}
	costume := newCostume(&coreproject.CostumeConfig{
		ImageWidth: 1, ImageHeight: 1, BitmapResolution: 1, X: 240, Y: 180,
	})
	if got := costume.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want {0 0}", got)
	}

	filled := costumeFrame{Width: 960, Height: 720, BitmapResolution: 2}
	if got := costumeCenterInAsset(filled, mathf.NewVec2(480, 360)); got != (mathf.NewVec2(480, 360)) {
		t.Fatalf("filled frame center = %v, want {480 360}", got)
	}

	// A 1xN costume has real pixels and may anchor at an end rather than its center.
	thin := costumeFrame{Width: 1, Height: 200, BitmapResolution: 1}
	if got := costumeCenterInAsset(thin, mathf.NewVec2(0.5, 200)); got != (mathf.NewVec2(0.5, 200)) {
		t.Fatalf("thin frame center = %v, want {0.5 200}", got)
	}
}

func TestSizedCostumeUsesImageCenter(t *testing.T) {
	costume := newCostumeWithSize(7, 5)
	if width, height := costume.displaySize(); width != 7 || height != 5 {
		t.Fatalf("sized costume = (%d, %d), want (7, 5)", width, height)
	}
	if got := costume.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want centered origin", got)
	}
	if costume.isAtlas() {
		t.Fatal("sized costume is marked as an atlas")
	}
}

func TestAtlasCostumePreservesFrameGeometry(t *testing.T) {
	c := newCostumeWith("frame1", &costumeSetImage{
		path: "atlas.png", width: 256, height: 128, nx: 2,
		rc: coreproject.CostumeSetRect{X: 16, Y: 8, W: 86, H: 43},
	}, 90, 1, 2)
	obj := baseObj{costumes: []*costume{c}}
	if !c.isAtlas() {
		t.Fatal("atlas costume is marked as a standalone image")
	}
	if got, want := obj.getCostumeAtlasRegion(), mathf.NewRect2(59, 8, 43, 43); got != want {
		t.Fatalf("atlas region = %v, want %v", got, want)
	}
	if got, want := c.frame.AtlasUVRect, mathf.NewVec4(59.0/256, 8.0/128, 43.0/256, 43.0/128); got != want {
		t.Fatalf("atlas UV remap = %v, want %v", got, want)
	}
	if width, height := c.displaySize(); width != 21 || height != 21 {
		t.Fatalf("integer size = (%d, %d), want (21, 21)", width, height)
	}
	if width, height := c.sizeInSPX(); width != 21.5 || height != 21.5 {
		t.Fatalf("logical size = (%v, %v), want (21.5, 21.5)", width, height)
	}
	if got := c.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want centered origin", got)
	}
}

func TestWorldRenderOffsetAppliesRootFlipAndRotation(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.scale = 2
	sprite.transform().rotationStyle = LeftRight
	sprite.transform().direction = -30

	x, y := getWorldRenderOffset(sprite)
	if x != -74 || y != -48 {
		t.Fatalf("left-right world render offset = (%v, %v), want (-74, -48)", x, y)
	}

	sprite.runtimeState.scale = 1
	sprite.transform().rotationStyle = Normal
	sprite.transform().direction = 45
	x, y = getWorldRenderOffset(sprite)
	wantX := 61 / math.Sqrt2
	wantY := 13 / math.Sqrt2
	if !(math.Abs(x-wantX) <= 1e-9 && math.Abs(y-wantY) <= 1e-9) {
		t.Fatalf("rotated world render offset = (%v, %v), want (%v, %v)", x, y, wantX, wantY)
	}
}

func TestLegacyLeftRightRotationStyleFlipsNegativeHeading(t *testing.T) {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		Heading:       -9,
		RotationStyle: "leftRight",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.costumes = []*costume{{frame: costumeFrame{BitmapResolution: 1}}}

	rotation, scaleX, scaleY := getRenderRotationAndScale(sprite)
	if rotation != 0 || scaleX != -1 || scaleY != 1 {
		t.Fatalf("legacy leftRight transform = (%v, %v, %v), want (0, -1, 1)", rotation, scaleX, scaleY)
	}
}

func TestPhysicsShapePivotSeparatesAutoAndExplicitShapes(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.scale = 2

	config := colliderConfig{Pivot: mathf.NewVec2(3, 4), Type: collision.ColliderRect}
	if got, want := config.shapePivot(sprite), mathf.NewVec2(6, 8); got != want {
		t.Fatalf("explicit shape pivot = %v, want %v", got, want)
	}

	config.Type = collision.ColliderAuto
	if got, want := config.shapePivot(sprite), mathf.NewVec2(80, -40); got != want {
		t.Fatalf("auto shape pivot = %v, want %v", got, want)
	}
}

func TestSpriteRenderingOverloadsAndDebugPreserveState(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	installAskTestUiMgr(t)
	sprite := newCloneLimitSprite(game, "actor")
	sprite.costumes = []*costume{{name: "first"}, {name: "second"}, {name: "third"}}
	for _, tt := range []struct {
		run  func()
		want string
	}{
		{func() { sprite.SetCostume__1(1) }, "second"},
		{func() { sprite.SetCostume__2(2) }, "third"},
		{func() { sprite.SetCostume__3(Next) }, "first"},
	} {
		tt.run()
		if sprite.CostumeName() != tt.want {
			t.Fatalf("costume=%q, want %q", sprite.CostumeName(), tt.want)
		}
	}
	if sprite.ResolveCostumeIndex("second") != 1 {
		t.Fatal("debug costume lookup changed indexing")
	}
	sprite.SetGraphicEffect(GhostEffect, 20)
	sprite.ChangeGraphicEffect(GhostEffect, 30)
	if sprite.greffUniforms[GhostEffect] != 50 {
		t.Fatalf("ghost effect = %v, want 50", sprite.greffUniforms[GhostEffect])
	}
	sprite.ClearGraphicEffects()
	if sprite.greffUniforms[GhostEffect] != 0 {
		t.Fatalf("effects remained after clear: %v", sprite.greffUniforms)
	}
	sprite.Show()
	sprite.Say__0("visible bubble")
	sprite.Hide()
	if sprite.Visible() || sprite.bubble().textObj != nil {
		t.Fatal("hiding sprite did not hide and clear its bubble")
	}
	sprite.SetLayer__0(Front)
	if game.shapeMgr.findShapeIndex(&sprite.SpriteImpl) != len(game.shapeMgr.items)-1 {
		t.Fatal("front layer overload did not place sprite last")
	}
}

func TestSpriteGhostFadeOutAfterExtendedFadeIn(t *testing.T) {
	// CoalDestroyer's Play button keeps fading in until clicked, then fades out.
	// See https://github.com/goplus/sb2xbp/issues/860.
	var sprite SpriteImpl
	sprite.SetGraphicEffect(GhostEffect, 100)
	for range 60 {
		sprite.ChangeGraphicEffect(GhostEffect, -5)
	}
	if got := sprite.greffUniforms[GhostEffect]; got != 0 {
		t.Errorf("ghost after extended fade-in = %v, want 0", got)
	}
	for range 20 {
		sprite.ChangeGraphicEffect(GhostEffect, 5)
	}
	if !sprite.isFullyGhosted() {
		t.Fatalf("sprite remains visible after fade-out: ghost = %v", sprite.greffUniforms[GhostEffect])
	}
}

func newTestRenderSprite() *SpriteImpl {
	return &SpriteImpl{
		name: "TestSprite",
		baseObj: baseObj{
			costumes: []*costume{
				{name: "idle"},
				{name: "run"},
			},
			costumeIndex: 1,
		},
	}
}

func newRenderOffsetTestSprite() *SpriteImpl {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.runtimeState.scale = 1
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		atlas: true,
		frame: costumeFrame{
			Center:           mathf.NewVec2(10, 20),
			BitmapResolution: 1,
			Width:            100,
			Height:           80,
		},
	}}
	return sprite
}

func TestAnimationPayloadBoundsError(t *testing.T) {
	tests := []struct {
		name     string
		from, to int
		costumes []*costume
		want     string
	}{
		{"empty", -1, 0, nil, "createAnimation: no costumes configured"},
		{"negative from", -1, 0, []*costume{{}}, "createAnimation: frame index out of bounds (from=-1, to=0, costumes=1)"},
		{"negative to", 0, -1, []*costume{{}}, "createAnimation: frame index out of bounds (from=0, to=-1, costumes=1)"},
		{"from past end", 1, 0, []*costume{{}}, "createAnimation: frame index out of bounds (from=1, to=0, costumes=1)"},
		{"to past end", 0, 1, []*costume{{}}, "createAnimation: frame index out of bounds (from=0, to=1, costumes=1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, isAtlas := range []bool{false, true} {
				got, bitmap, err := buildAnimationPayloadJSON(test.costumes, test.from, test.to, isAtlas)
				if err == nil || err.Error() != test.want || got != "" || bitmap != 0 {
					t.Fatalf("buildAnimationPayloadJSON(atlas=%v) = (%q, %d, %v), want empty/0/%q", isAtlas, got, bitmap, err, test.want)
				}
			}
		})
	}
}

func TestAnimationPayloadAtlasIgnoresNormalMetadata(t *testing.T) {
	costumes := []*costume{
		{path: "sprites/base.png", frame: costumeFrame{Width: 90, Height: 100}},
		{
			path: "sprites/other.png",
			frame: costumeFrame{
				PosX: 10, PosY: 20, Width: 30, Height: 40,
				BitmapResolution: 8, Center: mathf.NewVec2(math.NaN(), 0), ImageSize: mathf.NewVec2(0, math.Inf(1)),
			},
		},
		{path: "sprites/another.png", frame: costumeFrame{PosX: 50, PosY: 60, Width: 70, Height: 80}},
	}
	const base = `{"x":0,"y":0,"w":90,"h":100,"offset":[0,0]}`
	const first = `{"x":10,"y":20,"w":30,"h":40,"offset":[0,0]}`
	const second = `{"x":50,"y":60,"w":70,"h":80,"offset":[0,0]}`
	tests := []struct {
		name     string
		from, to int
		frames   string
	}{
		{"single", 1, 1, "[" + first + "]"},
		{"forward", 1, 2, "[" + first + "," + second + "]"},
		{"reverse", 2, 1, "[" + second + "," + first + "]"},
		{"reverse through zero", 2, 0, "[" + second + "," + first + "," + base + "]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := buildAnimationPayloadJSON(costumes, test.from, test.to, true)
			want := `{"base_path":` + strconv.Quote(engine.ToAssetPath(costumes[0].path)) + `,"frames":` + test.frames + `,"max_bitmap":1}`
			if err != nil || got != want || bitmap != 1 {
				t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want (%q, 1, nil)", got, bitmap, err, want)
			}
		})
	}
}

func TestAnimationPayloadRejectsNonFiniteNormalOffset(t *testing.T) {
	got, bitmap, err := buildAnimationPayloadJSON([]*costume{{frame: costumeFrame{Center: mathf.NewVec2(math.NaN(), 0)}}}, 0, 0, false)
	var unsupported *json.UnsupportedValueError
	if !errors.As(err, &unsupported) || !strings.HasPrefix(err.Error(), "createAnimation: failed to marshal animation payload:") || got != "" || bitmap != 0 {
		t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want empty/0/wrapped JSON error", got, bitmap, err)
	}
}

func TestAnimationPayloadNormalPreservesDescriptorGeometry(t *testing.T) {
	costumes := []*costume{
		{
			path: "sprites/fractional.svg",
			frame: costumeFrame{
				ImageSize: mathf.NewVec2(55.75, 54.25), Center: mathf.NewVec2(52, 89),
				// Normal payloads use logical ImageSize, not atlas-only geometry.
				Width: 999, Height: 999, PosX: 10, PosY: 20,
				AtlasUVRect: mathf.NewVec4(math.NaN(), 0, 0, 0),
			},
		},
		{
			path: "sprites/centered.png",
			frame: costumeFrame{
				BitmapResolution: 2, ImageSize: mathf.NewVec2(20, 30), Center: mathf.NewVec2(10, 15),
			},
		},
	}
	first := `{"path":` + strconv.Quote(engine.ToAssetPath(costumes[0].path)) + `,"offset":[24.125,-61.875],"bitmap":1}`
	second := `{"path":` + strconv.Quote(engine.ToAssetPath(costumes[1].path)) + `,"offset":[0,-0],"bitmap":2}`
	for _, test := range []struct {
		name     string
		from, to int
		frames   string
		bitmap   int
	}{
		{"single", 0, 0, "[" + first + "]", 1},
		{"forward", 0, 1, "[" + first + "," + second + "]", 2},
		{"reverse", 1, 0, "[" + second + "," + first + "]", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := buildAnimationPayloadJSON(costumes, test.from, test.to, false)
			want := `{"frames":` + test.frames + `,"max_bitmap":` + strconv.Itoa(test.bitmap) + `}`
			if err != nil || got != want || bitmap != test.bitmap {
				t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want (%q, %d, nil)", got, bitmap, err, want, test.bitmap)
			}
		})
	}
}
