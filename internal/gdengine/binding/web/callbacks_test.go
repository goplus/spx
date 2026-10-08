//go:build js && wasm

package webffi

import (
	"encoding/binary"
	"slices"
	"syscall/js"
	"testing"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestContactEventsDecodePackedBytes(t *testing.T) {
	previousCallbacks := callbacks
	t.Cleanup(func() { callbacks = previousCallbacks })
	type contact struct {
		kind        int
		self, other int64
	}
	want := []contact{
		{contactCollisionEnter, -1 << 63, 1<<63 - 1},
		{contactCollisionStay, 1<<54 + 3, -1},
		{contactCollisionExit, 0, 1},
		{contactTriggerEnter, -1, 1<<54 + 3},
		{contactTriggerStay, 1<<63 - 1, -1 << 63},
		{contactTriggerExit, 1, 0},
	}
	var got []contact
	record := func(kind int) func(int64, int64) {
		return func(self, other int64) { got = append(got, contact{kind, self, other}) }
	}
	callbacks = engine.CallbackInfo{
		OnCollisionEnter: record(contactCollisionEnter), OnCollisionStay: record(contactCollisionStay),
		OnCollisionExit: record(contactCollisionExit), OnTriggerEnter: record(contactTriggerEnter),
		OnTriggerStay: record(contactTriggerStay), OnTriggerExit: record(contactTriggerExit),
	}
	data := make([]byte, (len(want)+1)*contactEventBytes+7)
	for i, event := range want {
		entry := data[i*contactEventBytes:]
		binary.LittleEndian.PutUint32(entry, uint32(event.kind))
		binary.LittleEndian.PutUint64(entry[4:], uint64(event.self))
		binary.LittleEndian.PutUint64(entry[12:], uint64(event.other))
	}
	packed := jsUint8Array.New(len(data))
	js.CopyBytesToJS(packed, data)
	gdspxContactEvents(js.Undefined(), []js.Value{packed})
	if !slices.Equal(got, want) {
		t.Fatalf("decoded contacts = %v, want %v", got, want)
	}
	// Empty, non-byte, incomplete and unknown records produce no extra callbacks.
	for _, args := range [][]js.Value{
		nil, {js.Undefined()}, {js.Null()}, {js.ValueOf([]any{1, 0, 0, 0, 0})},
		{jsUint8Array.New(0)}, {jsUint8Array.New(contactEventBytes - 1)},
	} {
		gdspxContactEvents(js.Undefined(), args)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("invalid batches changed contacts: %v", got)
	}
}

func TestContactBatchStopsAtSessionBoundary(t *testing.T) {
	previousCallbacks := callbacks
	t.Cleanup(func() { BindCallback(previousCallbacks) })
	registerWebGlobals()
	dispatch := js.Global().Get("gdspx_dispatch")
	batch := js.Global().Get("gdspx_on_contact_events")
	data := make([]byte, 2*contactEventBytes)
	for i := range 2 {
		entry := data[i*contactEventBytes:]
		binary.LittleEndian.PutUint32(entry, contactTriggerEnter)
		binary.LittleEndian.PutUint64(entry[4:], uint64(101+i))
		binary.LittleEndian.PutUint64(entry[12:], 201)
	}
	packed := jsUint8Array.New(len(data))
	js.CopyBytesToJS(packed, data)

	for _, event := range []string{"OnEngineReset", "OnEngineDestroy", "OnEngineStart"} {
		for _, hasHandler := range []bool{false, true} {
			name := event + "/without_handler"
			if hasHandler {
				name = event + "/with_handler"
			}
			t.Run(name, func(t *testing.T) {
				var got []int64
				lifecycleCalls := 0
				info := engine.CallbackInfo{
					OnTriggerEnter: func(self, other int64) {
						got = append(got, self)
						if len(got) == 1 {
							dispatch.Invoke(event)
						}
					},
				}
				if hasHandler {
					handler := func() { lifecycleCalls++ }
					switch event {
					case "OnEngineReset":
						info.OnEngineReset = handler
					case "OnEngineDestroy":
						info.OnEngineDestroy = handler
					case "OnEngineStart":
						info.OnEngineStart = handler
					}
				}
				BindCallback(info)
				batch.Invoke(packed)
				if !slices.Equal(got, []int64{101}) {
					t.Fatalf("contacts crossing %s = %v, want [101]", event, got)
				}
				if hasHandler && lifecycleCalls != 1 {
					t.Fatalf("lifecycle calls = %d, want 1", lifecycleCalls)
				}
				batch.Invoke(packed)
				if !slices.Equal(got, []int64{101, 101, 102}) {
					t.Fatalf("contacts from a new batch = %v, want [101 101 102]", got)
				}
			})
		}
	}

	t.Run("BindCallback", func(t *testing.T) {
		var oldContacts, newContacts []int64
		BindCallback(engine.CallbackInfo{
			OnTriggerEnter: func(self, other int64) {
				oldContacts = append(oldContacts, self)
				BindCallback(engine.CallbackInfo{
					OnTriggerEnter: func(self, other int64) { newContacts = append(newContacts, self) },
				})
			},
		})
		batch.Invoke(packed)
		if !slices.Equal(oldContacts, []int64{101}) || len(newContacts) != 0 {
			t.Fatalf("contacts crossing rebind: old = %v, new = %v", oldContacts, newContacts)
		}
		batch.Invoke(packed)
		if !slices.Equal(newContacts, []int64{101, 102}) {
			t.Fatalf("contacts after rebind = %v, want [101 102]", newContacts)
		}
	})
}

func TestContactBatchReentryPreservesPendingEvents(t *testing.T) {
	previousCallbacks := callbacks
	t.Cleanup(func() { BindCallback(previousCallbacks) })
	registerWebGlobals()
	batch := js.Global().Get("gdspx_on_contact_events")
	pack := func(ids ...int64) js.Value {
		data := make([]byte, len(ids)*contactEventBytes)
		for i, id := range ids {
			entry := data[i*contactEventBytes:]
			binary.LittleEndian.PutUint32(entry, contactTriggerEnter)
			binary.LittleEndian.PutUint64(entry[4:], uint64(id))
			binary.LittleEndian.PutUint64(entry[12:], 99)
		}
		packed := jsUint8Array.New(len(data))
		js.CopyBytesToJS(packed, data)
		return packed
	}
	for _, tc := range []struct {
		name string
		ids  []int64
	}{
		{"smaller", []int64{201, 202}},
		{"same size", []int64{201, 202, 203}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outer, inner := pack(101, 102, 103), pack(tc.ids...)
			var got []int64
			BindCallback(engine.CallbackInfo{OnTriggerEnter: func(self, other int64) {
				got = append(got, self)
				if self == 101 {
					batch.Invoke(inner)
				}
			}})
			batch.Invoke(outer)
			want := append([]int64{101}, tc.ids...)
			want = append(want, 102, 103)
			if !slices.Equal(got, want) {
				t.Fatalf("nested contact dispatch = %v, want %v", got, want)
			}
		})
	}
}

