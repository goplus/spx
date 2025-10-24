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

// !!!Warning these method can only be called in main thread
func SyncNewSprite(obj any, pos Vec2) *Sprite {
	syncSprite := gdx.CreateEmptySprite[Sprite](pos)
	syncSprite.Target = obj
	return syncSprite
}

func SyncBindUI[T any](parentNode gdx.Object, path string) *T {
	return gdx.BindUI[T](parentNode, path)
}
func SyncGetTimeScale() float64 {
	return gdx.PlatformMgr.GetTimeScale()
}
func SyncGetMousePos() Vec2 {
	return inputMgr.SyncGetGlobalMousePos()
}

func SyncSetCameraPosition(pos Vec2) {
	cameraMgr.SyncSetCameraPosition(pos)
}

func SyncScreenToWorld(pos Vec2) Vec2 {
	zoom := gdx.CameraMgr.GetCameraZoom().X
	camPos := cameraMgr.SyncGetCameraPosition()
	return pos.Divf(zoom / windowScale).Add(camPos.Mulf(windowScale))
}

func SyncWorldToScreen(pos Vec2) Vec2 {
	zoom := gdx.CameraMgr.GetCameraZoom().X
	camPos := cameraMgr.SyncGetCameraPosition()
	return pos.Sub(camPos.Mulf(windowScale)).Mulf(zoom / windowScale)
}

func SyncGetBoundFromAlpha(assetPath string) Rect2 {
	return gdx.ResMgr.GetBoundFromAlpha(assetPath)
}
