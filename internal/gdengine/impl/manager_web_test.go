//go:build js && wasm

package impl

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
	"syscall/js"
	"testing"

	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	webffi "github.com/goplus/spx/v4/internal/gdengine/binding/web"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

var webManagerCacheTestID atomic.Uint64

func TestBatchRetrievePositionsUpdatesCallerBuffer(t *testing.T) {
	previousSprite := gdx.SpriteMgr
	t.Cleanup(func() { gdx.SpriteMgr = previousSprite })
	t.Cleanup(wrap.BindManagers([]gdx.IManager{new(spriteMgr)}))
	inside, dispatches := false, 0
	buffer := []float32{42, 42, 42}
	success := true
	useWebManagerDispatch(t, func(call func()) {
		dispatches++
		inside = true
		defer func() { inside = false }()
		call()
		if success && (buffer[0] != 3 || buffer[1] != -4) {
			t.Error("position output was not copied back within dispatch")
		}
	})
	useWebArrayStorage(t, &inside)
	previousRead := webffi.API.SpxSpriteBatchRetrievePositions
	read := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if !inside {
			t.Error("position query ran outside dispatch")
		}
		if len(args) != 2 || args[0].Get("count").Int() != 1 || args[0].Get("type").Int() != 6 || args[1].Get("count").Int() != 2 {
			t.Error("position query has unexpected arguments")
			return nil
		}
		bits := make([]byte, 8)
		js.CopyBytesToGo(bits, args[0].Get("data"))
		if bits[0] != 1 || bits[2] != 0xc0 || bits[3] != 0x7f {
			t.Error("position query changed the ID's NaN bit pattern")
		}
		initial := make([]byte, 8)
		js.CopyBytesToGo(initial, args[1].Get("data"))
		if !bytes.Equal(initial, bytes.Repeat([]byte{0xa5}, 8)) {
			t.Error("output storage was copied or cleared before the call")
		}
		js.CopyBytesToJS(args[1].Get("data"), []byte{0, 0, 64, 64, 0, 0, 128, 192})
		return success
	})
	webffi.API.SpxSpriteBatchRetrievePositions = read.Value
	t.Cleanup(func() {
		webffi.API.SpxSpriteBatchRetrievePositions = previousRead
		read.Release()
	})
	ids := []int64{0x112233447fc00001}
	if !gdx.SpriteMgr.BatchRetrievePositions(ids, buffer[:2]) {
		t.Fatal("successful query returned false")
	}
	if ids[0] != 0x112233447fc00001 || buffer[0] != 3 || buffer[1] != -4 || buffer[2] != 42 {
		t.Fatalf("position output = %v", buffer)
	}
	success = false
	buffer[0], buffer[1] = 11, 12
	if gdx.SpriteMgr.BatchRetrievePositions(ids, buffer[:2]) {
		t.Fatal("failed query returned true")
	}
	if buffer[0] != 11 || buffer[1] != 12 || buffer[2] != 42 {
		t.Fatalf("failed query copied output back: %v", buffer)
	}
	if dispatches != 2 {
		t.Fatalf("position query dispatches = %d, want 2", dispatches)
	}
}

func TestWriteSnapshotUpdatesCallerArray(t *testing.T) {
	previousInput := gdx.InputMgr
	t.Cleanup(func() { gdx.InputMgr = previousInput })
	t.Cleanup(wrap.BindManagers([]gdx.IManager{new(inputMgr)}))
	inside := false
	dispatches := 0
	out := [3]float32{42, 43, 44}
	useWebManagerDispatch(t, func(call func()) {
		dispatches++
		inside = true
		defer func() { inside = false }()
		call()
		if want := [3]float32{1.5, -2.5, 7}; out != want {
			t.Errorf("snapshot was not copied back within dispatch: %v", out)
		}
	})
	useWebArrayStorage(t, &inside)
	previousWrite := webffi.API.SpxInputWriteSnapshot
	calls := 0
	write := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if !inside {
			t.Error("snapshot call ran outside dispatch")
		}
		calls++
		if len(args) != 1 || args[0].Get("count").Int() != 3 {
			t.Errorf("unexpected snapshot arguments: %v", args)
			return nil
		}
		initial := make([]byte, 12)
		js.CopyBytesToGo(initial, args[0].Get("data"))
		if !bytes.Equal(initial, bytes.Repeat([]byte{0xa5}, 12)) {
			t.Error("snapshot output was copied or cleared before the call")
		}
		js.CopyBytesToJS(args[0].Get("data"), []byte{0, 0, 192, 63, 0, 0, 32, 192, 0, 0, 224, 64})
		return nil
	})
	webffi.API.SpxInputWriteSnapshot = write.Value
	t.Cleanup(func() {
		webffi.API.SpxInputWriteSnapshot = previousWrite
		write.Release()
	})

	mgr := gdx.InputMgr
	mgr.WriteSnapshot(&out)
	if want := [3]float32{1.5, -2.5, 7}; out != want {
		t.Fatalf("snapshot = %v, want %v", out, want)
	}
	t.Run("nil output", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected nil output to panic")
			}
			if calls != 1 {
				t.Error("nil output reached the Web binding")
			}
			if dispatches != 2 {
				t.Errorf("nil output was validated before dispatch: dispatches=%d, want 2", dispatches)
			}
		}()
		mgr.WriteSnapshot(nil)
	})
}

