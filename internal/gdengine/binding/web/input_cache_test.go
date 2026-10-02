//go:build js && wasm

package webffi

import (
	"slices"
	"syscall/js"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestInputValuesDoNotSurviveSessionBoundary(t *testing.T) {
	previousDown, previousSnapshot, previousCallbacks := keyDown, inputSnap, callbacks
	previousGeneration := actionGeneration
	previousBool, previousAxis := actionBool, actionAxis
	previousAPI, previousIDs := API, actionIDs
	t.Cleanup(func() {
		keyDown, inputSnap, callbacks = previousDown, previousSnapshot, previousCallbacks
		actionGeneration = previousGeneration
		actionBool, actionAxis = previousBool, previousAxis
		API, actionIDs = previousAPI, previousIDs
	})
	API.SpxInputIsActionPressedId = js.Undefined()
	actionIDs = map[string]int{"left": 1}
	for _, event := range []string{"OnEngineStart", "OnEngineReset", "OnEngineDestroy"} {
		for _, withHandler := range []bool{false, true} {
			name := event + "/without_handler"
			if withHandler {
				name = event + "/with_handler"
			}
			t.Run(name, func(t *testing.T) {
				keyDown = map[int64]bool{1: true, 2: false}
				inputSnap = inputSnapshot{mouse: mathf.NewVec2(11, 22), mouseBits: 1, ok: true}
				actionBool, actionAxis = map[string]bool{"pressed\x00left": true}, map[string]float64{}
				assertInput := func(wantOld bool) {
					t.Helper()
					if got := CachedInputGetKey(1, func() bool { return false }); got != wantOld {
						t.Errorf("pressed key = %v, want %v", got, wantOld)
					}
					if got := CachedInputGetKey(2, func() bool { return true }); got == wantOld {
						t.Errorf("released key = %v, want %v", got, !wantOld)
					}
					wantMouse := mathf.NewVec2(-1, -2)
					if wantOld {
						wantMouse = mathf.NewVec2(11, 22)
					}
					if got := CachedInputGetGlobalMousePos(func() mathf.Vec2 { return mathf.NewVec2(-1, -2) }); got != wantMouse {
						t.Errorf("mouse = %v, want %v", got, wantMouse)
					}
					if got := CachedInputGetMouseState(1, func() bool { return false }); got != wantOld {
						t.Errorf("mouse button = %v, want %v", got, wantOld)
					}
					if got := CachedInputIsActionPressed("left", func() bool { return false }); got != wantOld {
						t.Errorf("action = %v, want %v", got, wantOld)
					}
				}
				callbacks = engine.CallbackInfo{}
				calls := 0
				if withHandler {
					handler := func() { calls++; assertInput(event != "OnEngineStart") }
					switch event {
					case "OnEngineStart":
						callbacks.OnEngineStart = handler
					case "OnEngineReset":
						callbacks.OnEngineReset = handler
					case "OnEngineDestroy":
						callbacks.OnEngineDestroy = handler
					}
				}
				gdspxDispatch(js.Undefined(), []js.Value{js.ValueOf(event)})
				assertInput(false)
				if withHandler && calls != 1 {
					t.Fatalf("handler calls = %d, want 1", calls)
				}
			})
		}
	}
}

func TestInputCacheKeyState(t *testing.T) {
	previousDown := keyDown
	keyDown = map[int64]bool{}
	RecordWebKeyState(1, true)
	RecordWebKeyState(2, false)
	t.Cleanup(func() { keyDown = previousDown })
	for _, test := range []struct {
		key, fallback, want int64
		calls               int
	}{
		{1, 0, 1, 0},
		{2, 1, 0, 0},
		{3, -2, 1, 1},
		{3, 0, 0, 1},
	} {
		calls := 0
		got := CachedInputGetKeyState(test.key, func() int64 {
			calls++
			return test.fallback
		})
		if got != test.want || calls != test.calls {
			t.Fatalf("key %d, fallback %d: got (%d, %d calls), want (%d, %d calls)", test.key, test.fallback, got, calls, test.want, test.calls)
		}
	}
}

func TestInputActionCache(t *testing.T) {
	for _, kind := range []string{"pressed", "just_pressed", "just_released", "axis"} {
		t.Run(kind, func(t *testing.T) {
			previousAPI, previousIDs := API, actionIDs
			previousBindings := js.Global().Get("GdspxFuncs")
			js.Global().Set("GdspxFuncs", js.Undefined())
			previousSnapshot := inputSnap
			previousGeneration := actionGeneration
			previousBool, previousAxis := actionBool, actionAxis
			previousReset, previousDestroy := callbacks.OnEngineReset, callbacks.OnEngineDestroy
			API.SpxInputIsActionPressedId = js.Undefined()
			API.SpxInputIsActionJustPressedId = js.Undefined()
			API.SpxInputIsActionJustReleasedId = js.Undefined()
			API.SpxInputGetAxisId = js.Undefined()
			actionIDs = map[string]int{"left": 1, "right": 2}
			actionGeneration = 0
			actionBool, actionAxis = map[string]bool{}, map[string]float64{}
			t.Cleanup(func() {
				API, actionIDs = previousAPI, previousIDs
				js.Global().Set("GdspxFuncs", previousBindings)
				inputSnap = previousSnapshot
				actionGeneration = previousGeneration
				actionBool, actionAxis = previousBool, previousAxis
				callbacks.OnEngineReset, callbacks.OnEngineDestroy = previousReset, previousDestroy
			})
			query := func(fallback func() float64) float64 {
				if kind == "axis" {
					return CachedInputGetAxis("left", "right", fallback)
				}
				read := func() bool { return fallback() != 0 }
				var value bool
				switch kind {
				case "pressed":
					value = CachedInputIsActionPressed("left", read)
				case "just_pressed":
					value = CachedInputIsActionJustPressed("left", read)
				case "just_released":
					value = CachedInputIsActionJustReleased("left", read)
				}
				if value {
					return 1
				}
				return 0
			}
			calls := 0
			fallback := func() float64 { calls++; return 0 }
			first := query(fallback)
			cached := query(fallback)
			if first != 0 || cached != 0 || calls != 1 {
				t.Fatal("zero/false results must be cached")
			}
			SyncWebInputSnapshot()
			query(fallback)
			if calls != 2 {
				t.Fatal("next frame must refresh cached results")
			}

			SyncWebInputSnapshot()
			if query(func() float64 { SyncWebInputSnapshot(); return 1 }) != 1 {
				t.Fatal("a read spanning frames must still return its result")
			}
			if query(fallback) != 0 || calls != 3 {
				t.Fatal("a read spanning frames must not populate the new frame")
			}

			callbacks.OnEngineReset = func() {
				if query(func() float64 { calls++; return 1 }) != 0 || calls != 3 {
					t.Fatal("teardown callback must see the current frame")
				}
			}
			gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineReset})
			callbacks.OnEngineReset = nil
			if query(fallback) != 0 || calls != 4 {
				t.Fatal("reset must refresh cached results within the same frame")
			}
			gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineDestroy})
			if query(fallback) != 0 || calls != 5 {
				t.Fatal("destroy must refresh cached results within the same frame")
			}
			gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineReset})
			if query(func() float64 {
				gdspxDispatch(js.Undefined(), []js.Value{jsEventOnEngineReset})
				return 1
			}) != 1 {
				t.Fatal("a read spanning reset must still return its result")
			}
			if query(fallback) != 0 || calls != 6 {
				t.Fatal("a read spanning reset must not populate the cache")
			}

			SyncWebInputSnapshot()
			func() {
				defer func() {
					if recover() != "read failed" {
						t.Fatal("fallback panic must propagate")
					}
				}()
				query(func() float64 { panic("read failed") })
			}()
			query(fallback)
			if calls != 7 {
				t.Fatal("failed reads must not populate the cache")
			}
		})
	}
}

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
		if !slices.Equal(args, []int{7, 0}) {
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
	if !slices.Equal(args, []int{7, 0, 9, 0}) {
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
