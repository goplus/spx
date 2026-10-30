/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	assetutil "github.com/goplus/spx/v4/internal/assets"
	internalaudio "github.com/goplus/spx/v4/internal/audio"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	internalengine "github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type spyPenMgr struct {
	canvasWidth      int64
	canvasHeight     int64
	canvasCalls      int
	createCalls      int
	moveCalls        int
	penDownCalls     int
	penDownModes     []bool
	penUpCalls       int
	setColorCalls    int
	setSizeCalls     int
	stampSpriteCalls int
	lastMove         mathf.Vec2
	lastStampSprite  engine.Object
	spriteMgr        *spyStampSpriteMgr
	onStamp          func()
	batchCalls       int
	batches          [][]float32
	events           []string
}

type spyStampSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	position     mathf.Vec2
	rotation     float64
	scale        mathf.Vec2
	renderOffset mathf.Vec2
	renderScale  mathf.Vec2
	visible      bool
	texturePath  string
	atlasRegion  mathf.Rect2
	effects      map[string]float64
}

type penTestSprite struct {
	SpriteImpl
}

type penCanvasTilemapMgr struct {
	engine.ITilemapMgr
	events *[]string
}

type penCanvasCameraMgr struct {
	engine.ICameraMgr
	limits [4]int64
	calls  int
}

type penCanvasAudioBackend struct {
	internalaudio.Backend
}

func TestPenComponentInitializesDefaultColorComponents(t *testing.T) {
	sprite := newPenTestSprite()
	pen := sprite.pen()

	wantColor := mathf.NewColorRGBAi(66, 133, 244, 255)
	if !samePenColor(pen.penColor, wantColor) {
		t.Fatalf("penColor = %v, want %v", pen.penColor, wantColor)
	}

	h, s, v := wantColor.ToHSV()
	assertNearlyEqualPenValue(t, "penHue", pen.penHue, hueToPercent(h))
	assertNearlyEqualPenValue(t, "penSaturation", pen.penSaturation, normalizedToPercent(s))
	assertNearlyEqualPenValue(t, "penBrightness", pen.penBrightness, normalizedToPercent(v))
	assertNearlyEqualPenValue(t, "penTransparency", pen.penTransparency, alphaToTransparency(wantColor.A))
}

func TestPenComponentUsesTransparencySemantics(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		wantAlpha float64
	}{
		{name: "opaque", value: 0, wantAlpha: 1},
		{name: "half transparent", value: 50, wantAlpha: 0.5},
		{name: "fully transparent", value: 100, wantAlpha: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSpyPenMgr(t)
			sprite := newPenTestSprite()

			sprite.pen().setPenColorParam(PenTransparency, tt.value)

			assertNearlyEqualPenValue(t, "penTransparency", sprite.pen().penTransparency, tt.value)
			assertNearlyEqualPenValue(t, "alpha", sprite.pen().penColor.A, tt.wantAlpha)
		})
	}
}

func TestPenComponentDynamicTransparencyChangeUsesSemantics(t *testing.T) {
	setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	kind := PenColorParamFromString("transparency")

	sprite.pen().setPenColorParam(kind, 25)
	sprite.pen().changePenColor(kind, 25)

	assertNearlyEqualPenValue(t, "penTransparency", sprite.pen().penTransparency, 50)
	assertNearlyEqualPenValue(t, "alpha", sprite.pen().penColor.A, 0.5)

	sprite.pen().changePenColor(kind, 100)
	assertNearlyEqualPenValue(t, "clamped penTransparency", sprite.pen().penTransparency, 100)
	assertNearlyEqualPenValue(t, "clamped alpha", sprite.pen().penColor.A, 0)
}

func TestPenComponentSetPenColorSyncsTransparency(t *testing.T) {
	setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenColor(HSBA(20, 80, 90, 25))

	assertNearlyEqualPenValue(t, "penTransparency", sprite.pen().penTransparency, 75)
	assertNearlyEqualPenValue(t, "alpha", sprite.pen().penColor.A, 0.25)
}

func TestPenComponentRepeatedPenDownDrawsAtCurrentPosition(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().penDown()
	sprite.pen().penDown()

	if spy.createCalls != 1 {
		t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
	}
	if spy.penDownCalls != 2 {
		t.Fatalf("PenDown calls = %d, want 2", spy.penDownCalls)
	}
	if spy.moveCalls != 2 {
		t.Fatalf("MovePenTo calls = %d, want 2", spy.moveCalls)
	}
	if spy.setSizeCalls != 1 {
		t.Fatalf("SetPenSizeTo calls = %d, want 1", spy.setSizeCalls)
	}
	if spy.setColorCalls != 1 {
		t.Fatalf("SetPenColorTo calls = %d, want 1", spy.setColorCalls)
	}
}

func TestPenComponentPenUpDoesNotAllocateOrRepeat(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().penUp()
	if spy.createCalls != 0 {
		t.Fatalf("CreatePen calls after idle PenUp = %d, want 0", spy.createCalls)
	}
	if spy.penUpCalls != 0 {
		t.Fatalf("PenUp calls after idle PenUp = %d, want 0", spy.penUpCalls)
	}

	sprite.pen().penDown()
	sprite.pen().penUp()
	sprite.pen().penUp()

	if spy.penUpCalls != 1 {
		t.Fatalf("PenUp calls after repeated PenUp = %d, want 1", spy.penUpCalls)
	}
}

