package spx

import (
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type polygonColliderSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	collisionCenter  mathf.Vec2
	collisionPoints  []float32
	collisionEnabled []bool
	collisionLayer   int64
	collisionMask    int64
	triggerCenter    mathf.Vec2
	triggerPoints    []float32
	triggerEnabled   []bool
	triggerLayer     int64
	triggerMask      int64
}

func (m *polygonColliderSpriteMgr) SetCollisionLayer(_ pkgengine.Object, layer int64) {
	m.collisionLayer = layer
}

func (m *polygonColliderSpriteMgr) SetCollisionMask(_ pkgengine.Object, mask int64) {
	m.collisionMask = mask
}

func (m *polygonColliderSpriteMgr) SetTriggerLayer(_ pkgengine.Object, layer int64) {
	m.triggerLayer = layer
}

func (m *polygonColliderSpriteMgr) SetTriggerMask(_ pkgengine.Object, mask int64) {
	m.triggerMask = mask
}

func (m *polygonColliderSpriteMgr) SetGravityScale(pkgengine.Object, float64) {}

func (m *polygonColliderSpriteMgr) SetPhysicsMode(pkgengine.Object, int64) {}

func (m *polygonColliderSpriteMgr) SetColliderPolygon(_ pkgengine.Object, center mathf.Vec2, points pkgengine.Array) {
	m.collisionCenter = center
	m.collisionPoints = slices.Clone(points.([]float32))
}

func (m *polygonColliderSpriteMgr) SetCollisionEnabled(_ pkgengine.Object, enabled bool) {
	m.collisionEnabled = append(m.collisionEnabled, enabled)
}

func (m *polygonColliderSpriteMgr) SetTriggerPolygon(_ pkgengine.Object, center mathf.Vec2, points pkgengine.Array) {
	m.triggerCenter = center
	m.triggerPoints = slices.Clone(points.([]float32))
}

func (m *polygonColliderSpriteMgr) SetTriggerEnabled(_ pkgengine.Object, enabled bool) {
	m.triggerEnabled = append(m.triggerEnabled, enabled)
}

func newPolygonColliderTestSprite(t *testing.T) (*SpriteImpl, *polygonColliderSpriteMgr) {
	t.Helper()
	original := pkgengine.SpriteMgr
	mgr := &polygonColliderSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})

	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.Scale = 2
	sprite.runtimeState.SyncSprite = &internalengine.Sprite{
		Sprite: pkgengine.Sprite{Id: 1},
	}
	return sprite, mgr
}

func assertColliderParams(t *testing.T, sprite *SpriteImpl, isTrigger bool, wantType ColliderShapeType, want []float64) {
	t.Helper()
	typ, got := sprite.ColliderShape(isTrigger)
	if typ != wantType || !slices.Equal(got, want) {
		t.Fatalf("shape(trigger=%t) = type %d params %v, want type %d params %v", isTrigger, typ, got, wantType, want)
	}
}

func TestPhysicsLayersSurviveClone(t *testing.T) {
	sprite, mgr := newPolygonColliderTestSprite(t)
	sprite.SetCollisionLayer(2)
	sprite.SetCollisionMask(4)
	sprite.SetTriggerLayer(8)
	sprite.SetTriggerMask(16)

	if mgr.collisionLayer != 2 || mgr.collisionMask != 4 || mgr.triggerLayer != 8 || mgr.triggerMask != 16 {
		t.Fatalf("proxy layers = (%d, %d, %d, %d)", mgr.collisionLayer, mgr.collisionMask, mgr.triggerLayer, mgr.triggerMask)
	}

	clone := sprite.physics().cloneFor(&SpriteImpl{})
	if clone.collisionInfo.Layer != 2 || clone.collisionInfo.Mask != 4 || clone.triggerInfo.Layer != 8 || clone.triggerInfo.Mask != 16 {
		t.Fatalf("clone layers = (%d, %d, %d, %d)", clone.collisionInfo.Layer, clone.collisionInfo.Mask, clone.triggerInfo.Layer, clone.triggerInfo.Mask)
	}

	mgr.collisionLayer, mgr.collisionMask, mgr.triggerLayer, mgr.triggerMask = 0, 0, 0, 0
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	sprite.physics().applyPhysicsProxyConfig(sprite.runtimeState.SyncSprite)
	if mgr.collisionLayer != 2 || mgr.collisionMask != 4 || mgr.triggerLayer != 8 || mgr.triggerMask != 16 {
		t.Fatalf("replayed layers = (%d, %d, %d, %d)", mgr.collisionLayer, mgr.collisionMask, mgr.triggerLayer, mgr.triggerMask)
	}
}

