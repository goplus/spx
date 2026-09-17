//go:build js && wasm

package webffi

import (
	"syscall/js"
	"testing"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestDispatcherSyncsFrameInput(t *testing.T) {
	previousCallbacks, previousSnapshot := callbacks, inputSnap
	previousFrame, previousBool, previousAxis := actionFrame, actionBool, actionAxis
	previousBindings := js.Global().Get("GdspxFuncs")
	t.Cleanup(func() {
		callbacks, inputSnap = previousCallbacks, previousSnapshot
		actionFrame, actionBool, actionAxis = previousFrame, previousBool, previousAxis
		js.Global().Set("GdspxFuncs", previousBindings)
	})

	bindings := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	defer bindings.Release()
	reads := 0
	reader := js.FuncOf(func(js.Value, []js.Value) any {
		reads++
		data := jsUint8Array.New(12)
		js.CopyBytesToJS(data, []byte{0, 0, 192, 63, 0, 0, 32, 192, 0, 0, 128, 63})
		return map[string]any{arrayTag: true, "type": GdArrayTypeFloat, "count": 3, "data": data}
	})
	defer reader.Release()
	bindings.Set("arrayOutputs", map[string]any{"gdspx_input_write_snapshot": reader})
	js.Global().Set("GdspxFuncs", bindings)

	assertSynced := func(frame uint64) {
		t.Helper()
		if !inputSnap.ok || inputSnap.mouse.X != 1.5 || inputSnap.mouse.Y != -2.5 || inputSnap.mouseBits != 1 {
			t.Fatalf("input snapshot = %+v", inputSnap)
		}
		if inputSnap.frame != frame || actionFrame != frame || len(actionBool) != 0 || len(actionAxis) != 0 {
			t.Fatalf("frame = (%d, %d), cache sizes = (%d, %d)", inputSnap.frame, actionFrame, len(actionBool), len(actionAxis))
		}
	}

	inputSnap = inputSnapshot{frame: 10}
	actionFrame = 10
	actionBool = map[string]bool{"pressed\x00left": true}
	actionAxis = map[string]float64{"left\x00right": -1}
	callbacks = engine.CallbackInfo{}
	callbacks.OnEngineUpdate = func(delta float64) {
		if delta != 0.25 {
			t.Fatalf("delta = %v, want 0.25", delta)
		}
		assertSynced(11)
	}
	gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineUpdate, js.ValueOf(0.25)})

	callbacks = engine.CallbackInfo{}
	actionBool["pressed\x00left"] = true
	actionAxis["left\x00right"] = -1
	gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineFixedUpdate, js.ValueOf(0.25)})
	assertSynced(12)
	if reads != 2 {
		t.Fatalf("snapshot reads = %d, want 2", reads)
	}
}

func TestDispatcherRecordsKeyState(t *testing.T) {
	previousCallbacks, previousDown := callbacks, keyDown
	t.Cleanup(func() { callbacks, keyDown = previousCallbacks, previousDown })

	const key int64 = 1<<42 | 65
	keyDown = map[int64]bool{}
	callbacks = engine.CallbackInfo{}
	callbacks.OnKeyPressed = func(got int64) {
		if got != key || !keyDown[key] {
			t.Fatalf("key = %d, pressed = %v", got, keyDown[key])
		}
	}
	gdspxDispatch(js.Undefined(), []js.Value{jsEventOnKeyPressed, JsFromGdInt(key)})

	callbacks = engine.CallbackInfo{}
	gdspxDispatch(js.Undefined(), []js.Value{jsEventOnKeyReleased, JsFromGdInt(key)})
	if keyDown[key] {
		t.Fatal("key release was not recorded without a handler")
	}
}