func TestPenComponentIgnoresRepeatedPenStyleValues(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenSize(10)
	sprite.pen().setPenSize(10)
	sprite.pen().setPenColor(HSB(85, 33, 100))
	sprite.pen().setPenColor(HSB(85, 33, 100))

	if spy.setSizeCalls != 1 {
		t.Fatalf("SetPenSizeTo calls = %d, want 1", spy.setSizeCalls)
	}
	if spy.setColorCalls != 1 {
		t.Fatalf("SetPenColorTo calls = %d, want 1", spy.setColorCalls)
	}
}

func TestPenComponentDefaultPenSizeStillMaterializesPen(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenSize(1)

	if spy.createCalls != 1 {
		t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
	}
	if spy.setSizeCalls != 1 {
		t.Fatalf("SetPenSizeTo calls = %d, want 1", spy.setSizeCalls)
	}
}

func TestPenComponentDefaultPenColorStillMaterializesPen(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	defaultColor := toSpxColor(sprite.pen().penColor)
	sprite.pen().setPenColor(defaultColor)

	if spy.createCalls != 1 {
		t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
	}
	if spy.setColorCalls != 1 {
		t.Fatalf("SetPenColorTo calls = %d, want 1", spy.setColorCalls)
	}
}

func TestPenComponentDefaultHSVStillMaterializesPen(t *testing.T) {
	tests := []struct {
		name  string
		kind  PenColorParam
		value func(*penComponent) float64
	}{
		{name: "hue", kind: PenHue, value: func(p *penComponent) float64 { return p.penHue }},
		{name: "saturation", kind: PenSaturation, value: func(p *penComponent) float64 { return p.penSaturation }},
		{name: "brightness", kind: PenBrightness, value: func(p *penComponent) float64 { return p.penBrightness }},
		{name: "transparency", kind: PenTransparency, value: func(p *penComponent) float64 { return p.penTransparency }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := setupSpyPenMgr(t)
			sprite := newPenTestSprite()

			sprite.pen().setPenColorParam(tt.kind, tt.value(sprite.pen()))

			if spy.createCalls != 1 {
				t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
			}
			if spy.setColorCalls != 1 {
				t.Fatalf("SetPenColorTo calls = %d, want 1", spy.setColorCalls)
			}
			sprite.SetPenColor__1(tt.kind, tt.value(sprite.pen()))
			sprite.ChangePenColor(tt.kind, 0)
			if spy.setColorCalls != 1 {
				t.Fatalf("unchanged HSV emitted %d color updates, want 1", spy.setColorCalls)
			}
		})
	}
}

func TestPenColorParametersPreserveNonFiniteValues(t *testing.T) {
	for _, parameter := range []struct {
		name               string
		kind               PenColorParam
		value              func(*penComponent) float64
		positive, negative float64
	}{
		{"hue", PenHue, func(p *penComponent) float64 { return p.penHue }, math.NaN(), math.NaN()},
		{"saturation", PenSaturation, func(p *penComponent) float64 { return p.penSaturation }, 100, 0},
		{"brightness", PenBrightness, func(p *penComponent) float64 { return p.penBrightness }, 100, 0},
		{"transparency", PenTransparency, func(p *penComponent) float64 { return p.penTransparency }, 100, 0},
	} {
		for _, input := range []struct {
			name        string
			value, want float64
		}{
			{"nan", math.NaN(), math.NaN()},
			{"positive infinity", math.Inf(1), parameter.positive},
			{"negative infinity", math.Inf(-1), parameter.negative},
		} {
			t.Run(parameter.name+"/"+input.name, func(t *testing.T) {
				spy := setupSpyPenMgr(t)
				sprite := newPenTestSprite()
				sprite.SetPenColor__1(parameter.kind, input.value)
				got := parameter.value(sprite.pen())
				if got != input.want && !(math.IsNaN(got) && math.IsNaN(input.want)) {
					t.Fatalf("parameter = %v, want %v", got, input.want)
				}
				sprite.SetPenColor__1(parameter.kind, input.value)
				wantCalls := 1
				if math.IsNaN(input.want) {
					wantCalls = 2
				}
				if spy.setColorCalls != wantCalls {
					t.Fatalf("color updates = %d, want %d", spy.setColorCalls, wantCalls)
				}
			})
		}
	}
}

func TestPenComponentPenNoneDoesNothing(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenColorParam(PenNone, 50)
	sprite.pen().changePenColor(PenNone, 50)

	if spy.createCalls != 0 {
		t.Fatalf("CreatePen calls = %d, want 0", spy.createCalls)
	}
	if spy.setColorCalls != 0 {
		t.Fatalf("SetPenColorTo calls = %d, want 0", spy.setColorCalls)
	}
}

func TestPenComponentSetPenShadeUsesLegacyDefaults(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenShade(50)

	if spy.createCalls != 1 {
		t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
	}
	if spy.setColorCalls != 1 {
		t.Fatalf("SetPenColorTo calls = %d, want 1", spy.setColorCalls)
	}

	want := toMathfColor(HSB(66.66, 100, 100))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
	assertNearlyEqualPenValue(t, "penShade", sprite.pen().legacyPenColor.shade, 50)
}

func TestPenComponentChangePenShadeUsesStoredLegacyShade(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenColor(HSB(20, 80, 80))
	assertNearlyEqualPenValue(t, "penShade", sprite.pen().legacyPenColor.shade, 40)

	sprite.pen().changePenShade(10)

	want := toMathfColor(HSB(20, 100, 100))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
	assertNearlyEqualPenValue(t, "penShade", sprite.pen().legacyPenColor.shade, 50)
	if spy.setColorCalls != 2 {
		t.Fatalf("SetPenColorTo calls = %d, want 2", spy.setColorCalls)
	}
}

