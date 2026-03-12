package engine

import (
	"math"

	. "github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/enginewrap"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

var (
	windowScale float64
)

func SetWindowScale(scale float64) {
	windowScale = scale
}

type UiNode = gdx.UiNode

func NewUiNode[T any]() *T {
	var ret *T
	WaitMainThread(func() {
		ret = CreateEngineUIForType[T]("")
	})
	return ret
}

func NewBackdropProxy(obj any, path string, renderScale float64) *Sprite {
	var ret *Sprite
	WaitMainThread(func() {
		ret = CreateBackdropForType[Sprite]()
		ret.Target = obj
		ret.SetZIndex(-1)
		ret.DisablePhysic()
		ret.UpdateTexture(path, renderScale, true)
	})
	return ret
}

func BridgeNewSprite(obj any, pos Vec2) *Sprite {
	syncSprite := CreateEmptySpriteForType[Sprite](pos)
	syncSprite.Target = obj
	return syncSprite
}

func BridgeBindUI[T any](parentNode Object, path string) *T {
	return BindUIForType[T](parentNode, path)
}

func ReadAllText(path string) string {
	return Managers().ResMgr.ReadAllText(path)
}

func HasFile(path string) bool {
	return Managers().ResMgr.HasFile(path)
}

func SetDebugMode(isDebug bool) {
	Managers().PlatformMgr.SetDebugMode(isDebug)
}

func SetDefaultFont(path string) {
	Managers().ResMgr.SetDefaultFont(path)
}

// BridgeSetCameraPosition updates the camera from engine callback code without
// dispatching to the main thread.
func BridgeSetCameraPosition(pos Vec2) {
	Managers().CameraMgr.SetCameraPosition(NewVec2(pos.X, -pos.Y))
}

// BridgeScreenToWorld converts screen coordinates from engine callback code
// without dispatching to the main thread.
func BridgeScreenToWorld(pos Vec2) Vec2 {
	mgr := Managers()
	zoom := mgr.CameraMgr.GetCameraZoom().X
	camPos := bridgeGetCameraPosition(mgr)
	return pos.Divf(zoom / windowScale).Add(camPos.Mulf(windowScale))
}

// BridgeWorldToScreen converts world coordinates from engine callback code
// without dispatching to the main thread.
func BridgeWorldToScreen(pos Vec2) Vec2 {
	mgr := Managers()
	zoom := mgr.CameraMgr.GetCameraZoom().X
	camPos := bridgeGetCameraPosition(mgr)
	return pos.Sub(camPos.Mulf(windowScale)).Mulf(zoom / windowScale)
}

// BridgeGetCameraPosition reads the camera position from engine callback code
// without dispatching to the main thread.
func BridgeGetCameraPosition() Vec2 {
	return bridgeGetCameraPosition(Managers())
}

func bridgeGetCameraPosition(managers *enginewrap.EngineManagers) Vec2 {
	pos := managers.CameraMgr.GetCameraPosition()
	pos.Y = -pos.Y
	return pos
}

// ScreenToWorld converts screen coordinates and dispatches to the main thread
// when needed, so it is safe to call from any goroutine.
func ScreenToWorld(pos Vec2) Vec2 {
	var ret Vec2
	WaitMainThread(func() {
		ret = BridgeScreenToWorld(pos)
	})
	return ret
}

// WorldToScreen converts world coordinates and dispatches to the main thread
// when needed, so it is safe to call from any goroutine.
func WorldToScreen(pos Vec2) Vec2 {
	var ret Vec2
	WaitMainThread(func() {
		ret = BridgeWorldToScreen(pos)
	})
	return ret
}

// ClearAllSprites dispatches sprite teardown to the main thread and is safe to
// call from any goroutine.
func ClearAllSprites() {
	WaitMainThread(func() {
		clearAllSprites()
	})
}

func GetSprite(id gdx.Object) *Sprite {
	target := lookupSprite(id)
	sprite, ok := target.(*Sprite)
	if ok {
		return sprite
	}
	return nil
}

func GetFPS() float64 {
	return fps
}

func DegToRad(degree float64) float64 {
	return gdx.DegToRad(degree)
}

func RadToDeg(radians float64) float64 {
	return gdx.RadToDeg(radians)
}

func Sincos(rad float64) Vec2 {
	return NewVec2(math.Sincos(rad))
}

func F64Tof32(slice []float64) []float32 {
	if slice == nil {
		return nil
	}
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func F32Tof64(slice []float32) []float64 {
	if slice == nil {
		return nil
	}
	out := make([]float64, len(slice))
	for i, v := range slice {
		out[i] = float64(v)
	}
	return out
}
