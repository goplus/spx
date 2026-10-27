//go:build js && wasm

package webffi

import (
	"slices"
	"syscall/js"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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
	previousBool, previousAxis, previousGeneration := actionBool, actionAxis, actionGeneration
	previousSnapshot := inputSnap
	global := js.Global()
	previousBindings, previousEpoch := global.Get("GdspxFuncs"), global.Get("GdspxGetInputActionEpoch")
	t.Cleanup(func() {
		API, actionIDs = previousAPI, previousIDs
		actionBool, actionAxis, actionGeneration = previousBool, previousAxis, previousGeneration
		inputSnap = previousSnapshot
		global.Set("GdspxFuncs", previousBindings)
		global.Set("GdspxGetInputActionEpoch", previousEpoch)
	})
	global.Set("GdspxFuncs", js.Undefined())
	global.Set("GdspxGetInputActionEpoch", js.Undefined())
	actionIDs = map[string]int{"left": 7, "right": 9}
	actionBool, actionAxis = map[string]bool{}, map[string]float64{}

	var args []int
	calls, result := 0, false
	query := js.FuncOf(func(_ js.Value, values []js.Value) any {
		calls++
		args = nil
		for _, value := range values {
			args = append(args, value.Int())
		}
		return result
	})
	defer query.Release()
	API.SpxInputIsActionPressedId = query.Value
	API.SpxInputIsActionJustPressedId = query.Value
	API.SpxInputIsActionJustReleasedId = query.Value
	for _, test := range []struct {
		name string
		read func(string, func() bool) bool
	}{
		{"pressed", CachedInputIsActionPressed},
		{"just_pressed", CachedInputIsActionJustPressed},
		{"just_released", CachedInputIsActionJustReleased},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, want := range []bool{false, true} {
				SyncWebInputSnapshot()
				calls, result = 0, want
				for range 2 {
					got := test.read("left", func() bool { t.Fatal("valid action ID used fallback"); return false })
					if got != want {
						t.Fatalf("action value = %v, want current frame value %v", got, want)
					}
				}
				if calls != 1 || !slices.Equal(args, []int{7, 0}) {
					t.Fatalf("action binding: %d calls with %v, want one call with [7 0]", calls, args)
				}
			}
		})
	}
	if _, ok := webActionBool("unknown", "left"); ok {
		t.Fatal("unknown action kind should use fallback")
	}

	axisResult := 0.0
	axis := js.FuncOf(func(_ js.Value, values []js.Value) any {
		calls++
		args = nil
		for _, value := range values {
			args = append(args, value.Int())
		}
		return axisResult
	})
	defer axis.Release()
	API.SpxInputGetAxisId = axis.Value
	for _, want := range []float64{0, -0.5} {
		SyncWebInputSnapshot()
		calls, axisResult = 0, want
		for range 2 {
			got := CachedInputGetAxis("left", "right", func() float64 { t.Fatal("valid axis IDs used fallback"); return 0 })
			if got != want {
				t.Fatalf("axis value = %v, want current frame value %v", got, want)
			}
		}
		if calls != 1 || !slices.Equal(args, []int{7, 0, 9, 0}) {
			t.Fatalf("axis binding: %d calls with %v, want one call with [7 0 9 0]", calls, args)
		}
	}
}

func TestActionQueriesFallBackWhenUnavailable(t *testing.T) {
	previousAPI, previousIDs := API, actionIDs
	previousBool, previousAxis, previousGeneration := actionBool, actionAxis, actionGeneration
	global := js.Global()
	previousRegister, previousEpoch := global.Get("GdspxGetInputActionID"), global.Get("GdspxGetInputActionEpoch")
	t.Cleanup(func() {
		API, actionIDs = previousAPI, previousIDs
		actionBool, actionAxis, actionGeneration = previousBool, previousAxis, previousGeneration
		global.Set("GdspxGetInputActionID", previousRegister)
		global.Set("GdspxGetInputActionEpoch", previousEpoch)
	})
	global.Set("GdspxGetInputActionEpoch", js.Undefined())
	actionBool, actionAxis = map[string]bool{}, map[string]float64{}

	negativeID := js.FuncOf(func(js.Value, []js.Value) any { return -1 })
	undefined := js.FuncOf(func(js.Value, []js.Value) any { return js.Undefined() })
	null := js.FuncOf(func(js.Value, []js.Value) any { return js.Null() })
	unexpected := js.FuncOf(func(js.Value, []js.Value) any { t.Error("binding called without valid action IDs"); return nil })
	defer negativeID.Release()
	defer undefined.Release()
	defer null.Release()
	defer unexpected.Release()
	for _, test := range []struct {
		name              string
		ids               map[string]int
		register, binding js.Value
		axisOnly          bool
	}{
		{"missing ID provider", map[string]int{}, js.Undefined(), unexpected.Value, false},
		{"negative ID", map[string]int{}, negativeID.Value, unexpected.Value, false},
		{"missing positive axis ID", map[string]int{"left": 7}, negativeID.Value, unexpected.Value, true},
		{"missing binding", map[string]int{"left": 7, "right": 9}, js.Undefined(), js.Undefined(), false},
		{"undefined result", map[string]int{"left": 7, "right": 9}, js.Undefined(), undefined.Value, false},
		{"null result", map[string]int{"left": 7, "right": 9}, js.Undefined(), null.Value, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			clearActionCache()
			actionIDs = test.ids
			global.Set("GdspxGetInputActionID", test.register)
			API.SpxInputIsActionPressedId = test.binding
			API.SpxInputIsActionJustPressedId = test.binding
			API.SpxInputIsActionJustReleasedId = test.binding
			API.SpxInputGetAxisId = test.binding
			if !test.axisOnly {
				for _, read := range []func(string, func() bool) bool{CachedInputIsActionPressed, CachedInputIsActionJustPressed, CachedInputIsActionJustReleased} {
					calls := 0
					for range 2 {
						if !read("left", func() bool { calls++; return true }) {
							t.Fatal("unavailable action did not return fallback value")
						}
					}
					if calls != 1 {
						t.Fatalf("action fallback called %d times, want once per frame", calls)
					}
				}
			}
			calls := 0
			for range 2 {
				if got := CachedInputGetAxis("left", "right", func() float64 { calls++; return 0.75 }); got != 0.75 {
					t.Fatalf("unavailable axis = %v, want fallback value 0.75", got)
				}
			}
			if calls != 1 {
				t.Fatalf("axis fallback called %d times, want once per frame", calls)
			}
		})
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
