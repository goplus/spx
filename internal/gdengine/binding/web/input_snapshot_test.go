//go:build js && wasm

package webffi

import (
	"reflect"
	"syscall/js"
	"testing"
)

func TestActionQueriesUseGeneratedBindings(t *testing.T) {
	previousAPI, previousIDs := API, actionIDs
	actionIDs = map[string]int{"left": 7, "right": 9}
	t.Cleanup(func() { API, actionIDs = previousAPI, previousIDs })

	var args []int
	query := js.FuncOf(func(_ js.Value, values []js.Value) any {
		args = nil
		for _, value := range values {
			args = append(args, value.Int())
		}
		return true
	})
	defer query.Release()
	API.SpxInputIsActionPressedId = query.Value
	API.SpxInputIsActionJustPressedId = query.Value
	API.SpxInputIsActionJustReleasedId = query.Value
	for _, kind := range []string{"pressed", "just_pressed", "just_released"} {
		if value, ok := webActionBool(kind, "left"); !ok || !value {
			t.Fatalf("%s: got (%v, %v)", kind, value, ok)
		}
		if !reflect.DeepEqual(args, []int{7, 0}) {
			t.Fatalf("%s: flattened ID arguments = %v", kind, args)
		}
	}
	if _, ok := webActionBool("unknown", "left"); ok {
		t.Fatal("unknown action kind should use fallback")
	}

	axis := js.FuncOf(func(_ js.Value, values []js.Value) any {
		args = nil
		for _, value := range values {
			args = append(args, value.Int())
		}
		return -0.5
	})
	defer axis.Release()
	API.SpxInputGetAxisId = axis.Value
	if value, ok := webActionAxis("left", "right"); !ok || value != -0.5 {
		t.Fatalf("axis: got (%v, %v)", value, ok)
	}
	if !reflect.DeepEqual(args, []int{7, 0, 9, 0}) {
		t.Fatalf("flattened axis arguments = %v", args)
	}
	API.SpxInputGetAxisId = js.Undefined()
	API.SpxInputIsActionPressedId = js.Undefined()
	if _, ok := webActionAxis("left", "right"); ok {
		t.Fatal("missing axis binding should use fallback")
	}
	if _, ok := webActionBool("pressed", "left"); ok {
		t.Fatal("missing action binding should use fallback")
	}
}

func TestActionIDsRefreshAfterReset(t *testing.T) {
	global := js.Global()
	previousEpoch := global.Get("GdspxGetInputActionEpoch")
	previousRegister := global.Get("GdspxGetInputActionID")
	previousIDs, previousID := actionIDs, actionEpoch
	t.Cleanup(func() {
		global.Set("GdspxGetInputActionEpoch", previousEpoch)
		global.Set("GdspxGetInputActionID", previousRegister)
		actionIDs, actionEpoch = previousIDs, previousID
	})

	epoch, id, registrations := 1, 7, 0
	epochFn := js.FuncOf(func(js.Value, []js.Value) any { return epoch })
	registerFn := js.FuncOf(func(js.Value, []js.Value) any {
		registrations++
		return id
	})
	defer epochFn.Release()
	defer registerFn.Release()
	global.Set("GdspxGetInputActionEpoch", epochFn)
	global.Set("GdspxGetInputActionID", registerFn)
	actionIDs = map[string]int{}
	actionEpoch = 0

	for range 2 {
		if got, ok := webActionID("left"); !ok || got != 7 {
			t.Fatalf("before reset: got (%d, %v)", got, ok)
		}
	}
	if registrations != 1 {
		t.Fatalf("registered %d times before reset", registrations)
	}

	epoch, id = 2, 9
	if got, ok := webActionID("left"); !ok || got != 9 {
		t.Fatalf("after reset: got (%d, %v)", got, ok)
	}
	if registrations != 2 {
		t.Fatalf("registered %d times after reset", registrations)
	}
}

func TestInputSnapshotUsesGeneratedOutputReader(t *testing.T) {
	previousBindings, previousSnapshot := js.Global().Get("GdspxFuncs"), inputSnap
	t.Cleanup(func() {
		js.Global().Set("GdspxFuncs", previousBindings)
		inputSnap = previousSnapshot
	})
	bindings := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	defer bindings.Release()
	reader := js.FuncOf(func(js.Value, []js.Value) any {
		data := jsUint8Array.New(12)
		js.CopyBytesToJS(data, []byte{0, 0, 192, 63, 0, 0, 32, 192, 0, 0, 224, 64})
		return map[string]any{arrayTag: true, "type": GdArrayTypeFloat, "count": 3, "data": data}
	})
	defer reader.Release()
	bindings.Set("arrayOutputs", map[string]any{"gdspx_input_write_snapshot": reader})
	js.Global().Set("GdspxFuncs", bindings)
	SyncWebInputSnapshot()
	if !inputSnap.ok || inputSnap.mouse.X != 1.5 || inputSnap.mouse.Y != -2.5 || inputSnap.mouseBits != 7 {
		t.Fatalf("unexpected snapshot: %+v", inputSnap)
	}
	for _, outputs := range []any{nil, js.Undefined(), map[string]any{}} {
		bindings.Set("arrayOutputs", outputs)
		SyncWebInputSnapshot()
		if inputSnap.ok {
			t.Fatal("unavailable reader should invalidate snapshot")
		}
	}
}
