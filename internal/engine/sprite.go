package engine

import (
	. "github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/enginewrap"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

// !!!Warning all method belong to this class can only be called in main thread
type Sprite struct {
	enginewrap.Sprite
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
func (pself *Sprite) UpdateTextureAltas(path string, rect2 Rect2, renderScale float64, isUpdateTexture bool) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	if isUpdateTexture {
		pself.SetTextureAltas(pself.PicPath, rect2)
	}
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}

func (pself *Sprite) UpdateTransform(x, y float64, rot float64, scale64 float64, offsetX, offsetY float64, isSync bool) {
	pself.x = x
	pself.y = y
	rad := DegToRad(rot)
	pos := Vec2{X: float64(x), Y: float64(y)}
	posOffset := Vec2{X: float64(offsetX), Y: float64(offsetY)}
	scale := float64(scale64)
	if isSync {
		pself.SetPosition(pos)
		pself.SetRotation(rad)
		pself.SetScale(NewVec2(scale, 1))
		pself.SetPivot(posOffset)
	} else {
		WaitMainThread(func() {
			pself.SetPosition(pos)
			pself.SetRotation(rad)
			pself.SetScale(NewVec2(scale, 1))
			pself.SetPivot(posOffset)
		})
	}
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
