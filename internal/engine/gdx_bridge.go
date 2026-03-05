package engine

import (
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var (
	windowScale float64
)

func SetWindowScale(scale float64) {
	windowScale = scale
}

type UiNode = gdx.UiNode

func MainThreadNewSprite(obj any, pos Vec2) *Sprite {
	syncSprite := gdx.CreateEmptySprite[Sprite](pos)
	syncSprite.Target = obj
	return syncSprite
}

func MainThreadBindUI[T any](parentNode gdx.Object, path string) *T {
	return gdx.BindUI[T](parentNode, path)
}

func MainThreadGetTimeScale() float64 {
	return gdx.PlatformMgr.GetTimeScale()
}

func MainThreadGetMousePos() Vec2 {
	return gdx.InputMgr.GetGlobalMousePos()
}

func MainThreadSetCameraPosition(pos Vec2) {
	gdx.CameraMgr.SetCameraPosition(NewVec2(pos.X, -pos.Y))
}

func MainThreadScreenToWorld(pos Vec2) Vec2 {
	zoom := gdx.CameraMgr.GetCameraZoom().X
	camPos := MainThreadGetCameraPosition()
	return pos.Divf(zoom / windowScale).Add(camPos.Mulf(windowScale))
}

func MainThreadWorldToScreen(pos Vec2) Vec2 {
	zoom := gdx.CameraMgr.GetCameraZoom().X
	camPos := MainThreadGetCameraPosition()
	return pos.Sub(camPos.Mulf(windowScale)).Mulf(zoom / windowScale)
}

func MainThreadGetBoundFromAlpha(assetPath string) Rect2 {
	return gdx.ResMgr.GetBoundFromAlpha(assetPath)
}

func MainThreadGetCameraPosition() Vec2 {
	pos := gdx.CameraMgr.GetCameraPosition()
	pos.Y = -pos.Y
	return pos
}

func MainThreadGetCameraZoom() Vec2 {
	return gdx.CameraMgr.GetCameraZoom()
}

func MainThreadGetViewportRect() Rect2 {
	return gdx.CameraMgr.GetViewportRect()
}

func MainThreadUiSetVisible(obj Object, visible bool) {
	gdx.UiMgr.SetVisible(obj, visible)
}

func MainThreadUiSetScale(obj Object, scale Vec2) {
	gdx.UiMgr.SetScale(obj, scale)
}

func MainThreadUiSetGlobalPosition(obj Object, pos Vec2) {
	gdx.UiMgr.SetGlobalPosition(obj, pos)
}

func MainThreadUiSetPosition(obj Object, pos Vec2) {
	gdx.UiMgr.SetPosition(obj, pos)
}

func MainThreadUiSetSize(obj Object, size Vec2) {
	gdx.UiMgr.SetSize(obj, size)
}

func MainThreadUiSetText(obj Object, text string) {
	gdx.UiMgr.SetText(obj, text)
}

func MainThreadUiSetColor(obj Object, color Color) {
	gdx.UiMgr.SetColor(obj, color)
}