func TestWebManagerDirectAndWrappedCallsDispatchOnce(t *testing.T) {
	previousAPI, previousAudio := webffi.API, gdx.AudioMgr
	t.Cleanup(func() { webffi.API, gdx.AudioMgr = previousAPI, previousAudio })
	t.Cleanup(wrap.BindManagers([]gdx.IManager{new(audioMgr)}))
	scheduler := coroutine.New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	dispatches := 0
	inside := false
	useWebManagerDispatch(t, func(call func()) {
		dispatches++
		inside = true
		defer func() { inside = false }()
		scheduler.WaitMainThread(call)
	})
	sets, gets := 0, 0
	set := js.FuncOf(func(_ js.Value, args []js.Value) any {
		sets++
		if !inside || len(args) != 3 || args[0].Int() != 7 || args[1].Int() != 0 || args[2].Float() != 0.25 {
			t.Errorf("setter escaped dispatch or changed arguments: inside=%v, args=%v", inside, args)
		}
		if got := catchWebManagerPanic(func() { scheduler.RunAfterStop(nil) }); got != coroutine.ErrReentrantWait {
			t.Errorf("manager callback drain panic = %v, want %v", got, coroutine.ErrReentrantWait)
		}
		return nil
	})
	get := js.FuncOf(func(_ js.Value, args []js.Value) any {
		gets++
		if !inside || len(args) != 2 || args[0].Int() != 7 || args[1].Int() != 0 {
			t.Errorf("getter escaped dispatch or changed arguments: inside=%v, args=%v", inside, args)
		}
		return 0.625
	})
	t.Cleanup(func() { set.Release(); get.Release() })
	webffi.API.SpxAudioSetVolume, webffi.API.SpxAudioGetVolume = set.Value, get.Value
	for _, test := range []struct {
		name string
		mgr  gdx.IAudioMgr
	}{
		{"public", gdx.AudioMgr},
		{"facade", new(wrap.AudioMgrImpl)},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := dispatches
			test.mgr.SetVolume(7, 0.25)
			if got := test.mgr.GetVolume(7); got != 0.625 {
				t.Errorf("volume = %v, want 0.625", got)
			}
			if dispatches-before != 2 {
				t.Errorf("setter and getter dispatched %d times, want 2", dispatches-before)
			}
		})
	}
	if sets != 2 || gets != 2 {
		t.Fatalf("backend calls = (%d, %d), want (2, 2)", sets, gets)
	}
}

