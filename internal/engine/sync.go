package engine

import (
	. "github.com/realdream-ai/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

// =============== factory ===================
func SyncCreateUiNode[T any](path string) *T {
	var _ret1 *T
	WaitMainThread(func() {
		_ret1 = CreateUI[T](path)
	})
	return _ret1
}
func SyncCreateEngineUiNode[T any](path string) *T {
	var _ret1 *T
	WaitMainThread(func() {
		_ret1 = CreateEngineUI[T](path)
	})
	return _ret1
}

func SyncCreateSprite[T any]() *T {
	var _ret1 *T
	WaitMainThread(func() {
		_ret1 = CreateSprite[T]()
	})
	return _ret1
}

func SyncCreateEmptySprite[T any]() *T {
	var _ret1 *T
	WaitMainThread(func() {
		_ret1 = CreateEmptySprite[T]()
	})
	return _ret1
}

func SyncNewBackdropProxy(obj interface{}, path string, renderScale float64) *ProxySprite {
	var _ret1 *ProxySprite
	WaitMainThread(func() {
		_ret1 = newBackdropProxy(obj, path, renderScale)
	})
	return _ret1
}

func newBackdropProxy(obj interface{}, path string, renderScale float64) *ProxySprite {
	ret := CreateEmptySprite[ProxySprite]()
	ret.Target = obj
	ret.SetZIndex(-1)
	ret.DisablePhysic()
	ret.UpdateTexture(path, renderScale)
	return ret
}

// =============== input ===================
func SyncInputMousePressed() bool {
	return SyncInputGetMouseState(0) || SyncInputGetMouseState(1)
}

// =============== window ===================
func SyncSetRunnableOnUnfocused(flag bool) {
	if !flag {
		println("TODO tanjp SyncSetRunnableOnUnfocused")
	}
}

func SyncReadAllText(path string) string {
	return SyncResReadAllText(path)
}

// =============== setting ===================

func SyncSetDebugMode(isDebug bool) {
	SyncPlatformSetDebugMode(isDebug)
}

// =============== setting ===================
func ScreenToWorld(x, y float64) (float64, float64) {
	camPos := CameraMgr.GetCameraPosition()
	posX, posY := float64(camPos.X), -float64(camPos.Y)
	x += posX
	y += posY
	return x, y
}

func WorldToScreen(x, y float64) (float64, float64) {
	camPos := CameraMgr.GetCameraPosition()
	posX, posY := float64(camPos.X), -float64(camPos.Y)
	x -= posX
	y -= posY
	return x, y
}

func SyncScreenToWorld(x, y float64) (float64, float64) {
	var _ret1, _ret2 float64
	WaitMainThread(func() {
		_ret1, _ret2 = ScreenToWorld(x, y)
	})
	return _ret1, _ret2
}
func SyncWorldToScreen(x, y float64) (float64, float64) {
	var _ret1, _ret2 float64
	WaitMainThread(func() {
		_ret1, _ret2 = WorldToScreen(x, y)
	})
	return _ret1, _ret2
}

func SyncGetCameraLocalPosition(x, y float64) (float64, float64) {
	posX, posY := SyncGetCameraPosition()
	x -= posX
	y -= posY
	return x, y
}
func SyncGetCameraPosition() (float64, float64) {
	pos := SyncCameraGetCameraPosition()
	return float64(pos.X), -float64(pos.Y)
}
func SyncSetCameraPosition(x, y float64) {
	SyncCameraSetCameraPosition(NewVec2(float64(x), -float64(y)))
}

func SyncReloadScene() {
	WaitMainThread(func() {
		ClearAllSprites()
	})
}
