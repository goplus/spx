package engine

import (
	. "github.com/realdream-ai/gdspx/pkg/engine"
)

type ProxySprite struct {
	Sprite
	x, y    float64
	Name    string
	PicPath string
	Target  interface{}
}

func NewSpriteProxy(obj interface{}) *ProxySprite {
	proxy := CreateEmptySprite[ProxySprite]()
	proxy.Target = obj
	return proxy
}

func (pself *ProxySprite) UpdateTexture(path string, renderScale float64) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	pself.SetTexture(pself.PicPath)
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}
func (pself *ProxySprite) UpdateTextureAltas(path string, rect2 Rect2, renderScale float64) {
	if path == "" {
		return
	}
	resPath := ToAssetPath(path)
	pself.PicPath = resPath
	pself.SetTextureAltas(pself.PicPath, rect2)
	pself.SetRenderScale(NewVec2(renderScale, renderScale))
}

func (pself *ProxySprite) UpdateTransform(x, y float64, rot float64, scale64 float64, isSync bool) {
	pself.x = x
	pself.y = y
	rad := HeadingToRad(rot)
	pos := Vec2{X: float32(x), Y: float32(y)}
	scale := float32(scale64)
	if !isSync {
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

func (pself *ProxySprite) OnTriggerEnter(target ISpriter) {
	sprite, ok := target.(*ProxySprite)
	if ok {
		triggerEventsTemp = append(triggerEventsTemp, TriggerEvent{Src: pself, Dst: sprite})
	}
}
