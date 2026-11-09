/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package enginewrap

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/input"
	spxlog "github.com/goplus/spx/v4/internal/log"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type dispatchCameraMgr struct {
	gdx.IManager
	gdx.ICameraMgr
	position mathf.Vec2
	reads    int
}

type dispatchInputMgr struct {
	gdx.IManager
	gdx.IInputMgr
	pressed map[int64]bool
	queries []int64
}

type dispatchSpriteMgr struct {
	gdx.IManager
	gdx.ISpriteMgr
	retrieve func([]int64, []float32) bool
}

type bindingAudioMgr struct {
	gdx.IManager
	gdx.IAudioMgr
	setVolume func(gdx.Object, float64)
	getVolume func(gdx.Object) float64
}

type embeddedAudioMgr struct {
	AudioMgrImpl
	getVolume func(gdx.Object) float64
}

type bindingPlatformMgr struct {
	gdx.IManager
	gdx.IPlatformMgr
	probes int
	worker bool
}

type concurrentPlatformMgr struct {
	gdx.IManager
	gdx.IPlatformMgr
}

func TestCallInMainThreadRequiresInitOffMainThread(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("Web has no native main-thread boundary")
	}
	originalCallback, previousPlatform := mainCallback, gdx.PlatformMgr
	t.Cleanup(func() {
		mainCallback, gdx.PlatformMgr = originalCallback, previousPlatform
	})

	Init(nil)
	for _, test := range []struct {
		name     string
		platform gdx.IPlatformMgr
	}{
		{"uninitialized", nil},
		{"worker", &bindingPlatformMgr{worker: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			gdx.PlatformMgr = test.platform
			called := false
			got := catchManagerPanic(func() { CallInMainThread(func() { called = true }) })
			if got != "enginewrap: Init must be called before using manager methods off the main thread" || called {
				t.Fatalf("panic = %v, called = %v; want Init requirement without execution", got, called)
			}
		})
	}
}

func TestCallInMainThreadValue(t *testing.T) {
	originalCallback, previousPlatform := mainCallback, gdx.PlatformMgr
	t.Cleanup(func() {
		mainCallback, gdx.PlatformMgr = originalCallback, previousPlatform
	})
	platform := &bindingPlatformMgr{}
	gdx.PlatformMgr = platform

	calls := 0
	Init(func(call func()) {
		calls++
		call()
	})

	value := CallInMainThreadValue(func() int { return 42 })
	if value != 42 {
		t.Fatalf("value = %d, want 42", value)
	}
	if calls != 1 {
		t.Fatalf("main-thread callback calls = %d, want 1", calls)
	}
	if platform.probes != 0 {
		t.Fatal("installed dispatcher was bypassed by the platform fast path")
	}
}

func TestRunnableOnUnfocusedReportsUnsupportedDisable(t *testing.T) {
	var output bytes.Buffer
	spxlog.SetStdoutOutput(&output)
	t.Cleanup(func() { spxlog.SetStdoutOutput(os.Stdout) })
	platform := &PlatformMgrImpl{}
	platform.SetRunnableOnUnfocused(true)
	if output.Len() != 0 {
		t.Fatalf("supported setting logged a warning: %q", output.String())
	}
	platform.SetRunnableOnUnfocused(false)
	warning := output.String()
	if !strings.Contains(warning, "[WARN]") || !strings.Contains(warning, "SetRunnableOnUnfocused(false) is not implemented yet") {
		t.Fatalf("unsupported setting warning = %q", warning)
	}
	platform.SetRunnableOnUnfocused(true)
	if output.String() != warning {
		t.Fatalf("restoring the supported setting logged another warning: %q", output.String())
	}
}

func TestBoundManagersRunOnMainThreadWithoutInit(t *testing.T) {
	previousCallback, previousAudio, previousPlatform := mainCallback, gdx.AudioMgr, gdx.PlatformMgr
	t.Cleanup(func() {
		mainCallback, gdx.AudioMgr, gdx.PlatformMgr = previousCallback, previousAudio, previousPlatform
	})
	Init(nil)
	const object gdx.Object = 1<<54 + 3
	calls := 0
	backend := &bindingAudioMgr{getVolume: func(obj gdx.Object) float64 {
		calls++
		if obj != object {
			t.Errorf("object = %d, want %d", obj, object)
		}
		return 0.625
	}}
	platform := &bindingPlatformMgr{}
	t.Cleanup(BindManagers([]gdx.IManager{backend, platform}))
	var managers EngineManagers
	for _, manager := range []gdx.IAudioMgr{gdx.AudioMgr, &managers.AudioMgr} {
		if got := manager.GetVolume(object); got != 0.625 {
			t.Fatalf("volume = %g, want 0.625", got)
		}
	}
	wantProbes := 2
	if runtime.GOOS == "js" {
		wantProbes = 0
	}
	if calls != 2 || platform.probes != wantProbes {
		t.Fatalf("backend calls = %d, thread probes = %d; want 2 and %d", calls, platform.probes, wantProbes)
	}
}