func TestPenComponentSetPenShadeUsesCurrentHueParam(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenColor(HSB(25, 60, 70))
	sprite.pen().setPenColorParam(PenHue, 10)
	sprite.pen().setPenShade(50)

	want := toMathfColor(HSB(10, 100, 100))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
	if spy.setColorCalls != 3 {
		t.Fatalf("SetPenColorTo calls = %d, want 3", spy.setColorCalls)
	}
}

func TestPenComponentPenHueParamWraps(t *testing.T) {
	setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenColorParam(PenHue, 110)

	assertNearlyEqualPenValue(t, "penHue", sprite.pen().penHue, 10)
	want := toMathfColor(HSB(10, sprite.pen().penSaturation, sprite.pen().penBrightness))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
}

func TestPenComponentLegacyChangePenHueMatchesSemantics(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().setPenShade(50)
	sprite.pen().changePenHue(2)

	want := toMathfColor(HSB(legacyDefaultPenHue+1, 100, 100))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
	if spy.setColorCalls != 2 {
		t.Fatalf("SetPenColorTo calls = %d, want 2", spy.setColorCalls)
	}
}

func TestPenComponentCloneMoveMaterializesPenTrail(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	sprite.transform().x = 12
	sprite.transform().y = 34

	pen := sprite.pen()
	pen.isPenDown = true
	pen.penWidth = 10
	pen.penColor = toMathfColor(HSB(20, 80, 90))

	pen.movePen(20, 40)

	if spy.createCalls != 1 {
		t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
	}
	if spy.penDownCalls != 1 {
		t.Fatalf("PenDown calls = %d, want 1", spy.penDownCalls)
	}
	if spy.setSizeCalls != 1 {
		t.Fatalf("SetPenSizeTo calls = %d, want 1", spy.setSizeCalls)
	}
	if spy.setColorCalls != 1 {
		t.Fatalf("SetPenColorTo calls = %d, want 1", spy.setColorCalls)
	}
	if spy.moveCalls != 2 {
		t.Fatalf("MovePenTo calls = %d, want 2", spy.moveCalls)
	}
	want := mathf.NewVec2(20, 40)
	if spy.lastMove != want {
		t.Fatalf("MovePenTo position = %v, want %v", spy.lastMove, want)
	}
}

func TestPenComponentDirectCommandsUseLogicalPosition(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)

	sprite.pen().penDown()
	sprite.pen().penUp()
	sprite.g.flushPenCommands()

	if want := []string{"size", "color", "move", "down", "up"}; !slices.Equal(spy.events, want) {
		t.Fatalf("events = %v, want %v", spy.events, want)
	}
	if want := []bool{false}; !slices.Equal(spy.penDownModes, want) {
		t.Fatalf("pen down mouse modes = %v, want %v", spy.penDownModes, want)
	}

	want := mathf.NewVec2(50, 60)
	if spy.lastMove != want {
		t.Fatalf("MovePenTo position = %v, want %v", spy.lastMove, want)
	}
}

func TestPenComponentStampUsesRenderedPosition(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)

	sprite.pen().stamp()

	if spy.stampSpriteCalls != 1 || spy.lastStampSprite != sprite.runtimeState.syncSprite.Id {
		t.Fatalf("PenStampSprite calls = %d, sprite = %v", spy.stampSpriteCalls, spy.lastStampSprite)
	}
	if got, want := spy.spriteMgr.position, mathf.NewVec2(50, 60); got != want {
		t.Fatalf("sprite position = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.renderOffset, mathf.NewVec2(37, -24); got != want {
		t.Fatalf("sprite render offset = %v, want %v", got, want)
	}
}

func TestPenComponentStampSyncsRenderedTransform(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.runtimeState.scale = 2
	sprite.transform().direction = -30
	sprite.transform().rotationStyle = LeftRight

	sprite.pen().stamp()

	if spy.createCalls != 0 || sprite.pen().penObj != nil {
		t.Fatalf("stamp allocated a pen: create=%d pen=%v", spy.createCalls, sprite.pen().penObj)
	}
	if spy.stampSpriteCalls != 1 {
		t.Fatalf("PenStampSprite calls = %d, want 1", spy.stampSpriteCalls)
	}

	wantRotation := 0.0
	if spy.spriteMgr.rotation != wantRotation {
		t.Fatalf("sprite rotation = %v, want %v", spy.spriteMgr.rotation, wantRotation)
	}

	wantScale := mathf.NewVec2(-1, 1)
	if spy.spriteMgr.scale != wantScale {
		t.Fatalf("sprite scale = %v, want %v", spy.spriteMgr.scale, wantScale)
	}
	if got, want := spy.spriteMgr.renderScale, mathf.NewVec2(2, 2); got != want {
		t.Fatalf("sprite render scale = %v, want %v", got, want)
	}

	wantTexturePath := sprite.getCostumeAssetPath()
	if spy.spriteMgr.texturePath != wantTexturePath {
		t.Fatalf("sprite texturePath = %q, want %q", spy.spriteMgr.texturePath, wantTexturePath)
	}
}

func TestPenComponentStampSyncsNormalRotation(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.transform().direction = 45
	sprite.transform().rotationStyle = Normal

	sprite.pen().stamp()

	wantRotation := engine.DegToRad(-45)
	if spy.spriteMgr.rotation != wantRotation {
		t.Fatalf("sprite rotation = %v, want %v", spy.spriteMgr.rotation, wantRotation)
	}
}

