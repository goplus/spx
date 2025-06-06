package enginewrap

import (
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

type Sprite struct {
	gdx.Sprite
}

// --------------------------------------------------------------------------
// Override coordinate system-related functions to accommodate the
// difference between SPX and Godot coordinate systems (Y-axis inverted)

func (pself *Sprite) SetTriggerRect(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetTriggerRect(center, size)
}

func (pself *Sprite) SetTriggerCapsule(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetTriggerCapsule(center, size)
}

func (pself *Sprite) SetTriggerCircle(center Vec2, radius float64) {
	center.Y = -center.Y
	pself.Sprite.SetTriggerCircle(center, radius)
}

func (pself *Sprite) SetColliderRect(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetColliderRect(center, size)
}

func (pself *Sprite) SetColliderCapsule(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetColliderCapsule(center, size)
}

func (pself *Sprite) SetColliderCircle(center Vec2, radius float64) {
	center.Y = -center.Y
	pself.Sprite.SetColliderCircle(center, radius)
}
