package gdengine

import (
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/goplus/spx/v3/internal/gdengine/binding/facade"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type callbackTestRuntime struct {
	engine.RuntimeBridge
	sprites map[engine.Object]engine.ISpriter
	calls   *[]string
}

func (r *callbackTestRuntime) Sprites() map[engine.Object]engine.ISpriter {
	return r.sprites
}

func (r *callbackTestRuntime) AdvanceTimeSinceGameStart(delta float64) float64 {
	*r.calls = append(*r.calls, fmt.Sprintf("advance:%g", delta))
	return delta
}

func (r *callbackTestRuntime) InternalUpdateEngine(delta float64) {
	*r.calls = append(*r.calls, fmt.Sprintf("runtime:%g", delta))
}

type callbackTestSprite struct {
	engine.ISpriter
	calls *[]string
}

func (s *callbackTestSprite) OnUpdate(delta float64) {
	*s.calls = append(*s.calls, fmt.Sprintf("sprite-update:%g", delta))
}

func (s *callbackTestSprite) OnFixedUpdate(delta float64) {
	*s.calls = append(*s.calls, fmt.Sprintf("sprite-fixed:%g", delta))
}

func (s *callbackTestSprite) OnDestroy() {
	*s.calls = append(*s.calls, "sprite-destroy")
}

func TestEngineCallbacksPreserveRuntimeLifecycle(t *testing.T) {
	previousCallbacks, previousSprites := coreCallbacks, sprites
	previousLink, previousInterpreter := activeLink, isWebIntepreterMode
	previousAudio, previousCamera, previousDebug := engine.AudioMgr, engine.CameraMgr, engine.DebugMgr
	previousExt, previousInput, previousNavigation := engine.ExtMgr, engine.InputMgr, engine.NavigationMgr
	previousPen, previousPhysics, previousPlatform := engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr
	previousRes, previousScene, previousSprite := engine.ResMgr, engine.SceneMgr, engine.SpriteMgr
	previousTilemap, previousTilemapparser, previousUI := engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr
	previousDelta, _ := itime.FixedDeltaTime()
	t.Cleanup(func() {
		coreCallbacks, sprites = previousCallbacks, previousSprites
		activeLink, isWebIntepreterMode = previousLink, previousInterpreter
		engine.AudioMgr, engine.CameraMgr, engine.DebugMgr = previousAudio, previousCamera, previousDebug
		engine.ExtMgr, engine.InputMgr, engine.NavigationMgr = previousExt, previousInput, previousNavigation
		engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr = previousPen, previousPhysics, previousPlatform
		engine.ResMgr, engine.SceneMgr, engine.SpriteMgr = previousRes, previousScene, previousSprite
		engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr = previousTilemap, previousTilemapparser, previousUI
		if runtime.GOOS != "js" {
			facade.RegisterCallbacks(bindCallbacks())
		}
		itime.SetFixedDeltaTime(previousDelta)
		engine.SetRuntimeBridge(nil)
	})
	if activeLink != nil {
		t.Fatal("lifecycle fixture requires no active link")
	}

	for _, fixed := range []float64{0, 0.1} {
		for _, withCore := range []bool{false, true} {
			t.Run(fmt.Sprintf("fixed=%g/core=%v", fixed, withCore), func(t *testing.T) {
				var calls, want []string
				sprite := &callbackTestSprite{calls: &calls}
				engine.SetRuntimeBridge(&callbackTestRuntime{
					sprites: map[engine.Object]engine.ISpriter{1: sprite}, calls: &calls,
				})
				itime.SetFixedDeltaTime(fixed)
				core := engine.CoreCallbackInfo{}
				if withCore {
					core = engine.CoreCallbackInfo{
						OnEngineStart:       func() { calls = append(calls, "core-start") },
						OnEngineUpdate:      func(delta float64) { calls = append(calls, fmt.Sprintf("core-update:%g", delta)) },
						OnEngineFixedUpdate: func(delta float64) { calls = append(calls, fmt.Sprintf("core-fixed:%g", delta)) },
						OnEnginePause:       func(paused bool) { calls = append(calls, fmt.Sprintf("core-pause:%t", paused)) },
						OnEngineDestroy:     func() { calls = append(calls, "core-destroy") },
					}
					want = append(want, "core-start")
				}
				logical := 0.25
				if fixed > 0 {
					logical = fixed
				}
				want = append(want, fmt.Sprintf("advance:%g", logical), fmt.Sprintf("sprite-update:%g", logical))
				if withCore {
					want = append(want, fmt.Sprintf("core-update:%g", logical))
				}
				want = append(want, fmt.Sprintf("runtime:%g", logical), "sprite-fixed:0.5")
				if withCore {
					want = append(want, "core-fixed:0.5", "core-pause:true", "core-destroy")
				}
				want = append(want, "sprite-destroy")

				if runtime.GOOS == "js" {
					// Web linking requires the host's full JS API; exercise callbacks directly.
					coreCallbacks = core
				} else {
					engine.AudioMgr, engine.CameraMgr, engine.DebugMgr = nil, nil, nil
					engine.ExtMgr, engine.InputMgr, engine.NavigationMgr = nil, nil, nil
					engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr = nil, nil, nil
					engine.ResMgr, engine.SceneMgr, engine.SpriteMgr = nil, nil, nil
					engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr = nil, nil, nil
					session := PrepareLink(core)
					t.Cleanup(func() {
						session.Unlink()
						if activeLink != nil || isWebIntepreterMode || !reflect.ValueOf(coreCallbacks).IsZero() {
							t.Error("Unlink retained the active session or callbacks")
						}
					})
					if activeLink != session || session.backend == nil {
						t.Fatal("PrepareLink did not install its backend session")
					}
					for i, manager := range []any{
						engine.AudioMgr, engine.CameraMgr, engine.DebugMgr, engine.ExtMgr, engine.InputMgr,
						engine.NavigationMgr, engine.PenMgr, engine.PhysicsMgr, engine.PlatformMgr, engine.ResMgr,
						engine.SceneMgr, engine.SpriteMgr, engine.TilemapMgr, engine.TilemapparserMgr, engine.UiMgr,
					} {
						if manager == nil {
							t.Fatalf("PrepareLink did not bind manager %d", i)
						}
					}
				}
				callbacks := bindCallbacks()
				callbacks.OnEngineStart()
				callbacks.OnEngineUpdate(0.25)
				callbacks.OnEngineFixedUpdate(0.5)
				callbacks.OnEnginePause(true)
				callbacks.OnEngineDestroy()
				if !slices.Equal(calls, want) {
					t.Fatalf("lifecycle calls = %v, want %v", calls, want)
				}
			})
		}
	}
}