func TestWebManagerCacheHitStillChecksCancellation(t *testing.T) {
	previousAPI, previousInput := webffi.API, gdx.InputMgr
	t.Cleanup(func() { webffi.API, gdx.InputMgr = previousAPI, previousInput })
	t.Cleanup(wrap.BindManagers([]gdx.IManager{new(inputMgr)}))
	scheduler := coroutine.New(nil)
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	dispatches := 0
	useWebManagerDispatch(t, func(call func()) {
		dispatches++
		scheduler.WaitMainThread(call)
	})
	fallbacks := 0
	read := js.FuncOf(func(js.Value, []js.Value) any {
		fallbacks++
		return true
	})
	t.Cleanup(read.Release)
	webffi.API.SpxInputIsActionPressed = read.Value
	action := fmt.Sprintf("%s/%d", t.Name(), webManagerCacheTestID.Add(1))
	if !gdx.InputMgr.IsActionPressed(action) {
		t.Fatal("cache miss lost the backend result")
	}
	if !gdx.InputMgr.IsActionPressed(action) || fallbacks != 1 || dispatches != 2 {
		t.Fatalf("cache hit: fallbacks=%d, dispatches=%d, want 1 and 2", fallbacks, dispatches)
	}
	for _, test := range []struct {
		name string
		read func(string) bool
	}{
		{"public", gdx.InputMgr.IsActionPressed},
		{"facade", new(wrap.InputMgrImpl).IsActionPressed},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := dispatches
			var canceled any
			scheduler.RunSync(test.name, func(me coroutine.Thread) {
				me.Cancel()
				canceled = catchWebManagerPanic(func() { test.read(action) })
			})
			if canceled != coroutine.ErrAbortThread {
				t.Fatalf("cached call cancellation = %v, want %v", canceled, coroutine.ErrAbortThread)
			}
			if dispatches-before != 1 || fallbacks != 1 {
				t.Fatalf("canceled cache hit: dispatches=%d, fallbacks=%d, want 1 and 1", dispatches-before, fallbacks)
			}
		})
	}
}

func TestWebManagerPropagatesBridgePanicAndRestoresScope(t *testing.T) {
	previous, previousAudio := webffi.API.SpxAudioSetVolume, gdx.AudioMgr
	t.Cleanup(func() { webffi.API.SpxAudioSetVolume, gdx.AudioMgr = previous, previousAudio })
	t.Cleanup(wrap.BindManagers([]gdx.IManager{new(audioMgr)}))
	scheduler := coroutine.New(nil)
	useWebManagerDispatch(t, scheduler.WaitMainThread)
	webffi.API.SpxAudioSetVolume = js.Global().Get("Function").New("throw new Error('web manager failure')")
	value := catchWebManagerPanic(func() { gdx.AudioMgr.SetVolume(1, 0.5) })
	if err, ok := value.(js.Error); !ok || !strings.Contains(err.Error(), "web manager failure") {
		t.Fatalf("bridge panic = %#v, want original JavaScript error", value)
	}
	// A failed callback must remove its no-drain scope before returning.
	scheduler.RunAfterStop(nil)
}

func TestWebManagerThreadProbeDoesNotDispatch(t *testing.T) {
	previous, previousPlatform := webffi.API.SpxPlatformIsMainThread, gdx.PlatformMgr
	t.Cleanup(func() { webffi.API.SpxPlatformIsMainThread, gdx.PlatformMgr = previous, previousPlatform })
	t.Cleanup(wrap.BindManagers([]gdx.IManager{new(platformMgr)}))
	useWebManagerDispatch(t, func(func()) { t.Fatal("thread probe attempted recursive dispatch") })
	probe := js.FuncOf(func(js.Value, []js.Value) any { return true })
	t.Cleanup(probe.Release)
	webffi.API.SpxPlatformIsMainThread = probe.Value
	if !gdx.PlatformMgr.IsMainThread() || !new(wrap.PlatformMgrImpl).IsMainThread() {
		t.Fatal("thread probe lost the backend result")
	}
}

func useWebManagerDispatch(t *testing.T, dispatch func(func())) {
	t.Helper()
	wrap.Init(dispatch)
	t.Cleanup(func() { wrap.Init(nil) })
}

func useWebArrayStorage(t *testing.T, inside *bool) {
	t.Helper()
	previous := js.Global().Get("GdspxBorrowNativeArray")
	borrow := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if !*inside {
			t.Error("array conversion ran outside dispatch")
		}
		data := js.Global().Get("Uint8Array").New(args[2].Int())
		data.Call("fill", 0xa5)
		return map[string]any{
			"__gdspx_array": true,
			"type":          args[0].Int(), "count": args[1].Int(),
			"data": data,
		}
	})
	js.Global().Set("GdspxBorrowNativeArray", borrow)
	t.Cleanup(func() {
		js.Global().Set("GdspxBorrowNativeArray", previous)
		borrow.Release()
	})
}

func catchWebManagerPanic(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}
