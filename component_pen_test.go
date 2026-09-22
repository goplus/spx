package spx

import (
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type spyPenMgr struct {
	canvasWidth      int64
	canvasHeight     int64
	canvasCalls      int
	createCalls      int
	moveCalls        int
	penDownCalls     int
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

type penTestSprite struct {
	SpriteImpl
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
}

func (s *spyPenMgr) PenDown(obj engine.Object, moveByMouse bool) {
	s.penDownCalls++
}

func (s *spyPenMgr) PenUp(obj engine.Object) {
	s.penUpCalls++
}

func (s *spyPenMgr) SetPenColorTo(obj engine.Object, color mathf.Color) {
	s.setColorCalls++
}

func (s *spyPenMgr) ChangePenBy(obj engine.Object, property int64, amount float64) {}

func (s *spyPenMgr) SetPenTo(obj engine.Object, property int64, value float64) {}

func (s *spyPenMgr) ChangePenSizeBy(obj engine.Object, amount float64) {}

func (s *spyPenMgr) SetPenSizeTo(obj engine.Object, size float64) {
	s.setSizeCalls++
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
	sprite.runtimeState.Scale = 1
	sprite.transform().x = 50
	sprite.transform().y = 60
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		setIndex:         -1,
		path:             "sprites/PenTest/costume1.svg",
		center:           mathf.NewVec2(10, 20),
		bitmapResolution: 1,
		width:            100,
		height:           80,
	}}
	sprite.costumeIndex = 0
	sprite.runtimeState.SyncSprite = &internalengine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(101)
	sprite.runtimeState.IsCostumeDirty = true
	sprite.markProxyDirty()
}

func setupSpyPenMgr(t *testing.T) *spyPenMgr {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

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
		})
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

func TestPenComponentPenDownUsesLogicalPosition(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)

	sprite.pen().penDown()

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

	if spy.stampSpriteCalls != 1 || spy.lastStampSprite != sprite.runtimeState.SyncSprite.Id {
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
	sprite.runtimeState.Scale = 2
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
	sprite.spriteState.IsVisible = false
	sprite.runtimeState.Scale = 2
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
	costume.setIndex = 0
	costume.posX, costume.posY = 32, 16
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

func assertNearlyEqualPenValue(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !nearlyEqualPenValue(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
