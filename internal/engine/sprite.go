package engine

import (
	"github.com/goplus/spx/v2/internal/enginewrap"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

// !!!Warning all method belong to this class can only be called in main thread
type Sprite struct {
	enginewrap.Sprite
	x, y    float64
	Name    string
	PicPath string
	Target  any
}

func (pself *Sprite) UpdateTexture(path string, renderScale float64) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	pself.SetTexture(pself.PicPath)
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}
func (pself *Sprite) UpdateTextureAltas(path string, rect2 Rect2, renderScale float64) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	pself.SetTextureAltas(pself.PicPath, rect2)
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}

func (pself *Sprite) UpdateTransform(x, y float64, rot float64, scale64 float64, isSync bool) {
	pself.x = x
	pself.y = y
	rad := DegToRad(rot)
	pos := Vec2{X: float64(x), Y: float64(y)}
	scale := float64(scale64)
	if isSync {
		pself.SetPosition(pos)
		pself.SetRotation(rad)
		pself.SetScaleX(scale)
	} else {
		WaitMainThread(func() {
			pself.SetPosition(pos)
			pself.SetRotation(rad)
			pself.SetScaleX(scale)
		})
	}
}

func (pself *Sprite) OnTriggerEnter(target gdx.ISpriter) {
	sprite, ok := target.(*Sprite)
	if ok {
		triggerEventsTemp = append(triggerEventsTemp, TriggerEvent{Src: pself, Dst: sprite})
	}
}
