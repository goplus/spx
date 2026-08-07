package spx

import (
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/enginewrap"
	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type spyPenMgr struct {
	canvasWidth          int64
	canvasHeight         int64
	canvasCalls          int
	createCalls          int
	moveCalls            int
	penDownCalls         int
	penUpCalls           int
	setColorCalls        int
	setSizeCalls         int
	stampWithCalls       int
	lastMove             mathf.Vec2
	lastStampPosition    mathf.Vec2
	lastStampTexturePath string
	lastStampRotation    float64
	lastStampScale       mathf.Vec2
	batchCalls           int
	batches              [][]float32
	events               []string
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

func (s *spyPenMgr) PenStampWithTransform(obj engine.Object, texturePath string, position mathf.Vec2, rotationRadians float64, scale mathf.Vec2) {
	s.stampWithCalls++
	s.events = append(s.events, "stamp")
	s.lastStampTexturePath = texturePath
	s.lastStampPosition = position
	s.lastStampRotation = rotationRadians
	s.lastStampScale = scale
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
	sprite.scriptEventBindings.init(&game.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func configurePenRenderOffsetSprite(sprite *penTestSprite) {
	sprite.runtimeState.Scale = 1
	sprite.transform().x = 50
	sprite.transform().y = 60
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		path:             "sprites/PenTest/costume1.svg",
		center:           mathf.NewVec2(10, 20),
		bitmapResolution: 1,
		width:            100,
		height:           80,
	}}
	sprite.costumeIndex = 0
}

func setupSpyPenMgr(t *testing.T) *spyPenMgr {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	spy := &spyPenMgr{}
	original := engine.PenMgr
	engine.PenMgr = spy
	t.Cleanup(func() {
		engine.PenMgr = original
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
	assertNearlyEqualPenValue(t, "penTransparency", pen.penTransparency, normalizedToPercent(wantColor.A))
}

func TestPenComponentRepeatedPenDownDrawsAtCurrentPosition(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().PenDown()
	sprite.pen().PenDown()

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

	sprite.pen().PenUp()
	if spy.createCalls != 0 {
		t.Fatalf("CreatePen calls after idle PenUp = %d, want 0", spy.createCalls)
	}
	if spy.penUpCalls != 0 {
		t.Fatalf("PenUp calls after idle PenUp = %d, want 0", spy.penUpCalls)
	}

	sprite.pen().PenDown()
	sprite.pen().PenUp()
	sprite.pen().PenUp()

	if spy.penUpCalls != 1 {
		t.Fatalf("PenUp calls after repeated PenUp = %d, want 1", spy.penUpCalls)
	}
}

func TestPenComponentIgnoresRepeatedPenStyleValues(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().SetPenSize(10)
	sprite.pen().SetPenSize(10)
	sprite.pen().SetPenColor(HSB(85, 33, 100))
	sprite.pen().SetPenColor(HSB(85, 33, 100))

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

	sprite.pen().SetPenSize(1)

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
	sprite.pen().SetPenColor(defaultColor)

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

			sprite.pen().SetPenColorParam(tt.kind, tt.value(sprite.pen()))

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

	sprite.pen().SetPenColorParam(PenNone, 50)
	sprite.pen().ChangePenColor(PenNone, 50)

	if spy.createCalls != 0 {
		t.Fatalf("CreatePen calls = %d, want 0", spy.createCalls)
	}
	if spy.setColorCalls != 0 {
		t.Fatalf("SetPenColorTo calls = %d, want 0", spy.setColorCalls)
	}
}

func TestPenComponentSetPenShadeUsesScratchLegacyDefaults(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().SetPenShade(50)

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

	sprite.pen().SetPenColor(HSB(20, 80, 80))
	assertNearlyEqualPenValue(t, "penShade", sprite.pen().legacyPenColor.shade, 40)

	sprite.pen().ChangePenShade(10)

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

	sprite.pen().SetPenColor(HSB(25, 60, 70))
	sprite.pen().SetPenColorParam(PenHue, 10)
	sprite.pen().SetPenShade(50)

	want := toMathfColor(HSB(10, 100, 100))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
	if spy.setColorCalls != 3 {
		t.Fatalf("SetPenColorTo calls = %d, want 3", spy.setColorCalls)
	}
}

func TestPenComponentPenHueParamWrapsLikeScratch(t *testing.T) {
	setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().SetPenColorParam(PenHue, 110)

	assertNearlyEqualPenValue(t, "penHue", sprite.pen().penHue, 10)
	want := toMathfColor(HSB(10, sprite.pen().penSaturation, sprite.pen().penBrightness))
	if !samePenColor(sprite.pen().penColor, want) {
		t.Fatalf("penColor = %v, want %v", sprite.pen().penColor, want)
	}
}

func TestPenComponentLegacyChangePenHueMatchesScratchSemantics(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()

	sprite.pen().SetPenShade(50)
	sprite.pen().changePenHue(2)

	want := toMathfColor(HSB(scratchLegacyDefaultPenHue+1, 100, 100))
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
	pen.penDown = true
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

	sprite.pen().PenDown()

	want := mathf.NewVec2(50, 60)
	if spy.lastMove != want {
		t.Fatalf("MovePenTo position = %v, want %v", spy.lastMove, want)
	}
}

func TestPenComponentStampUsesRenderedPosition(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)

	sprite.pen().Stamp()

	want := mathf.NewVec2(74, 97)
	if spy.stampWithCalls != 1 {
		t.Fatalf("PenStampWithTransform calls = %d, want 1", spy.stampWithCalls)
	}
	if spy.lastStampPosition != want {
		t.Fatalf("PenStampWithTransform position = %v, want %v", spy.lastStampPosition, want)
	}
}

func TestPenComponentStampSyncsRenderedTransform(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.runtimeState.Scale = 2
	sprite.transform().direction = -30
	sprite.transform().rotationStyle = LeftRight

	sprite.pen().Stamp()

	if spy.createCalls != 1 {
		t.Fatalf("CreatePen calls = %d, want 1", spy.createCalls)
	}
	if spy.stampWithCalls != 1 {
		t.Fatalf("PenStampWithTransform calls = %d, want 1", spy.stampWithCalls)
	}

	wantRotation := 0.0
	if spy.lastStampRotation != wantRotation {
		t.Fatalf("PenStampWithTransform rotation = %v, want %v", spy.lastStampRotation, wantRotation)
	}

	wantScale := mathf.NewVec2(-2, 2)
	if spy.lastStampScale != wantScale {
		t.Fatalf("PenStampWithTransform scale = %v, want %v", spy.lastStampScale, wantScale)
	}
	wantPosition := mathf.NewVec2(-24, 12)
	if spy.lastStampPosition != wantPosition {
		t.Fatalf("PenStampWithTransform position = %v, want %v", spy.lastStampPosition, wantPosition)
	}

	wantTexturePath := sprite.getCostumeAssetPath()
	if spy.lastStampTexturePath != wantTexturePath {
		t.Fatalf("PenStampWithTransform texturePath = %q, want %q", spy.lastStampTexturePath, wantTexturePath)
	}
}

func TestPenComponentStampSyncsNormalRotation(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.transform().direction = 45
	sprite.transform().rotationStyle = Normal

	sprite.pen().Stamp()

	wantRotation := engine.DegToRad(-45)
	if spy.lastStampRotation != wantRotation {
		t.Fatalf("PenStampWithTransform rotation = %v, want %v", spy.lastStampRotation, wantRotation)
	}
}

func assertNearlyEqualPenValue(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !nearlyEqualPenValue(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
