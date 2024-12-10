package engine

import (
	gdx "github.com/realdream-ai/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

// !!!Warning these method can only be called in main thread
func SyncNewSprite(obj interface{}) *Sprite {
	syncSprite := gdx.CreateEmptySprite[Sprite]()
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
	return gdx.InputMgr.GetMousePos()
}

func SyncSetCameraPosition(pos Vec2) {
	gdx.CameraMgr.SetCameraPosition(NewVec2(pos.X, -pos.Y))
}

func SyncScreenToWorld(pos Vec2) Vec2 {
	camPos := gdx.CameraMgr.GetCameraPosition()
	camPos.Y *= -1
	return pos.Add(camPos)
}

func SyncWorldToScreen(pos Vec2) Vec2 {
	camPos := gdx.CameraMgr.GetCameraPosition()
	camPos.Y *= -1
	return pos.Sub(camPos)
}

func SyncGetBoundFromAlpha(assetPath string) Rect2 {
	return gdx.ResMgr.GetBoundFromAlpha(assetPath)
}
