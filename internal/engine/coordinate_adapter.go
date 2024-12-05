package engine

import (
	. "github.com/realdream-ai/mathf"
)

// --------------------------------------------------------------------------
// Override coordinate system-related functions to accommodate the
// difference between SPX and Godot coordinate systems (Y-axis inverted)

func (pself *ProxySprite) SetTriggerRect(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetTriggerRect(center, size)
}

func (pself *ProxySprite) SetTriggerCapsule(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetTriggerCapsule(center, size)
}

func (pself *ProxySprite) SetTriggerCircle(center Vec2, radius float64) {
	center.Y = -center.Y
	pself.Sprite.SetTriggerCircle(center, radius)
}

func (pself *ProxySprite) SetColliderRect(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetColliderRect(center, size)
}

func (pself *ProxySprite) SetColliderCapsule(center Vec2, size Vec2) {
	center.Y = -center.Y
	pself.Sprite.SetColliderCapsule(center, size)
}

func (pself *ProxySprite) SetColliderCircle(center Vec2, radius float64) {
	center.Y = -center.Y
	pself.Sprite.SetColliderCircle(center, radius)
}
