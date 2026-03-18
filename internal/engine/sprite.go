package engine

import (
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// !!!Warning all method belong to this class can only be called in main thread
type Sprite struct {
	gdx.Sprite
	x, y    float64
	Name    string
	PicPath string
	Target  any
}

func (pself *Sprite) UpdateTexture(path string, renderScale float64, isUpdateTexture bool) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	if isUpdateTexture {
		pself.SetTexture(pself.PicPath)
	}
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}

func (pself *Sprite) UpdateTextureAtlas(path string, rect2 Rect2, renderScale float64, isUpdateTexture bool) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	if isUpdateTexture {
		pself.SetTextureAtlas(pself.PicPath, rect2)
	}
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}

func (pself *Sprite) OnTriggerEnter(target gdx.ISpriter) {
	sprite, ok := target.(*Sprite)
	if ok {
		triggerEventsTemp = append(triggerEventsTemp, TriggerEvent{Src: pself, Dst: sprite})
	}
}

func (pself *Sprite) RegisterOnAnimationLooped(f func()) {
	pself.Sprite.OnAnimationLoopedEvent.Subscribe(f)
}

func (pself *Sprite) UnRegisterOnAnimationLooped() {
	pself.Sprite.OnAnimationLoopedEvent.UnsubscribeAll()
}

func (pself *Sprite) RegisterOnAnimationFinished(f func()) {
	pself.Sprite.OnAnimationFinishedEvent.Subscribe(f)
}

func (pself *Sprite) UnRegisterOnAnimationFinished() {
	pself.Sprite.OnAnimationFinishedEvent.UnsubscribeAll()
}

// --------------------------------------------------------------------------
// Override coordinate system-related functions to accommodate the
// difference between spx and Godot coordinate systems (Y-axis inverted)

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