func TestPenComponentStampSyncsHiddenSpriteBeforeCapture(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.spriteState.isVisible = false
	sprite.runtimeState.scale = 2
	sprite.transform().x = 120
	spy.onStamp = func() {
		if got := spy.spriteMgr.position.X; got != 120 {
			t.Fatalf("position at capture = %v, want 120", got)
		}
		if spy.spriteMgr.visible {
			t.Fatal("stamping made a hidden sprite visible")
		}
		if spy.spriteMgr.texturePath != sprite.getCostumeAssetPath() || spy.spriteMgr.renderScale.X != 2 {
			t.Fatal("capture ran before the pending costume and scale were synchronized")
		}
	}

	sprite.pen().stamp()
	if spy.stampSpriteCalls != 1 {
		t.Fatalf("hidden sprite stamp calls = %d, want 1", spy.stampSpriteCalls)
	}
}

func TestPenComponentStampReusesAllSpriteEffects(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	for kind := EffectKind(0); kind < enumNumOfEffect; kind++ {
		sprite.setGraphicEffect(kind, 35)
	}
	spy.onStamp = func() {
		for kind := EffectKind(0); kind < enumNumOfEffect; kind++ {
			if got, want := spy.spriteMgr.effects[kind.String()], normalizeEffectValue(kind, 35); got != want {
				t.Fatalf("%v at capture = %v, want %v", kind, got, want)
			}
		}
	}
	sprite.pen().stamp()
}

func TestPenComponentStampSyncsAtlasCostume(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	costume := sprite.currentCostume()
	costume.atlas = true
	costume.frame.PosX, costume.frame.PosY = 32, 16
	spy.onStamp = func() {
		if got, want := spy.spriteMgr.atlasRegion, mathf.NewRect2(32, 16, 100, 80); got != want {
			t.Fatalf("atlas region at capture = %v, want %v", got, want)
		}
	}
	sprite.pen().stamp()
}

func TestPenComponentStampSkipsUnavailableSprite(t *testing.T) {
	for _, destroyed := range []bool{false, true} {
		spy := setupSpyPenMgr(t)
		sprite := newPenTestSprite()
		if destroyed {
			configurePenRenderOffsetSprite(sprite)
			sprite.markDestroyed()
		}
		sprite.pen().stamp()
		if spy.createCalls != 0 || spy.stampSpriteCalls != 0 {
			t.Fatalf("unavailable sprite allocated a pen or stamped: create=%d stamp=%d", spy.createCalls, spy.stampSpriteCalls)
		}
	}
}

func TestClonePenStateComesFromOriginal(t *testing.T) {
	game := setupCloneLimitGame(t)
	original := newCloneLimitSprite(game, "original")
	original.pen().penState = penState{
		penColor:        toMathfColor(HSB(20, 80, 90)),
		penWidth:        5,
		penHue:          20,
		legacyPenColor:  legacyPenState{hue: 20, shade: 40},
		penSaturation:   80,
		penBrightness:   90,
		penTransparency: 30,
		isPenDown:       true,
	}
	penObj := internalengine.Object(42)
	original.pen().penObj = &penObj
	parent := createRuntimeClone(&original.SpriteImpl)
	parent.pen().penWidth = 50
	parent.pen().penHue = 80
	parent.pen().isPenDown = false
	original.pen().penWidth = 7
	child := createRuntimeClone(parent)
	if got, want := child.pen().penState, original.pen().penState; got != want {
		t.Fatalf("descendant pen state = %+v, want current original state %+v", got, want)
	}
	if child.pen().penObj != nil {
		t.Fatal("cloning allocated or shared a drawing resource")
	}
	child.pen().penWidth = 9
	if original.pen().penWidth != 7 || parent.pen().penWidth != 50 {
		t.Fatal("clone pen state aliases an ancestor")
	}
}

func TestClonePenStateFromStageInstance(t *testing.T) {
	game := setupCloneLimitGame(t)
	template := newCloneLimitSprite(game, "template")
	template.pen().penWidth = 5
	var instance cloneLimitSprite
	instantiateStageSprite(reflect.ValueOf(&instance).Elem(), template, spriteProperties{})
	game.shapeMgr.add(&instance.SpriteImpl)
	instance.pen().penWidth = 10
	child := createRuntimeClone(&instance.SpriteImpl)
	if child.pen().penWidth != 10 {
		t.Fatal("clone used template state instead of its original instance")
	}
}

func TestPenStateWithoutAllocatedResourceDoesNotSubmitCommands(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	pen := sprite.pen()
	pen.isPenDown = true
	pen.penUp()
	pen.syncPenPosition(10, 20)
	sprite.g.flushPenCommands()
	if pen.isPenDown || pen.penObj != nil || spy.createCalls != 0 || spy.batchCalls != 0 {
		t.Fatal("unallocated pen operation created or submitted a resource")
	}
}

func TestLegacyPenHueResetsTransparencyAndSkipsUnchangedProperties(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	pen := sprite.pen()
	pen.setPenColorParam(PenTransparency, 60)
	pen.setPenHue(-20)
	sprite.g.flushPenCommands()
	if pen.legacyPenColor.hue != 90 || pen.penTransparency != 0 || pen.penColor.A != 1 {
		t.Fatalf("legacy hue/transparency/color = %v/%v/%v", pen.legacyPenColor.hue, pen.penTransparency, pen.penColor)
	}
	batches := spy.batchCalls
	pen.setPenHue(-20)
	pen.changePenHue(0)
	pen.setPenShade(pen.legacyPenColor.shade)
	sprite.g.flushPenCommands()
	if spy.batchCalls != batches {
		t.Fatal("unchanged legacy appearance submitted another pen batch")
	}
	if got := wrapLegacyPenShade(-25); got != 175 {
		t.Fatalf("wrapped shade = %v, want 175", got)
	}
	for _, shade := range []float64{25, 175} {
		got := makeLegacyPenColor(0, shade, 25)
		want := mathf.NewColor(35.0/60, 0, 0, 0.75)
		if !samePenColor(got, want) {
			t.Fatalf("legacy shade %v = %v, want %v", shade, got, want)
		}
	}
}