func TestBoundManagerPublicAndFacadeCallsDispatchOnce(t *testing.T) {
	previousCallback, previousAudio := mainCallback, gdx.AudioMgr
	t.Cleanup(func() { mainCallback, gdx.AudioMgr = previousCallback, previousAudio })
	dispatches, calls := 0, 0
	inside := false
	Init(func(call func()) {
		dispatches++
		if inside {
			t.Fatal("the facade dispatched its registered wrapper again")
		}
		inside = true
		defer func() { inside = false }()
		call()
	})
	const object gdx.Object = 1<<54 + 3
	volume := 0.0
	marker := new(int)
	backend := &bindingAudioMgr{
		setVolume: func(obj gdx.Object, value float64) {
			calls++
			if !inside || obj != object {
				t.Errorf("setter escaped dispatch or changed object: inside=%v, object=%d", inside, obj)
			}
			volume = value
		},
		getVolume: func(obj gdx.Object) float64 {
			calls++
			if !inside || obj != object {
				t.Errorf("getter escaped dispatch or changed object: inside=%v, object=%d", inside, obj)
			}
			if volume < 0 {
				panic(marker)
			}
			return volume
		},
	}
	t.Cleanup(BindManagers([]gdx.IManager{backend}))
	var managers EngineManagers
	for _, test := range []struct {
		name string
		mgr  gdx.IAudioMgr
	}{
		{"public", gdx.AudioMgr},
		{"facade", &managers.AudioMgr},
	} {
		t.Run(test.name, func(t *testing.T) {
			beforeDispatch, beforeCalls := dispatches, calls
			test.mgr.SetVolume(object, 0.75)
			if got := test.mgr.GetVolume(object); got != 0.75 {
				t.Fatalf("volume = %v, want 0.75", got)
			}
			volume = -1
			if got := catchManagerPanic(func() { test.mgr.GetVolume(object) }); got != marker || inside {
				t.Fatalf("panic = %v, dispatch active=%v; want original panic after cleanup", got, inside)
			}
			if dispatches-beforeDispatch != 3 || calls-beforeCalls != 3 {
				t.Fatalf("dispatches=%d, backend calls=%d, want 3 each", dispatches-beforeDispatch, calls-beforeCalls)
			}
		})
	}
}

func TestFacadePreservesEmbeddedWrapperOverrides(t *testing.T) {
	previousCallback, previousAudio := mainCallback, gdx.AudioMgr
	t.Cleanup(func() { mainCallback, gdx.AudioMgr = previousCallback, previousAudio })
	dispatches, calls := 0, 0
	inside := false
	Init(func(call func()) {
		dispatches++
		inside = true
		defer func() { inside = false }()
		call()
	})
	const object gdx.Object = 1<<54 + 3
	gdx.AudioMgr = &embeddedAudioMgr{getVolume: func(obj gdx.Object) float64 {
		calls++
		if !inside || obj != object {
			t.Errorf("mock override escaped dispatch or changed object: inside=%v, object=%d", inside, obj)
		}
		return 0.625
	}}
	var managers EngineManagers
	if got := managers.AudioMgr.GetVolume(object); got != 0.625 {
		t.Fatalf("volume = %v, want the mock override's 0.625", got)
	}
	if dispatches != 1 || calls != 1 {
		t.Fatalf("dispatches=%d, override calls=%d, want one each", dispatches, calls)
	}
}

