package engine

import (
	"github.com/goplus/spx/internal/enginewrap"
	gdx "github.com/realdream-ai/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

// !!!Warning all method belong to this class can only be called in main thread
type Sprite struct {
	enginewrap.Sprite
	x, y    float64
	Name    string
	PicPath string
	Target  interface{}
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

func (pself *Sprite) UpdateColor(val float64) {
	WaitMainThread(func() {
		// oldColor := pself.GetColor()
		//val range 0..100 => 0...1
		// val = val / 100.0
		// newColor := oldColor.Add(NewColor(val, val, val, 1))

		// pself.SetColor(newColor)

		pself.SetMaterialParams("color_amount", val)
	})
}

func (pself *Sprite) UpdateAlpha(val float64) {
	WaitMainThread(func() {
		pself.SetMaterialParams("alpha_amount", val)
	})
}

func (pself *Sprite) UpdateBrightness(val float64) {
	WaitMainThread(func() {
		pself.SetMaterialParams("brightness_amount", val)
	})
}

func (pself *Sprite) UpdateMosaic(val float64) {
	WaitMainThread(func() {
		pself.SetMaterialParams("mosaic_amount", val)
	})
}

func (pself *Sprite) UpdateWhirl(val float64) {
	WaitMainThread(func() {
		pself.SetMaterialParams("whirl_amount", val)
	})
}

func (pself *Sprite) UpdateFishEye(val float64) {
	WaitMainThread(func() {
		pself.SetMaterialParams("fisheye_amount", val)
	})
}

func (pself *Sprite) UpdateUVEffect(val float64) {
	WaitMainThread(func() {
		pself.SetMaterialParams("uv_amount", val)
	})
}

func (pself *Sprite) ClearGraphEffects() {
	WaitMainThread(func() {
		pself.SetMaterialParams("color_amount", 0)
		pself.SetMaterialParams("alpha_amount", 0)
		pself.SetMaterialParams("brightness_amount", 0)
		pself.SetMaterialParams("mosaic_amount", 0)
		pself.SetMaterialParams("whirl_amount", 0)
		pself.SetMaterialParams("fisheye_amount", 0)
		pself.SetMaterialParams("uv_amount", 0)
	})
}

func (pself *Sprite) OnTriggerEnter(target gdx.ISpriter) {
	sprite, ok := target.(*Sprite)
	if ok {
		triggerEventsTemp = append(triggerEventsTemp, TriggerEvent{Src: pself, Dst: sprite})
	}
}
