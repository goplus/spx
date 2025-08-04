package engine

import (
	"fmt"
	"io"
	"math"

	"log"

	"github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine/platform"
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
	. "github.com/realdream-ai/mathf"
)

var supportedFileTypes = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".svg":  true,
	".webp": true,
	".mp3":  true,
	".wav":  true,
}

// check invalid files
var checkedAssetFiles = make(map[string]bool)

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

func CheckAssetFile(rawPath string) {
	if checkedAssetFiles[rawPath] {
		return
	}
	checkedAssetFiles[rawPath] = true
	path := ToAssetPath(rawPath)
	if platform.IsWeb() {
		path = path[7:] // remove "assets/"
	}
	info := GetFileFormat(path)
	if !info.IsCorrect {
		supportStr := ""
		if !supportedFileTypes[info.Extension] {
			supportStr = ", \n and the current engine does not support this file type (" + info.Extension + "). "
		}
		msg := fmt.Sprintf("ERROR: The file (%s) has an incorrect extension, its actual format is %s"+supportStr, path, info.Extension)
		log.Println(msg)
	} else if !supportedFileTypes[info.Extension] {
		msg := fmt.Sprintf("ERROR: The file (%s) has an incorrect extension, current engine does not support this file type (%s). ", path, info.Extension)
		log.Println(msg)
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
		_ret1 = gdx.CreateEmptySprite[Sprite]()
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

// =============== setting ===================

func SetDebugMode(isDebug bool) {
	platformMgr.SetDebugMode(isDebug)
}
func SetDefaultFont(path string) {
	resMgr.SetDefaultFont(path)
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