func TestUnboundGlobalWrapperRejectsBeforeDispatch(t *testing.T) {
	previousCallback, previousAudio := mainCallback, gdx.AudioMgr
	t.Cleanup(func() { mainCallback, gdx.AudioMgr = previousCallback, previousAudio })
	gdx.AudioMgr = new(AudioMgrImpl)
	dispatches := 0
	Init(func(func()) { dispatches++ })
	var managers EngineManagers
	for _, test := range []struct {
		name string
		call func()
	}{
		{"public", func() { gdx.AudioMgr.GetVolume(1) }},
		{"facade", func() { managers.AudioMgr.GetVolume(1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := catchManagerPanic(test.call); got == nil || dispatches != 0 {
				t.Fatalf("unbound global panic=%v, dispatches=%d; want rejection before recursive dispatch", got, dispatches)
			}
		})
	}
}

func TestManagerBindingsRejectStaleWrappersAndPreserveReplacement(t *testing.T) {
	previousCallback, previousAudio := mainCallback, gdx.AudioMgr
	t.Cleanup(func() { mainCallback, gdx.AudioMgr = previousCallback, previousAudio })
	Init(func(call func()) { call() })
	cleanupConcurrently := func(cleanup func()) {
		start := make(chan struct{})
		var workers sync.WaitGroup
		for range 8 {
			workers.Go(func() {
				<-start
				cleanup()
			})
		}
		close(start)
		workers.Wait()
		cleanup()
	}
	firstCalls, secondCalls := 0, 0
	first := &bindingAudioMgr{getVolume: func(gdx.Object) float64 { firstCalls++; return 0.25 }}
	second := &bindingAudioMgr{getVolume: func(gdx.Object) float64 { secondCalls++; return 0.75 }}
	firstCleanup := BindManagers([]gdx.IManager{first})
	t.Cleanup(firstCleanup)
	firstWrapper := gdx.AudioMgr
	secondCleanup := BindManagers([]gdx.IManager{second})
	t.Cleanup(secondCleanup)
	secondWrapper := gdx.AudioMgr
	cleanupConcurrently(firstCleanup)
	if gdx.AudioMgr != secondWrapper {
		t.Fatal("old cleanup removed the replacement binding")
	}
	if got := catchManagerPanic(func() { firstWrapper.GetVolume(1) }); got == nil || firstCalls != 0 {
		t.Fatalf("stale wrapper panic=%v, raw calls=%d; want rejection before raw access", got, firstCalls)
	}
	if got := secondWrapper.GetVolume(1); got != 0.75 || secondCalls != 1 {
		t.Fatalf("replacement volume=%v, raw calls=%d, want 0.75 and 1", got, secondCalls)
	}
	cleanupConcurrently(secondCleanup)
	if gdx.AudioMgr != nil {
		t.Fatal("cleanup retained the public manager")
	}
	dispatchesAfterClose := 0
	Init(func(func()) { dispatchesAfterClose++ })
	var managers EngineManagers
	for _, test := range []struct {
		name string
		call func()
	}{
		{"retained wrapper", func() { secondWrapper.GetVolume(1) }},
		{"facade", func() { managers.AudioMgr.GetVolume(1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := catchManagerPanic(test.call); got == nil || secondCalls != 1 || dispatchesAfterClose != 0 {
				t.Fatalf("unbound wrapper panic=%v, raw calls=%d, dispatches=%d; want rejection before dispatch or raw access", got, secondCalls, dispatchesAfterClose)
			}
		})
	}
}

func TestManagerBindingChecksLifetimeWhenDispatchExecutes(t *testing.T) {
	previousCallback, previousAudio := mainCallback, gdx.AudioMgr
	t.Cleanup(func() { mainCallback, gdx.AudioMgr = previousCallback, previousAudio })
	for _, facade := range []bool{false, true} {
		for _, replace := range []bool{false, true} {
			t.Run(fmt.Sprintf("facade=%t/replace=%t", facade, replace), func(t *testing.T) {
				calls := 0
				backend := &bindingAudioMgr{setVolume: func(gdx.Object, float64) { calls++ }}
				cleanup := BindManagers([]gdx.IManager{backend})
				t.Cleanup(cleanup)
				manager := gdx.AudioMgr
				if facade {
					manager = new(AudioMgrImpl)
				}
				Init(func(call func()) {
					cleanup()
					if replace {
						t.Cleanup(BindManagers([]gdx.IManager{backend}))
					}
					call()
				})
				if got := catchManagerPanic(func() { manager.SetVolume(1, 0.75) }); got != "enginewrap: audio manager is not bound" || calls != 0 {
					t.Fatalf("closed dispatch panic=%v, raw calls=%d; want rejection before either backend", got, calls)
				}
			})
		}
	}
}

func TestBoundManagerThreadProbeDoesNotDispatch(t *testing.T) {
	previousCallback, previousPlatform := mainCallback, gdx.PlatformMgr
	t.Cleanup(func() { mainCallback, gdx.PlatformMgr = previousCallback, previousPlatform })
	Init(func(func()) { t.Fatal("thread probe attempted recursive dispatch") })
	backend := &bindingPlatformMgr{}
	cleanup := BindManagers([]gdx.IManager{backend})
	t.Cleanup(cleanup)
	retained := gdx.PlatformMgr
	var managers EngineManagers
	if !retained.IsMainThread() || !managers.PlatformMgr.IsMainThread() || backend.probes != 2 {
		t.Fatalf("thread probe did not reach the raw backend twice: probes=%d", backend.probes)
	}
	cleanup()
	if got := catchManagerPanic(func() { retained.IsMainThread() }); got == nil || backend.probes != 2 {
		t.Fatalf("unbound probe panic=%v, raw probes=%d; want rejection without raw access", got, backend.probes)
	}
}

func TestManagerResultWaitsForMainThreadDispatch(t *testing.T) {
	var cameraMgr CameraMgrImpl
	previousCallback, previousCamera := mainCallback, gdx.CameraMgr
	backend := &dispatchCameraMgr{position: mathf.NewVec2(-12, 34)}
	unbind := BindManagers([]gdx.IManager{backend})
	queued := make(chan func(), 1)
	released := make(chan struct{})
	release := sync.OnceFunc(func() { close(released) })
	dispatches := 0
	Init(func(call func()) {
		dispatches++
		if dispatches != 1 {
			t.Error("manager wrapper dispatched again before reaching the backend")
			return
		}
		queued <- call
		<-released
	})
	finished := make(chan mathf.Vec2, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		release()
		<-done
		unbind()
		mainCallback, gdx.CameraMgr = previousCallback, previousCamera
	})
	go func() {
		defer close(done)
		finished <- cameraMgr.GetPosition()
	}()

	var call func()
	select {
	case call = <-queued:
	case <-time.After(5 * time.Second):
		t.Fatal("camera request was not dispatched")
	}
	if backend.reads != 0 {
		t.Fatal("backend ran before the main-thread dispatcher executed the request")
	}
	call()
	if backend.reads != 1 {
		t.Fatalf("backend calls = %d, want one", backend.reads)
	}
	select {
	case <-finished:
		t.Fatal("manager returned before the synchronous dispatcher completed")
	default:
	}
	release()
	select {
	case got := <-finished:
		if got != backend.position {
			t.Fatalf("camera position = %v, want %v", got, backend.position)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("manager did not return after dispatch completed")
	}

	Init(func(call func()) {
		dispatches++
		call()
	})
	cameraMgr.SetPosition(mathf.NewVec2(56, -78))
	if got := cameraMgr.GetPosition(); got != mathf.NewVec2(56, -78) {
		t.Fatalf("camera aliases lost the updated position: %v", got)
	}
	if dispatches != 3 {
		t.Fatalf("manager dispatches = %d, want one per getter/setter call", dispatches)
	}
}

func TestMousePressedUsesPrimaryButtons(t *testing.T) {
	var inputMgr InputMgrImpl
	previousCallback, previousInput := mainCallback, gdx.InputMgr
	Init(func(call func()) { call() })
	t.Cleanup(func() { mainCallback, gdx.InputMgr = previousCallback, previousInput })
	for _, test := range []struct {
		name    string
		buttons []int64
		want    bool
	}{
		{"none", nil, false},
		{"left", []int64{1}, true},
		{"right", []int64{2}, true},
		{"middle only", []int64{3}, false},
		{"both primary", []int64{1, 2}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &dispatchInputMgr{pressed: make(map[int64]bool)}
			for _, button := range test.buttons {
				backend.pressed[button] = true
			}
			t.Cleanup(BindManagers([]gdx.IManager{backend}))
			if got := inputMgr.MousePressed(); got != test.want {
				t.Fatalf("MousePressed() = %v, want %v", got, test.want)
			}
			for _, button := range backend.queries {
				if button != input.MouseButtonLeft && button != input.MouseButtonRight {
					t.Errorf("queried non-primary mouse button %d", button)
				}
			}
		})
	}
}

func TestMousePressedRetainsItsBoundInput(t *testing.T) {
	previousCallback, previousInput := mainCallback, gdx.InputMgr
	t.Cleanup(func() { mainCallback, gdx.InputMgr = previousCallback, previousInput })
	Init(func(call func()) { call() })
	first := &dispatchInputMgr{pressed: map[int64]bool{input.MouseButtonLeft: true}}
	firstCleanup := BindManagers([]gdx.IManager{first})
	t.Cleanup(firstCleanup)
	retained := gdx.InputMgr.(*InputMgrImpl)
	second := &dispatchInputMgr{pressed: map[int64]bool{}}
	t.Cleanup(BindManagers([]gdx.IManager{second}))
	if !retained.MousePressed() || len(second.queries) != 0 {
		t.Fatal("retained manager used the replacement input backend")
	}
	firstCleanup()
	before := len(first.queries)
	if got := catchManagerPanic(func() { retained.MousePressed() }); got == nil || len(first.queries) != before || len(second.queries) != 0 {
		t.Fatalf("stale mouse query panic=%v, raw queries=(%d, %d); want rejection before either backend", got, len(first.queries)-before, len(second.queries))
	}
}

func TestBatchPositionBridgePreservesBuffersAndFailure(t *testing.T) {
	var spriteMgr SpriteMgrImpl
	previousCallback, previousSprite := mainCallback, gdx.SpriteMgr
	insideDispatch := false
	dispatches := 0
	Init(func(call func()) {
		dispatches++
		if insideDispatch {
			t.Fatal("manager wrapper repeated dispatch")
		}
		insideDispatch = true
		defer func() { insideDispatch = false }()
		call()
	})
	t.Cleanup(func() { mainCallback, gdx.SpriteMgr = previousCallback, previousSprite })
	for _, success := range []bool{true, false} {
		dispatches = 0
		ids := []int64{1<<54 + 3, 7}
		positions := []float32{9, 8, 7, 6}
		calls := 0
		backend := &dispatchSpriteMgr{retrieve: func(gotIDs []int64, out []float32) bool {
			calls++
			if !insideDispatch || len(gotIDs) != len(ids) || &gotIDs[0] != &ids[0] || len(out) != len(positions) || &out[0] != &positions[0] {
				t.Fatal("batch buffers were copied or invoked outside main-thread dispatch")
			}
			if success {
				copy(out, []float32{-10, 20, 30, -40})
			}
			return success
		}}
		t.Cleanup(BindManagers([]gdx.IManager{backend}))
		if got := spriteMgr.BatchRetrievePositions(ids, positions); got != success || calls != 1 || dispatches != 1 {
			t.Fatalf("batch result = %v, calls = %d, dispatches = %d, want %v and one call/dispatch", got, calls, dispatches, success)
		}
		want := []float32{9, 8, 7, 6}
		if success {
			want = []float32{-10, 20, 30, -40}
		}
		if !slices.Equal(positions, want) || !slices.Equal(ids, []int64{1<<54 + 3, 7}) {
			t.Fatalf("batch payload changed: IDs=%v positions=%v", ids, positions)
		}
	}
	marker := new(int)
	dispatches = 0
	t.Cleanup(BindManagers([]gdx.IManager{&dispatchSpriteMgr{retrieve: func([]int64, []float32) bool { panic(marker) }}}))
	defer func() {
		if got := recover(); got != marker || insideDispatch || dispatches != 1 {
			t.Errorf("panic = %v, dispatch active = %v, dispatches = %d; want original panic after one dispatch and cleanup", got, insideDispatch, dispatches)
		}
	}()
	spriteMgr.BatchRetrievePositions(nil, nil)
}

// Public globals are captured before starting workers. Installation and direct
// reads of those globals still belong to the engine's coordinated link boundary.
func TestManagerBindingsConcurrentReleaseAndRebind(t *testing.T) {
	previousCallback, previousAudio, previousPlatform := mainCallback, gdx.AudioMgr, gdx.PlatformMgr
	t.Cleanup(func() {
		mainCallback, gdx.AudioMgr, gdx.PlatformMgr = previousCallback, previousAudio, previousPlatform
	})
	for _, dispatch := range []struct {
		name string
		call func(func())
	}{
		{"inline", func(call func()) { call() }},
		{"scheduler", coroutine.New(nil).WaitMainThread},
		{"standalone", nil},
	} {
		t.Run(dispatch.name, func(t *testing.T) {
			Init(dispatch.call)
			audio := &bindingAudioMgr{getVolume: func(gdx.Object) float64 { return 0.5 }}
			platform := &concurrentPlatformMgr{}
			cleanup := BindManagers([]gdx.IManager{audio, platform})
			t.Cleanup(cleanup)
			retainedAudio, retainedPlatform := gdx.AudioMgr, gdx.PlatformMgr
			var facade EngineManagers
			start := make(chan struct{})
			var workers sync.WaitGroup
			for _, call := range []func(){
				func() { retainedAudio.GetVolume(1) },
				func() { facade.AudioMgr.GetVolume(1) },
				func() { retainedPlatform.IsMainThread() },
				func() { facade.PlatformMgr.IsMainThread() },
			} {
				workers.Go(func() {
					<-start
					for range 1000 {
						if got := catchManagerPanic(call); got != nil &&
							got != "enginewrap: audio manager is not bound" &&
							got != "enginewrap: platform manager is not bound" {
							t.Errorf("unexpected manager panic: %v", got)
						}
					}
				})
			}
			close(start)
			for range 100 {
				Init(dispatch.call)
				next := BindManagers([]gdx.IManager{audio, platform})
				cleanup()
				cleanup = next
			}
			workers.Wait()
			cleanup()
		})
	}
}

func TestManagerBindingReleaseFromBackend(t *testing.T) {
	previousCallback, previousAudio := mainCallback, gdx.AudioMgr
	t.Cleanup(func() { mainCallback, gdx.AudioMgr = previousCallback, previousAudio })
	Init(func(call func()) { call() })
	var cleanup func()
	cleanup = BindManagers([]gdx.IManager{&bindingAudioMgr{getVolume: func(gdx.Object) float64 {
		cleanup()
		return 0.5
	}}})
	t.Cleanup(cleanup)
	retained := gdx.AudioMgr
	// A backend may call into Go and release its own binding. No registry lock
	// may cross that callback, and the already accepted call may finish.
	if got := retained.GetVolume(1); got != 0.5 {
		t.Fatalf("accepted call result = %v, want 0.5", got)
	}
	if got := catchManagerPanic(func() { retained.GetVolume(1) }); got != "enginewrap: audio manager is not bound" {
		t.Fatalf("released manager panic = %v", got)
	}
}

func TestBindManagersRollsBackOnUnknownManager(t *testing.T) {
	previousAudio := gdx.AudioMgr
	t.Cleanup(func() { gdx.AudioMgr = previousAudio })
	if got := catchManagerPanic(func() {
		BindManagers([]gdx.IManager{&bindingAudioMgr{}, nil})
	}); got != "enginewrap: unknown manager type <nil>" {
		t.Fatalf("unknown manager panic = %v", got)
	}
	if gdx.AudioMgr != nil {
		t.Fatal("failed binding retained its partially installed audio wrapper")
	}
}

func TestMousePressedUsesOneBindingAndDispatch(t *testing.T) {
	previousCallback, previousInput := mainCallback, gdx.InputMgr
	t.Cleanup(func() { mainCallback, gdx.InputMgr = previousCallback, previousInput })
	first := &dispatchInputMgr{}
	cleanup := BindManagers([]gdx.IManager{first})
	t.Cleanup(cleanup)
	second := &dispatchInputMgr{pressed: map[int64]bool{input.MouseButtonRight: true}}
	dispatches := 0
	Init(func(call func()) {
		dispatches++
		call()
		cleanup()
		t.Cleanup(BindManagers([]gdx.IManager{second}))
	})
	var inputMgr InputMgrImpl
	if inputMgr.MousePressed() || dispatches != 1 || len(second.queries) != 0 {
		t.Fatalf("mouse query crossed bindings or repeated dispatch: dispatches=%d, replacement queries=%v", dispatches, second.queries)
	}
	if !slices.Equal(first.queries, []int64{input.MouseButtonLeft, input.MouseButtonRight}) {
		t.Fatalf("original queries = %v, want left and right buttons", first.queries)
	}
}

func (m *dispatchCameraMgr) GetCameraPosition() mathf.Vec2 {
	m.reads++
	return m.position
}

func (m *dispatchCameraMgr) SetCameraPosition(position mathf.Vec2) {
	m.position = position
}

func (m *dispatchInputMgr) GetMouseState(button int64) bool {
	m.queries = append(m.queries, button)
	return m.pressed[button]
}

func (m *dispatchSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	return m.retrieve(ids, out)
}

func (m *bindingAudioMgr) SetVolume(obj gdx.Object, volume float64) { m.setVolume(obj, volume) }
func (m *bindingAudioMgr) GetVolume(obj gdx.Object) float64         { return m.getVolume(obj) }

func (m *embeddedAudioMgr) GetVolume(obj gdx.Object) float64 { return m.getVolume(obj) }

func (m *bindingPlatformMgr) IsMainThread() bool { m.probes++; return !m.worker }

func (*concurrentPlatformMgr) IsMainThread() bool { return true }

func catchManagerPanic(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}