func TestSetPolygonColliderSyncsScaledShape(t *testing.T) {
	sprite, mgr := newPolygonColliderTestSprite(t)
	points := []float64{-1, -2, 3, -4, 5, 6}

	sprite.physics().collisionInfo.Pivot = mathf.NewVec2(2, 3)
	if err := sprite.SetColliderShape(false, PolygonCollider, points); err != nil {
		t.Fatalf("SetColliderShape(collision) failed: %v", err)
	}
	sprite.physics().triggerInfo.Pivot = mathf.NewVec2(-2, 1)
	if err := sprite.SetColliderShape(true, PolygonCollider, points); err != nil {
		t.Fatalf("SetColliderShape(trigger) failed: %v", err)
	}

	wantPoints := []float32{-2, -4, 6, -8, 10, 12}
	if mgr.collisionCenter != mathf.NewVec2(4, 6) || !slices.Equal(mgr.collisionPoints, wantPoints) {
		t.Fatalf("collision polygon = (%v, %v), want (%v, %v)", mgr.collisionCenter, mgr.collisionPoints, mathf.NewVec2(4, 6), wantPoints)
	}
	if !slices.Equal(mgr.collisionEnabled, []bool{true}) {
		t.Fatalf("collision enabled calls = %v, want [true]", mgr.collisionEnabled)
	}
	if mgr.triggerCenter != mathf.NewVec2(-4, 2) || !slices.Equal(mgr.triggerPoints, wantPoints) {
		t.Fatalf("trigger polygon = (%v, %v), want (%v, %v)", mgr.triggerCenter, mgr.triggerPoints, mathf.NewVec2(-4, 2), wantPoints)
	}
	if !slices.Equal(mgr.triggerEnabled, []bool{true}) {
		t.Fatalf("trigger enabled calls = %v, want [true]", mgr.triggerEnabled)
	}
	if !slices.Equal(points, []float64{-1, -2, 3, -4, 5, 6}) {
		t.Fatalf("input points were mutated: %v", points)
	}

	sprite.runtimeState.Scale = 3
	sprite.updatePhysicsShapesScale()
	wantRescaledPoints := []float32{-3, -6, 9, -12, 15, 18}
	if !slices.Equal(mgr.collisionPoints, wantRescaledPoints) || !slices.Equal(mgr.triggerPoints, wantRescaledPoints) {
		t.Fatalf("rescaled polygons = (%v, %v), want %v", mgr.collisionPoints, mgr.triggerPoints, wantRescaledPoints)
	}
}

