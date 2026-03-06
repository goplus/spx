package engine

import (
	"io"
	"math"

	. "github.com/goplus/spbase/mathf"

	"github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine/platform"
	gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

func RegisterFileSystem(fs fs.Dir) {
	if platform.IsWeb() {
		RegisterIoReader(func(file string, length int) ([]byte, error) {
			rc, err := fs.Open(file)
			if err != nil {
				return nil, err
			}
			buf := make([]byte, length)
			defer rc.Close()

			n, err := io.ReadFull(rc, buf)
			if err != nil {
				if err == io.ErrUnexpectedEOF {
					return buf[:n], nil
				}
				return buf[:n], err
			}
			return buf[:n], nil
		})
	}
}

// =============== factory ===================

func NewUiNode[T any]() *T {
	var _ret1 *T
	WaitMainThread(func() {
		_ret1 = gdx.CreateEngineUI[T]("")
	})
	return _ret1
}

func NewBackdropProxy(obj any, path string, renderScale float64) *Sprite {
	var _ret1 *Sprite
	WaitMainThread(func() {
		_ret1 = gdx.CreateBackdrop[Sprite]()
		_ret1.Target = obj
		_ret1.SetZIndex(-1)
		_ret1.DisablePhysic()
		_ret1.UpdateTexture(path, renderScale, true)
	})
	return _ret1
}

func ReadAllText(path string) string {
	return resMgr.ReadAllText(path)
}

func HasFile(path string) bool {
	return resMgr.HasFile(path)
}

func SetDebugMode(isDebug bool) {
	platformMgr.SetDebugMode(isDebug)
}
func SetDefaultFont(path string) {
	resMgr.SetDefaultFont(path)
}

func ScreenToWorld(pos Vec2) Vec2 {
	var _ret1 Vec2
	WaitMainThread(func() {
		_ret1 = MainThreadScreenToWorld(pos)
	})
	return _ret1
}
func WorldToScreen(pos Vec2) Vec2 {
	var _ret1 Vec2
	WaitMainThread(func() {
		_ret1 = MainThreadWorldToScreen(pos)
	})
	return _ret1
}

func ClearAllSprites() {
	WaitMainThread(func() {
		gdx.ClearAllSprites()
	})
}

func GetSprite(id gdx.Object) *Sprite {
	target := gdx.GetSprite(id)
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
		return []float32{}
	}
	out := make([]float32, len(slice))
	for i, v := range slice {
		out[i] = float32(v)
	}
	return out
}

func F32Tof64(slice []float32) []float64 {
	if slice == nil {
		return []float64{}
	}
	out := make([]float64, len(slice))
	for i, v := range slice {
		out[i] = float64(v)
	}
	return out
}
