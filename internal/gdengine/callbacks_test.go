package gdengine

import (
	"fmt"
	"reflect"
	"testing"

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
	previousDelta, _ := itime.FixedDeltaTime()
	t.Cleanup(func() {
		coreCallbacks, sprites = previousCallbacks, previousSprites
		itime.SetFixedDeltaTime(previousDelta)
		engine.SetRuntimeBridge(nil)
	})

	for _, fixed := range []float64{0, 0.1} {
		for _, withCore := range []bool{false, true} {
			t.Run(fmt.Sprintf("fixed=%g/core=%v", fixed, withCore), func(t *testing.T) {
				var calls, want []string
				sprite := &callbackTestSprite{calls: &calls}
				engine.SetRuntimeBridge(&callbackTestRuntime{
					sprites: map[engine.Object]engine.ISpriter{1: sprite}, calls: &calls,
				})
				itime.SetFixedDeltaTime(fixed)
				coreCallbacks = engine.CoreCallbackInfo{}
				if withCore {
					coreCallbacks = engine.CoreCallbackInfo{
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

				callbacks := bindCallbacks()
				callbacks.OnEngineStart()
				callbacks.OnEngineUpdate(0.25)
				callbacks.OnEngineFixedUpdate(0.5)
				callbacks.OnEnginePause(true)
				callbacks.OnEngineDestroy()
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("lifecycle calls = %v, want %v", calls, want)
				}
			})
		}
	}
}
