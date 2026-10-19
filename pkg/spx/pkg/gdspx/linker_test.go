package gdspx

import (
	"testing"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type testLinkerBridge struct {
	web            bool
	links, unlinks int
	callbacks      engine.CoreCallbackInfo
}

func (b *testLinkerBridge) IsWebIntepreterMode() bool { return b.web }
func (b *testLinkerBridge) Link(callbacks engine.CoreCallbackInfo) {
	b.links++
	b.callbacks = callbacks
}
func (b *testLinkerBridge) Unlink() { b.unlinks++ }

func TestLinkerBridgeLifecycle(t *testing.T) {
	previous := linkerBridge
	t.Cleanup(func() { SetLinkerBridge(previous) })
	for _, web := range []bool{false, true} {
		bridge := &testLinkerBridge{web: web}
		SetLinkerBridge(bridge)
		if got := IsWebIntepreterMode(); got != web {
			t.Fatalf("IsWebIntepreterMode() = %v, want %v", got, web)
		}
		var starts int
		var delta float64
		var key int64
		LinkEngine(engine.CoreCallbackInfo{
			OnEngineStart:  func() { starts++ },
			OnEngineUpdate: func(dt float64) { delta = dt },
			OnKeyPressed:   func(code int64) { key = code },
		})
		if bridge.links != 1 || starts != 0 {
			t.Fatalf("link calls = %d, start callbacks = %d; linking must not run callbacks", bridge.links, starts)
		}
		if bridge.callbacks.OnEngineStart == nil || bridge.callbacks.OnEngineUpdate == nil || bridge.callbacks.OnKeyPressed == nil {
			t.Fatal("linking lost runtime callbacks")
		}
		bridge.callbacks.OnEngineStart()
		bridge.callbacks.OnEngineUpdate(0.125)
		bridge.callbacks.OnKeyPressed(1<<40 + 7)
		if starts != 1 || delta != 0.125 || key != 1<<40+7 {
			t.Fatalf("callback state = (%d, %v, %d)", starts, delta, key)
		}
		UnlinkEngine()
		if bridge.unlinks != 1 {
			t.Fatalf("unlink calls = %d, want 1", bridge.unlinks)
		}
	}
}

func TestLinkerRequiresBridge(t *testing.T) {
	previous := linkerBridge
	SetLinkerBridge(nil)
	t.Cleanup(func() { SetLinkerBridge(previous) })
	for name, call := range map[string]func(){
		"mode":   func() { IsWebIntepreterMode() },
		"link":   func() { LinkEngine(engine.CoreCallbackInfo{}) },
		"unlink": UnlinkEngine,
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != "gdspx linker bridge is not initialized" {
					t.Errorf("panic = %v, want missing-bridge diagnostic", got)
				}
			}()
			call()
		})
	}
}
