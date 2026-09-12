//go:build js && wasm

package webffi

import (
	"syscall/js"
	"testing"
)

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
			previousFrame, previousBool, previousAxis := actionFrame, actionBool, actionAxis
			API.SpxInputIsActionPressedId = js.Undefined()
			API.SpxInputIsActionJustPressedId = js.Undefined()
			API.SpxInputIsActionJustReleasedId = js.Undefined()
			API.SpxInputGetAxisId = js.Undefined()
			actionIDs = map[string]int{"left": 1, "right": 2}
			actionFrame, actionBool, actionAxis = 1, map[string]bool{}, map[string]float64{}
			t.Cleanup(func() {
				API, actionIDs = previousAPI, previousIDs
				actionFrame, actionBool, actionAxis = previousFrame, previousBool, previousAxis
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
			if query(fallback) != 0 || query(fallback) != 0 || calls != 1 {
				t.Fatal("zero/false results must be cached")
			}
			clearActionCache(1)
			query(fallback)
			if calls != 1 {
				t.Fatal("same frame must retain cached results")
			}
			clearActionCache(2)
			query(fallback)
			if calls != 2 {
				t.Fatal("next frame must refresh cached results")
			}

			clearActionCache(3)
			if query(func() float64 { clearActionCache(4); return 1 }) != 1 {
				t.Fatal("a read spanning frames must still return its result")
			}
			if query(fallback) != 0 || calls != 3 {
				t.Fatal("a read spanning frames must not populate the new frame")
			}
			clearActionCache(5)
			func() {
				defer func() {
					if recover() != "read failed" {
						t.Fatal("fallback panic must propagate")
					}
				}()
				query(func() float64 { panic("read failed") })
			}()
			query(fallback)
			if calls != 4 {
				t.Fatal("failed reads must not populate the cache")
			}
		})
	}
}