func TestPenComponentQueuesOneOrderedBatch(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	sprite.g.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
	sprite.transform().x = 12
	sprite.transform().y = 34

	sprite.pen().setPenColor(HSB(20, 80, 90))
	sprite.pen().penDown()
	sprite.pen().penUp()
	if len(spy.events) != 0 {
		t.Fatalf("engine calls before flush = %v, want none", spy.events)
	}
	sprite.g.flushPenCommands()

	if spy.batchCalls != 1 {
		t.Fatalf("batch calls = %d, want 1", spy.batchCalls)
	}
	if spy.moveCalls != 0 || spy.penDownCalls != 0 || spy.penUpCalls != 0 || spy.setColorCalls != 0 || spy.setSizeCalls != 0 {
		t.Fatalf("individual calls = move:%d down:%d up:%d color:%d size:%d, want all zero",
			spy.moveCalls, spy.penDownCalls, spy.penUpCalls, spy.setColorCalls, spy.setSizeCalls)
	}

	batch := spy.batches[0]
	if got, want := int(batch[0]), 6; got != want {
		t.Fatalf("command count = %d, want %d", got, want)
	}
	wantOps := []int{
		internalengine.PenBatchColor,
		internalengine.PenBatchSetSize,
		internalengine.PenBatchColor,
		internalengine.PenBatchMove,
		internalengine.PenBatchDown,
		internalengine.PenBatchUp,
	}
	gotOps := penBatchOperations(batch)
	if !slices.Equal(gotOps, wantOps) {
		t.Fatalf("command operations = %v, want %v", gotOps, wantOps)
	}

	if got := batch[1+4*internalengine.PenBatchFields+3]; got != 0 {
		t.Fatalf("pen down moveByMouse = %v, want 0", got)
	}

	move := batch[1+3*internalengine.PenBatchFields : 1+4*internalengine.PenBatchFields]
	if got, want := move[3], float32(12); got != want {
		t.Fatalf("move x = %v, want %v", got, want)
	}
	if got, want := move[4], float32(34); got != want {
		t.Fatalf("move y = %v, want logical SPX y %v", got, want)
	}
}

func TestClonePenColorChangeFollowsInitialDrawingState(t *testing.T) {
	spy := setupSpyPenMgr(t)
	original := newPenTestSprite()
	original.SetPenColor__0(HSBA(20, 80, 90, 25))
	original.PenDown()

	clone := newPenTestSprite()
	clone.components.pen = original.pen().cloneFor(&clone.SpriteImpl)
	clone.g.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
	clone.ChangePenColor(PenTransparency, 10)
	clone.g.flushPenCommands()

	if len(spy.batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(spy.batches))
	}
	batch := spy.batches[0]
	wantOps := []int{
		internalengine.PenBatchSetSize,
		internalengine.PenBatchColor,
		internalengine.PenBatchMove,
		internalengine.PenBatchDown,
		internalengine.PenBatchColor,
	}
	if got := penBatchOperations(batch); !slices.Equal(got, wantOps) {
		t.Fatalf("commands = %v, want %v", got, wantOps)
	}
	if got := batch[1+3*internalengine.PenBatchFields+3]; got != 0 {
		t.Fatalf("clone pen down moveByMouse = %v, want 0", got)
	}
	if got := batch[1+internalengine.PenBatchFields+6]; got != 0.25 {
		t.Fatalf("initial alpha = %v, want 0.25", got)
	}
	if got := batch[1+4*internalengine.PenBatchFields+6]; got != float32(0.15) {
		t.Fatalf("changed alpha = %v, want 0.15", got)
	}
	if original.pen().penColor.A != 0.25 {
		t.Fatal("clone color change modified the original")
	}
}

func TestPenBatchBarriersPreserveOrder(t *testing.T) {
	tests := []struct {
		name   string
		action func(*penTestSprite)
		last   string
	}{
		{
			name: "stamp",
			action: func(sprite *penTestSprite) {
				configurePenRenderOffsetSprite(sprite)
				sprite.pen().stamp()
			},
			last: "stamp",
		},
		{
			name: "erase all",
			action: func(sprite *penTestSprite) {
				sprite.g.EraseAll()
			},
			last: "erase",
		},
		{
			name: "destroy",
			action: func(sprite *penTestSprite) {
				sprite.pen().onDestroy()
			},
			last: "destroy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := setupSpyPenMgr(t)
			sprite := newPenTestSprite()
			sprite.g.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
			sprite.pen().penDown()

			tt.action(sprite)

			if got, want := spy.events, []string{"batch", tt.last}; !slices.Equal(got, want) {
				t.Fatalf("events = %v, want %v", got, want)
			}
		})
	}
}

func TestOnEngineRenderFlushesPendingPenCommandsOnEarlyReturn(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	game.queuePenUp(7)

	game.OnEngineRender(0)

	if spy.batchCalls != 1 {
		t.Fatalf("batch calls = %d, want 1", spy.batchCalls)
	}
	if got, want := penBatchOperations(spy.batches[0]), []int{internalengine.PenBatchUp}; !slices.Equal(got, want) {
		t.Fatalf("command operations = %v, want %v", got, want)
	}
}