func TestDispatcherSyncsFrameInput(t *testing.T) {
	previousCallbacks, previousSnapshot := callbacks, inputSnap
	previousGeneration, previousBool, previousAxis := actionGeneration, actionBool, actionAxis
	previousBindings := js.Global().Get("GdspxFuncs")
	t.Cleanup(func() {
		callbacks, inputSnap = previousCallbacks, previousSnapshot
		actionGeneration, actionBool, actionAxis = previousGeneration, previousBool, previousAxis
		js.Global().Set("GdspxFuncs", previousBindings)
	})

	bindings := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	defer bindings.Release()
	reads := 0
	reader := js.FuncOf(func(js.Value, []js.Value) any {
		reads++
		data := jsUint8Array.New(12)
		js.CopyBytesToJS(data, []byte{0, 0, 192, 63, 0, 0, 32, 192, 0, 0, 224, 64})
		return map[string]any{arrayTag: true, "type": GdArrayTypeFloat, "count": 3, "data": data}
	})
	defer reader.Release()
	bindings.Set("arrayOutputs", map[string]any{"gdspx_input_write_snapshot": reader})
	js.Global().Set("GdspxFuncs", bindings)

	assertSynced := func() {
		t.Helper()
		if !inputSnap.ok || inputSnap.mouse.X != 1.5 || inputSnap.mouse.Y != -2.5 || inputSnap.mouseBits != 7 {
			t.Fatalf("input snapshot = %+v", inputSnap)
		}
		if len(actionBool) != 0 || len(actionAxis) != 0 {
			t.Fatalf("cache sizes = (%d, %d)", len(actionBool), len(actionAxis))
		}
	}

	inputSnap = inputSnapshot{}
	actionBool = map[string]bool{"pressed\x00left": true}
	actionAxis = map[string]float64{"left\x00right": -1}
	callbacks = engine.CallbackInfo{}
	callbacks.OnEngineUpdate = func(delta float64) {
		if delta != 0.25 {
			t.Fatalf("delta = %v, want 0.25", delta)
		}
		assertSynced()
	}
	gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineUpdate, js.ValueOf(0.25)})

	callbacks = engine.CallbackInfo{}
	actionBool["pressed\x00left"] = true
	actionAxis["left\x00right"] = -1
	gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineFixedUpdate, js.ValueOf(0.25)})
	assertSynced()
	if reads != 2 {
		t.Fatalf("snapshot reads = %d, want 2", reads)
	}

	invalidReader := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	defer invalidReader.Release()
	shortReader := js.FuncOf(func(js.Value, []js.Value) any {
		return map[string]any{arrayTag: true, "type": GdArrayTypeFloat, "count": 2, "data": jsUint8Array.New(8)}
	})
	defer shortReader.Release()
	for _, test := range []struct {
		name    string
		binding any
		outputs any
	}{
		{"missing bindings", js.Undefined(), nil},
		{"nonfunction bindings", map[string]any{}, nil},
		{"missing outputs", bindings, js.Undefined()},
		{"null outputs", bindings, nil},
		{"missing reader", bindings, map[string]any{}},
		{"invalid snapshot", bindings, map[string]any{"gdspx_input_write_snapshot": invalidReader}},
		{"short snapshot", bindings, map[string]any{"gdspx_input_write_snapshot": shortReader}},
	} {
		t.Run(test.name, func(t *testing.T) {
			js.Global().Set("GdspxFuncs", test.binding)
			bindings.Set("arrayOutputs", test.outputs)
			for _, event := range []js.Value{jsEventOnEngineUpdate, jsEventOnEngineFixedUpdate} {
				inputSnap.ok = true
				actionBool["pressed\x00left"] = true
				actionAxis["left\x00right"] = -1
				gdspxDispatch(js.Undefined(), []js.Value{event, js.ValueOf(0.25)})
				if inputSnap.ok || len(actionBool) != 0 || len(actionAxis) != 0 {
					t.Fatalf("unavailable snapshot retained input: valid=%v, cache sizes=(%d, %d)", inputSnap.ok, len(actionBool), len(actionAxis))
				}
			}
		})
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