func TestSetColliderShapeRejectsInvalidParamsAtomically(t *testing.T) {
	sprite, mgr := newPolygonColliderTestSprite(t)
	valid := []float64{-1, -2, 3, -4, 5, 6}
	if err := sprite.SetColliderShape(false, PolygonCollider, valid); err != nil {
		t.Fatalf("SetColliderShape(collision) failed: %v", err)
	}
	if err := sprite.SetColliderShape(true, PolygonCollider, valid); err != nil {
		t.Fatalf("SetColliderShape(trigger) failed: %v", err)
	}

	collisionType, collisionParams := sprite.ColliderShape(false)
	triggerType, triggerParams := sprite.ColliderShape(true)
	collisionPoints := slices.Clone(mgr.collisionPoints)
	triggerPoints := slices.Clone(mgr.triggerPoints)
	collisionEnabledCalls := slices.Clone(mgr.collisionEnabled)
	triggerEnabledCalls := slices.Clone(mgr.triggerEnabled)
	collisionParamsStorage := &sprite.physics().collisionInfo.Params[0]
	triggerParamsStorage := &sprite.physics().triggerInfo.Params[0]

	if err := sprite.SetColliderShape(false, PolygonCollider, []float64{1, 2, 3, 4, 5}); err == nil {
		t.Fatal("SetColliderShape(collision) accepted an invalid polygon")
	}
	if err := sprite.SetColliderShape(true, PolygonCollider, []float64{1, 2, 3, 4, 5}); err == nil {
		t.Fatal("SetColliderShape(trigger) accepted an invalid polygon")
	}

	gotCollisionType, gotCollisionParams := sprite.ColliderShape(false)
	if gotCollisionType != collisionType || !slices.Equal(gotCollisionParams, collisionParams) {
		t.Fatalf("collision shape changed after rejection: type=%d params=%v, want type=%d params=%v", gotCollisionType, gotCollisionParams, collisionType, collisionParams)
	}
	gotTriggerType, gotTriggerParams := sprite.ColliderShape(true)
	if gotTriggerType != triggerType || !slices.Equal(gotTriggerParams, triggerParams) {
		t.Fatalf("trigger shape changed after rejection: type=%d params=%v, want type=%d params=%v", gotTriggerType, gotTriggerParams, triggerType, triggerParams)
	}
	if &sprite.physics().collisionInfo.Params[0] != collisionParamsStorage || &sprite.physics().triggerInfo.Params[0] != triggerParamsStorage {
		t.Fatal("rejected shape replaced live parameter storage")
	}
	if !slices.Equal(mgr.collisionPoints, collisionPoints) || !slices.Equal(mgr.triggerPoints, triggerPoints) {
		t.Fatalf("engine proxy points changed after rejection: collision=%v trigger=%v", mgr.collisionPoints, mgr.triggerPoints)
	}
	if !slices.Equal(mgr.collisionEnabled, collisionEnabledCalls) || !slices.Equal(mgr.triggerEnabled, triggerEnabledCalls) {
		t.Fatalf("engine proxy enabled calls changed after rejection: collision=%v trigger=%v", mgr.collisionEnabled, mgr.triggerEnabled)
	}
}

func TestSetColliderShapeCopiesInputParams(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	tests := []struct {
		name      string
		isTrigger bool
		params    []float64
	}{
		{name: "collision", params: []float64{-1, -2, 3, -4, 5, 6}},
		{name: "trigger", isTrigger: true, params: []float64{1, 2, 3, 4, 5, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := slices.Clone(tt.params)
			if err := sprite.SetColliderShape(tt.isTrigger, PolygonCollider, tt.params); err != nil {
				t.Fatalf("SetColliderShape failed: %v", err)
			}
			tt.params[0] = 99
			assertColliderParams(t, sprite, tt.isTrigger, PolygonCollider, want)
		})
	}
}

func TestPhysicsInitializationCopiesShapeParams(t *testing.T) {
	collisionParams := []float64{-1, -2, 3, -4, 5, 6}
	triggerParams := []float64{1, 2, 3, 4, 5, 6}
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		CollisionShapeType:   "polygon",
		CollisionShapeParams: collisionParams,
		TriggerShapeType:     "polygon",
		TriggerShapeParams:   triggerParams,
	})

	collisionParams[0] = 99
	triggerParams[0] = 101
	assertColliderParams(t, sprite, false, PolygonCollider, []float64{-1, -2, 3, -4, 5, 6})
	assertColliderParams(t, sprite, true, PolygonCollider, []float64{1, 2, 3, 4, 5, 6})
}