func TestOnEngineDestroyDiscardsPendingPenCommands(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	game.queuePenUp(7)

	game.OnEngineDestroy()
	game.flushPenCommands()

	if spy.batchCalls != 0 {
		t.Fatalf("batch calls = %d, want 0", spy.batchCalls)
	}
}

func TestPenBatchFlushesAtCommandLimit(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	for i := 0; i < 1025; i++ {
		game.queuePenUp(internalengine.Object(i + 1))
	}

	if spy.batchCalls != 1 {
		t.Fatalf("batch calls at limit = %d, want 1", spy.batchCalls)
	}
	if got, want := int(spy.batches[0][0]), 1024; got != want {
		t.Fatalf("first batch command count = %d, want %d", got, want)
	}
	game.flushPenCommands()
	if spy.batchCalls != 2 {
		t.Fatalf("batch calls after final flush = %d, want 2", spy.batchCalls)
	}
	if got, want := int(spy.batches[1][0]), 1; got != want {
		t.Fatalf("second batch command count = %d, want %d", got, want)
	}
}

func TestSyncPenCanvasToWorldUsesLogicalStageSize(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{}
	game.displayState.WorldWidth = 640
	game.displayState.WorldHeight = 480
	game.displayState.WindowWidth = 480
	game.displayState.WindowHeight = 360

	game.syncPenCanvasToWorld()

	if spy.canvasCalls != 1 || spy.canvasWidth != 640 || spy.canvasHeight != 480 {
		t.Fatalf("pen canvas calls/size = %d/%dx%d, want 1/640x480", spy.canvasCalls, spy.canvasWidth, spy.canvasHeight)
	}
}

func TestSyncPenCanvasToWorldFlushesPendingCommandsFirst(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	game.displayState.WorldWidth = 640
	game.displayState.WorldHeight = 480
	game.queuePenMove(1, mathf.NewVec2(10, 20))

	game.syncPenCanvasToWorld()

	if want := []string{"batch", "canvas"}; !slices.Equal(spy.events, want) {
		t.Fatalf("events = %v, want %v", spy.events, want)
	}
}

func TestEveryPenCommandFlushesAtBatchLimit(t *testing.T) {
	for _, tt := range []struct {
		name string
		op   int
		add  func(*Game)
	}{
		{"move", internalengine.PenBatchMove, func(g *Game) { g.queuePenMove(1, mathf.NewVec2(2, 3)) }},
		{"down", internalengine.PenBatchDown, func(g *Game) { g.queuePenDown(1) }},
		{"color", internalengine.PenBatchColor, func(g *Game) { g.queuePenColor(1, mathf.NewColorRGBA(1, 0, 0, 1)) }},
		{"size", internalengine.PenBatchSetSize, func(g *Game) { g.queuePenSize(1, 3) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spy := setupSpyPenMgr(t)
			game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
			for range 1024 {
				tt.add(game)
			}
			if spy.batchCalls != 1 || len(spy.batches) != 1 || int(spy.batches[0][0]) != 1024 {
				t.Fatalf("full batch was not flushed once: %d", spy.batchCalls)
			}
			for _, op := range penBatchOperations(spy.batches[0]) {
				if op != tt.op {
					t.Fatalf("operation = %d, want %d", op, tt.op)
				}
			}
			game.flushPenCommands()
			if spy.batchCalls != 1 {
				t.Fatal("flushed batch was replayed")
			}
		})
	}
}

func TestSpritePenPublicAPIUpdatesDrawingState(t *testing.T) {
	mgr := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.SetPenHue(20)
	sprite.ChangePenHue(10)
	sprite.SetPenShade(40)
	sprite.ChangePenShade(5)
	sprite.SetPenSize(3)
	sprite.ChangePenSize(2)
	sprite.PenDown()
	sprite.PenUp()
	sprite.Stamp()
	if sprite.pen().legacyPenColor.hue != 15 || sprite.pen().legacyPenColor.shade != 45 || sprite.pen().penWidth != 5 || sprite.pen().isPenDown {
		t.Fatalf("pen API state=%+v", sprite.pen().penState)
	}
	if mgr.stampSpriteCalls != 1 {
		t.Fatalf("stamp captures=%d,want1", mgr.stampSpriteCalls)
	}
}

func TestPenColorParamFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected PenColorParam
	}{
		{name: "color lowercase", input: "color", expected: PenHue},
		{name: "color uppercase", input: "COLOR", expected: PenHue},
		{name: "color mixed case", input: "Color", expected: PenHue},
		{name: "saturation", input: "saturation", expected: PenSaturation},
		{name: "saturation uppercase", input: "SATURATION", expected: PenSaturation},
		{name: "brightness", input: "brightness", expected: PenBrightness},
		{name: "brightness uppercase", input: "BRIGHTNESS", expected: PenBrightness},
		{name: "transparency", input: "transparency", expected: PenTransparency},
		{name: "transparency uppercase", input: "TRANSPARENCY", expected: PenTransparency},
		{name: "unknown string falls back to PenNone", input: "unknown", expected: PenNone},
		{name: "empty string falls back to PenNone", input: "", expected: PenNone},
		{name: "numeric string falls back to PenNone", input: "123", expected: PenNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PenColorParamFromString(tt.input)
			if got != tt.expected {
				t.Errorf("PenColorParamFromString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestColorSensingFlushesPendingPenBeforeQuery(t *testing.T) {
	for _, tt := range []struct {
		name  string
		query func(*SpriteImpl) bool
	}{
		{"touching color", func(sprite *SpriteImpl) bool { return sprite.TouchingColor__0(HSB(0, 100, 100)) }},
		{"color touching color", func(sprite *SpriteImpl) bool {
			return sprite.TouchingColor__1(HSB(0, 0, 100), HSB(0, 100, 100))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pen := setupSpyPenMgr(t)
			mgr := newTouchingSyncSpriteMgr()
			installTouchingSyncSpriteMgr(t, mgr)
			sprite := newTouchingTestSprite("sensor", 0, 0, 1)
			sprite.g.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
			sprite.g.queuePenDown(7)
			sprite.g.queuePenMove(7, mathf.NewVec2(10, 20))
			mgr.onColorCollision = func() {
				if pen.batchCalls != 1 {
					t.Fatalf("color query saw %d pen batches, want 1", pen.batchCalls)
				}
				ops := penBatchOperations(pen.batches[0])
				if len(ops) != 2 || ops[0] != internalengine.PenBatchDown || ops[1] != internalengine.PenBatchMove {
					t.Fatalf("color query saw pen commands %v, want down then move", ops)
				}
			}
			for query := range 2 {
				if !tt.query(sprite) {
					t.Errorf("color sensing query %d returned false", query+1)
				}
			}
			sprite.g.flushPenCommands()
			if pen.batchCalls != 1 {
				t.Fatalf("pen batches after repeated queries = %d, want 1", pen.batchCalls)
			}
		})
	}
}

func TestTilemapSyncsPenCanvasAfterWorldSize(t *testing.T) {
	for _, tt := range []struct {
		name   string
		load   func(*Game, string)
		events []string
	}{
		{"setup", func(game *Game, path string) {
			game.tilemapMgr.loadMap(path)
			game.setupAudioAndTilemap(&coreproject.ProjectConfig{})
		}, []string{"tiles", "canvas"}},
		{"dynamic load", func(game *Game, path string) {
			game.penSyncBuffer = internalengine.NewPenSyncBuffer(1)
			game.queuePenMove(1, mathf.NewVec2(10, 20))
			game.LoadTilemap(path)
		}, []string{"tiles", "batch", "canvas"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game, penSpy, cameraSpy, tilemapPath := newPenCanvasTilemapTestGame(t)
			tt.load(game, tilemapPath)
			if game.displayState.WorldWidth != 96 || game.displayState.WorldHeight != 48 {
				t.Fatalf("world size = %dx%d, want 96x48", game.displayState.WorldWidth, game.displayState.WorldHeight)
			}
			if penSpy.canvasCalls != 1 || penSpy.canvasWidth != 96 || penSpy.canvasHeight != 48 {
				t.Fatalf("pen canvas calls/size = %d/%dx%d, want 1/96x48", penSpy.canvasCalls, penSpy.canvasWidth, penSpy.canvasHeight)
			}
			wantLimits := [4]int64{64, -80, 160, -32}
			if cameraSpy.calls != 4 || cameraSpy.limits != wantLimits {
				t.Fatalf("camera limit calls/values = %d/%v, want 4/%v", cameraSpy.calls, cameraSpy.limits, wantLimits)
			}
			if !reflect.DeepEqual(penSpy.events, tt.events) {
				t.Fatalf("events = %v, want %v", penSpy.events, tt.events)
			}
		})
	}
}

func TestClonePenComponentWithoutGame(t *testing.T) {
	sourceSprite, target := &SpriteImpl{}, &SpriteImpl{}
	pen := &penComponent{sprite: sourceSprite}
	pen.penWidth = 7
	clonedPen := pen.cloneFor(target)
	if clonedPen.sprite != target || clonedPen.penWidth != 7 {
		t.Fatal("standalone pen clone lost state")
	}
}

func (s *spyStampSpriteMgr) SetTransform(_ engine.Object, position mathf.Vec2, rotation float64, scale mathf.Vec2, visible bool, offset mathf.Vec2) {
	s.position, s.rotation, s.scale = position, rotation, scale
	s.visible, s.renderOffset = visible, offset
}

func (s *spyStampSpriteMgr) SetTexture(_ engine.Object, path string) {
	s.texturePath = path
}

func (s *spyStampSpriteMgr) SetTextureAtlas(_ engine.Object, path string, region mathf.Rect2) {
	s.texturePath, s.atlasRegion = path, region
}

func (s *spyStampSpriteMgr) SetRenderScale(_ engine.Object, scale mathf.Vec2) {
	s.renderScale = scale
}

func (*spyStampSpriteMgr) SetMaterialShader(engine.Object, string) {}

func (*spyStampSpriteMgr) SetMaterialParamsVec(engine.Object, string, float64, float64, float64, float64) {
}

func (s *spyStampSpriteMgr) SetMaterialParams(_ engine.Object, effect string, amount float64) {
	s.effects[effect] = amount
}

func (*penTestSprite) Main() {}

func (s *spyPenMgr) DestroyAllPens() {
	s.events = append(s.events, "erase")
}

func (s *spyPenMgr) SetCanvasSize(width, height int64) {
	s.canvasCalls++
	s.canvasWidth = width
	s.canvasHeight = height
	s.events = append(s.events, "canvas")
}

func (s *spyPenMgr) CreatePen() engine.Object {
	s.createCalls++
	return engine.Object(s.createCalls)
}

func (s *spyPenMgr) DestroyPen(obj engine.Object) {
	s.events = append(s.events, "destroy")
}

func (s *spyPenMgr) PenStamp(obj engine.Object) {}

func (s *spyPenMgr) MovePenTo(obj engine.Object, position mathf.Vec2) {
	s.moveCalls++
	s.lastMove = position
	s.events = append(s.events, "move")
}

func (s *spyPenMgr) PenDown(obj engine.Object, moveByMouse bool) {
	s.penDownCalls++
	s.penDownModes = append(s.penDownModes, moveByMouse)
	s.events = append(s.events, "down")
}

func (s *spyPenMgr) PenUp(obj engine.Object) {
	s.penUpCalls++
	s.events = append(s.events, "up")
}

func (s *spyPenMgr) SetPenColorTo(obj engine.Object, color mathf.Color) {
	s.setColorCalls++
	s.events = append(s.events, "color")
}

func (s *spyPenMgr) ChangePenBy(obj engine.Object, property int64, amount float64) {}

func (s *spyPenMgr) SetPenTo(obj engine.Object, property int64, value float64) {}

func (s *spyPenMgr) ChangePenSizeBy(obj engine.Object, amount float64) {}

func (s *spyPenMgr) SetPenSizeTo(obj engine.Object, size float64) {
	s.setSizeCalls++
	s.events = append(s.events, "size")
}

func (s *spyPenMgr) SetPenStampTexture(obj engine.Object, texturePath string) {}

func (s *spyPenMgr) PenStampWithTransform(engine.Object, string, mathf.Vec2, float64, mathf.Vec2) {
	panic("sprite stamps must use the current rendered sprite")
}

func (s *spyPenMgr) PenStampSprite(spriteID engine.Object) {
	s.stampSpriteCalls++
	s.events = append(s.events, "stamp")
	s.lastStampSprite = spriteID
	if s.onStamp != nil {
		s.onStamp()
	}
}

func (s *spyPenMgr) BatchUpdateCommands(buffer []float32) {
	s.batchCalls++
	s.batches = append(s.batches, append([]float32(nil), buffer...))
	s.events = append(s.events, "batch")
}

func (*penCanvasTilemapMgr) SetLayerIndex(int64) {}

func (m *penCanvasTilemapMgr) PlaceTilesWithLayer(engine.Array, string, int64) {
	*m.events = append(*m.events, "tiles")
}

func (m *penCanvasCameraMgr) SetCameraLimit(side, limit int64) {
	m.calls++
	m.limits[side] = limit
}

func (*penCanvasCameraMgr) SetCameraSmoothing(bool) {}

func (*penCanvasAudioBackend) CreateAudio() engine.Object { return 1 }

func newPenTestSprite() *penTestSprite {
	game := &Game{}
	sprite := &penTestSprite{}
	sprite.g = game
	sprite.name = "PenTest"
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&game.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func configurePenRenderOffsetSprite(sprite *penTestSprite) {
	sprite.runtimeState.scale = 1
	sprite.transform().x = 50
	sprite.transform().y = 60
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		path: "sprites/PenTest/costume1.svg",
		frame: assetutil.FrameDescriptor{
			Center:           mathf.NewVec2(10, 20),
			BitmapResolution: 1,
			Width:            100,
			Height:           80,
		},
	}}
	sprite.costumeIndex = 0
	sprite.runtimeState.syncSprite = &internalengine.Sprite{}
	sprite.runtimeState.syncSprite.SetId(101)
	sprite.runtimeState.isCostumeDirty = true
	sprite.markProxyDirty()
}

func setupSpyPenMgr(t *testing.T) *spyPenMgr {
	t.Helper()

	setupImmediateEngineCalls(t)

	spy := &spyPenMgr{spriteMgr: &spyStampSpriteMgr{effects: make(map[string]float64)}}
	original := engine.PenMgr
	originalSprites := engine.SpriteMgr
	engine.PenMgr = spy
	engine.SpriteMgr = spy.spriteMgr
	t.Cleanup(func() {
		engine.PenMgr = original
		engine.SpriteMgr = originalSprites
	})
	return spy
}

func assertNearlyEqualPenValue(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !nearlyEqualPenValue(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func penBatchOperations(batch []float32) []int {
	count := int(batch[0])
	operations := make([]int, count)
	for i := range operations {
		operations[i] = int(batch[1+i*internalengine.PenBatchFields])
	}
	return operations
}

func newPenCanvasTilemapTestGame(t *testing.T) (*Game, *spyPenMgr, *penCanvasCameraMgr, string) {
	t.Helper()

	spy := setupSpyPenMgr(t)
	originalTilemapMgr, originalCameraMgr := engine.TilemapMgr, engine.CameraMgr
	engine.TilemapMgr = &penCanvasTilemapMgr{events: &spy.events}
	cameraSpy := &penCanvasCameraMgr{}
	engine.CameraMgr = cameraSpy
	t.Cleanup(func() {
		engine.TilemapMgr, engine.CameraMgr = originalTilemapMgr, originalCameraMgr
	})

	const tilemapPath = "tilemaps/map1.json"
	fs := gameTestFS{
		tilemapPath: `{
			"tilemap": {
				"tile_size": {"width": 32, "height": 16},
				"tileset": {"sources": []},
				"layers": [{"tile_data": [1, 2, 3, 0, 0, 1, 4, 5, 0, 0]}]
			},
			"decorators": [],
			"sprites": []
		}`,
	}
	game := &Game{fs: fs}
	game.displayState.WorldWidth = 480
	game.displayState.WorldHeight = 360
	game.tilemapMgr.g = game
	game.tilemapMgr.fs = fs
	game.camera = &cameraImpl{g: game}
	game.Camera = game.camera
	game.soundMgr.Init(&penCanvasAudioBackend{})
	return game, spy, cameraSpy, tilemapPath
}
