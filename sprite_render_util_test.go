package spx

import (
	"math"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

func newRenderOffsetTestSprite() *SpriteImpl {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.runtimeState.Scale = 1
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		center:           mathf.NewVec2(10, 20),
		bitmapResolution: 1,
		width:            100,
		height:           80,
	}}
	return sprite
}

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

func TestCostumeRenderAnchorConvertsAssetCoordinatesToSPX(t *testing.T) {
	costume := &costume{
		width:            200,
		height:           120,
		bitmapResolution: 2,
		center:           mathf.NewVec2(80, 40),
	}

	want := mathf.NewVec2(-10, 10)
	if got := costume.renderAnchorInSPX(); got != want {
		t.Fatalf("renderAnchorInSPX = %v, want %v", got, want)
	}
}

func TestWorldRenderOffsetAppliesRootFlipAndRotation(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.Scale = 2
	sprite.transform().rotationStyle = LeftRight
	sprite.transform().direction = -30

	x, y := getWorldRenderOffset(sprite)
	if x != -74 || y != -48 {
		t.Fatalf("left-right world render offset = (%v, %v), want (-74, -48)", x, y)
	}

	sprite.runtimeState.Scale = 1
	sprite.transform().rotationStyle = Normal
	sprite.transform().direction = 45
	x, y = getWorldRenderOffset(sprite)
	wantX := 61 / math.Sqrt2
	wantY := 13 / math.Sqrt2
	if math.Abs(x-wantX) > 1e-9 || math.Abs(y-wantY) > 1e-9 {
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
	sprite.costumes = []*costume{{bitmapResolution: 1}}

	rotation, scaleX, scaleY := getRenderRotationAndScale(sprite)
	if rotation != 0 || scaleX != -1 || scaleY != 1 {
		t.Fatalf("legacy leftRight transform = (%v, %v, %v), want (0, -1, 1)", rotation, scaleX, scaleY)
	}
}

func TestPhysicsShapePivotSeparatesAutoAndExplicitShapes(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.Scale = 2

	config := physicConfig{Pivot: mathf.NewVec2(3, 4), Type: physicsColliderRect}
	if got, want := config.shapePivot(sprite), mathf.NewVec2(6, 8); got != want {
		t.Fatalf("explicit shape pivot = %v, want %v", got, want)
	}

	config.Type = physicsColliderAuto
	if got, want := config.shapePivot(sprite), mathf.NewVec2(80, -40); got != want {
		t.Fatalf("auto shape pivot = %v, want %v", got, want)
	}
}
