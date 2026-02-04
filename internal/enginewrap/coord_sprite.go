package enginewrap

import (
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

type Sprite struct {
	gdx.Sprite
}

// --------------------------------------------------------------------------
// Override coordinate system-related functions to accommodate the
// difference between SPX and Godot coordinate systems (Y-axis inverted)

func (pself *Sprite) SetColliderShapeRect(isTrigger bool, center Vec2, size Vec2) {
	center.Y = -center.Y
	if isTrigger {
		pself.Sprite.SetTriggerRect(center, size)
	} else {
		pself.Sprite.SetColliderRect(center, size)
	}
}

func (pself *Sprite) SetColliderShapeCircle(isTrigger bool, center Vec2, radius float64) {
	center.Y = -center.Y
	if isTrigger {
		pself.Sprite.SetTriggerCircle(center, radius)
	} else {
		pself.Sprite.SetColliderCircle(center, radius)
	}
}

func (pself *Sprite) SetColliderShapeCapsule(isTrigger bool, center Vec2, size Vec2) {
	center.Y = -center.Y
	if isTrigger {
		pself.Sprite.SetTriggerCapsule(center, size)
	} else {
		pself.Sprite.SetColliderCapsule(center, size)
	}
}

func (pself *Sprite) SetColliderShapePolygon(isTrigger bool, center Vec2, points []float64) {
	center.Y = -center.Y
	points32 := F64Tof32(points)
	if isTrigger {
		pself.Sprite.SetTriggerPolygon(center, points32)
	} else {
		pself.Sprite.SetColliderPolygon(center, points32)
	}
}

func (pself *Sprite) SetColliderEnabled(isTrigger bool, enabled bool) {
	if isTrigger {
		pself.Sprite.SetTriggerEnabled(enabled)
	} else {
		pself.Sprite.SetCollisionEnabled(enabled)
	}
}
