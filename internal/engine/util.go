package engine

import (
	"math"

	gdx "github.com/realdream-ai/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

// =============== factory ===================

func NewUiNode[T any]() *T {
	var _ret1 *T
	WaitMainThread(func() {
		_ret1 = gdx.CreateEngineUI[T]("")
	})
	return _ret1
}

func NewBackdropProxy(obj interface{}, path string, renderScale float64) *Sprite {
	var _ret1 *Sprite
	WaitMainThread(func() {
		_ret1 = gdx.CreateEmptySprite[Sprite]()
		_ret1.Target = obj
		_ret1.SetZIndex(-1)
		_ret1.DisablePhysic()
		_ret1.UpdateTexture(path, renderScale)
	})
	return _ret1
}

func ReadAllText(path string) string {
	return resMgr.ReadAllText(path)
}

// =============== setting ===================

func SetDebugMode(isDebug bool) {
	platformMgr.SetDebugMode(isDebug)
}

// =============== setting ===================

func ScreenToWorld(pos Vec2) Vec2 {
	var _ret1 Vec2
	WaitMainThread(func() {
		_ret1 = SyncScreenToWorld(pos)
	})
	return _ret1
}
func WorldToScreen(pos Vec2) Vec2 {
	var _ret1 Vec2
	WaitMainThread(func() {
		_ret1 = SyncWorldToScreen(pos)
	})
	return _ret1
}

func ReloadScene() {
	WaitMainThread(func() {
		gdx.ClearAllSprites()
	})
}

func GetFPS() float64 {
	return fps
}

func DegToRad(p_y float64) float64 {
	return p_y * (gdx.Math_PI / 180.0)
}
func Sincos(rad float64) Vec2 {
	return NewVec2(math.Sincos(rad))
}
