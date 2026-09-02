package spx

import (
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type polygonColliderSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	collisionCenter  mathf.Vec2
	collisionPoints  []float32
	collisionEnabled []bool
	triggerCenter    mathf.Vec2
	triggerPoints    []float32
	triggerEnabled   []bool
}

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

func TestSetPolygonColliderSyncsScaledShape(t *testing.T) {
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
