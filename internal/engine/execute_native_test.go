//go:build !js && !pure_engine

package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/coroutine"
	gde "github.com/goplus/spx/v4/internal/gdengine"
	itime "github.com/goplus/spx/v4/internal/time"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type executeMainThreadPlatform struct {
	gdx.IPlatformMgr
	mainGID atomic.Uint64
}

func (p *executeMainThreadPlatform) IsMainThread() bool {
	return gid.Get() == p.mainGID.Load()
}

func TestExecuteOnEngineThreadServicesMainThreadCalls(t *testing.T) {
	setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	returned := make(chan struct{})
	var calls atomic.Int32
	var wrongThread atomic.Bool
	go func() {
		platform.mainGID.Store(gid.Get())
		Execute("engine-caller", func(context.Context, any) {
			for range 2 {
				WaitMainThread(func() {
					if !platform.IsMainThread() {
						wrongThread.Store(true)
					}
					calls.Add(1)
				})
			}
		})
		close(returned)
	}()
	waitForEngineSignal(t, returned, time.Second, "Execute on the engine thread did not service main-thread calls")
	if calls.Load() != 2 || wrongThread.Load() {
		t.Fatalf("main-thread calls = %d, wrong thread = %v", calls.Load(), wrongThread.Load())
	}
}

func TestFrameRuntimeCallbacksCanRegisterScriptsAndCallEngine(t *testing.T) {
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	resetInputState()
	resetTriggerEvents()
	resetRuntimeDeltaTestState()
	t.Cleanup(func() {
		co.RunAfterStop(nil)
		resetRuntimeDeltaTestState()
		resetInputState()
		resetTriggerEvents()
	})
	isolateCaptureState(t)

	var runtimeManaged, nestedManaged, calledEngine, scriptRan, renderSawScript atomic.Bool
	game := &frameRecordingGame{step: func(name string) {
		if name == "render" {
			renderSawScript.Store(scriptRan.Load())
		}
	}}
	if _, err := bindGame(game, game); err != nil {
		t.Fatal(err)
	}
	delayCall(0, func() {
		runtimeManaged.Store(IsInCoroutine())
		Execute(nil, func(context.Context, any) {
			nestedManaged.Store(IsInCoroutine())
			WaitMainThread(func() { calledEngine.Store(platform.IsMainThread()) })
		})
		// Model the asynchronous batch used by broadcasts.
		co.StartBatch([]coroutine.Task{{Owner: game, Run: func(coroutine.Thread) {
			WaitMainThread(func() { scriptRan.Store(platform.IsMainThread()) })
		}}}, coroutine.BatchAsync)
	})
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		platform.mainGID.Store(gid.Get())
		onUpdate(0.25)
	}()
	waitForEngineSignal(t, returned, time.Second, "frame callback did not complete its engine calls and registered scripts")
	if !runtimeManaged.Load() || !nestedManaged.Load() || !calledEngine.Load() || !renderSawScript.Load() {
		t.Fatalf("runtime managed=%v nested managed=%v engine call=%v render saw script=%v",
			runtimeManaged.Load(), nestedManaged.Load(), calledEngine.Load(), renderSawScript.Load())
	}
}

func TestExecuteRejectsMainThreadCallbackReentry(t *testing.T) {
	setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	done := make(chan struct{})
	var recovered any
	var nestedRan, subsequentRan bool
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		Execute("outer", func(context.Context, any) {
			WaitMainThread(func() {
				defer func() { recovered = recover() }()
				Execute("nested", func(context.Context, any) { nestedRan = true })
			})
		})
		Execute("subsequent", func(context.Context, any) { subsequentRan = true })
	}()
	waitForEngineSignal(t, done, 2*time.Second, "main-thread callback deadlocked reentering Execute")
	if recovered != coroutine.ErrReentrantWait || nestedRan || !subsequentRan {
		t.Fatalf("reentry panic = %v, nested ran = %v, subsequent ran = %v", recovered, nestedRan, subsequentRan)
	}
}

func TestExecuteRejectsBetweenScriptsReentry(t *testing.T) {
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	done := make(chan struct{})
	var recovered any
	var nestedRan, subsequentRan bool
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		co.RunBetweenScripts(func() {
			defer func() { recovered = recover() }()
			Execute("nested", func(context.Context, any) { nestedRan = true })
		})
		co.RunBetweenScripts(func() { subsequentRan = true })
	}()
	waitForEngineSignal(t, done, 2*time.Second, "between-scripts callback deadlocked reentering Execute")
	if recovered != coroutine.ErrReentrantWait || nestedRan || !subsequentRan {
		t.Fatalf("reentry panic = %v, nested ran = %v, subsequent ran = %v", recovered, nestedRan, subsequentRan)
	}
}

func TestExecuteFromNativeWorkerCanCallMainThread(t *testing.T) {
	setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	done := make(chan struct{})
	called := false
	go func() {
		defer close(done)
		platform.mainGID.Store(gid.Get())
		Execute("outer", func(context.Context, any) {
			ExecuteNative(func(context.Context, any) {
				Execute("nested", func(context.Context, any) {
					WaitMainThread(func() { called = platform.IsMainThread() })
				})
			})
		})
	}()
	waitForEngineSignal(t, done, 2*time.Second, "worker could not reenter Execute and call the engine")
	if !called {
		t.Fatal("worker's managed callback did not run its engine call on the main thread")
	}
}

func TestEngineDrainServicesWorkerMainThreadCleanup(t *testing.T) {
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	started := make(chan struct{})
	called := false
	co.Create("caller", func(thread coroutine.Thread) {
		co.WaitToDo(func() {
			close(started)
			<-thread.Context().Done()
			WaitMainThread(func() { called = platform.IsMainThread() })
		})
	})
	waitForEngineSignal(t, started, 2*time.Second, "worker did not start")
	drained := make(chan struct{})
	go func() {
		platform.mainGID.Store(gid.Get())
		co.RunAfterStop(nil)
		close(drained)
	}()
	waitForEngineSignal(t, drained, 2*time.Second, "engine drain did not return")
	if !called {
		t.Fatal("drain did not service worker cleanup on the main thread")
	}
}

func TestEngineDrainStopsWaitsWithoutUpdate(t *testing.T) {
	for _, test := range []struct {
		name string
		wait func(*coroutine.Coroutines, coroutine.Thread)
	}{
		{"Wait", func(co *coroutine.Coroutines, _ coroutine.Thread) { co.Wait(3600) }},
		{"WaitNextFrame", func(co *coroutine.Coroutines, _ coroutine.Thread) { co.WaitNextFrame() }},
		{"WaitYield", (*coroutine.Coroutines).WaitYield},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := setupExecuteTest(t)
			platform := &executeMainThreadPlatform{}
			platform.mainGID.Store(gid.Get())
			gdx.PlatformMgr = platform
			cleaned, afterCalled := false, false
			thread := co.Create("suspended-script", func(me coroutine.Thread) {
				defer co.CleanupMainThread(func() {
					if !platform.IsMainThread() {
						t.Error("cleanup did not run on the engine thread")
					}
					cleaned = true
				})
				test.wait(co, me)
				t.Error("canceled script resumed after its wait")
			})
			co.JoinFirstSlice(thread)
			frame, round := itime.Frame(), co.ScriptRound()
			co.RunAfterStop(func() {
				afterCalled = true
				if !cleaned {
					t.Error("shutdown callback ran before script cleanup")
				}
			})
			if !thread.Stopped() || !afterCalled {
				t.Fatalf("shutdown stopped=%v after=%v", thread.Stopped(), afterCalled)
			}
			if itime.Frame() != frame || co.ScriptRound() != round {
				t.Fatal("shutdown advanced a frame or script round")
			}
		})
	}
}

func TestEngineDrainServicesJobsWhileAnotherDrainOwnsLock(t *testing.T) {
	co := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	called := false
	co.Create("caller", func(thread coroutine.Thread) {
		co.WaitToDo(func() {
			close(started)
			<-thread.Context().Done()
			close(canceled)
			<-release
			WaitMainThread(func() { called = platform.IsMainThread() })
		})
	})
	waitForEngineSignal(t, started, 2*time.Second, "worker did not start")
	first := make(chan struct{})
	go func() {
		co.RunAfterStop(nil)
		close(first)
	}()
	// Cancellation proves the first drain owns the shutdown gate before the engine waits.
	waitForEngineSignal(t, canceled, 2*time.Second, "first drain did not cancel the caller")
	engineStarted := make(chan struct{})
	second := make(chan struct{})
	go func() {
		platform.mainGID.Store(gid.Get())
		close(engineStarted)
		co.RunAfterStop(nil)
		close(second)
	}()
	waitForEngineSignal(t, engineStarted, 2*time.Second, "engine drain did not start")
	unblock()
	waitForEngineSignal(t, first, 2*time.Second, "first engine drain did not return")
	waitForEngineSignal(t, second, 2*time.Second, "second engine drain did not return")
	if !called {
		t.Fatal("waiting for the shutdown gate prevented worker cleanup on the main thread")
	}
}

func TestLinkedGameResetReleasesBackendForNextGame(t *testing.T) {
	isolateGameBinding(t)
	previousCo := gco
	SetCoroutines(nil)
	previousAudio, previousCamera, previousDebug := gdx.AudioMgr, gdx.CameraMgr, gdx.DebugMgr
	previousExt, previousInput, previousNavigation := gdx.ExtMgr, gdx.InputMgr, gdx.NavigationMgr
	previousPen, previousPhysics, previousPlatform := gdx.PenMgr, gdx.PhysicsMgr, gdx.PlatformMgr
	previousRes, previousScene, previousSprite := gdx.ResMgr, gdx.SceneMgr, gdx.SpriteMgr
	previousTilemap, previousTilemapparser, previousUI := gdx.TilemapMgr, gdx.TilemapparserMgr, gdx.UiMgr
	t.Cleanup(func() {
		SetCoroutines(previousCo)
		gdx.AudioMgr, gdx.CameraMgr, gdx.DebugMgr = previousAudio, previousCamera, previousDebug
		gdx.ExtMgr, gdx.InputMgr, gdx.NavigationMgr = previousExt, previousInput, previousNavigation
		gdx.PenMgr, gdx.PhysicsMgr, gdx.PlatformMgr = previousPen, previousPhysics, previousPlatform
		gdx.ResMgr, gdx.SceneMgr, gdx.SpriteMgr = previousRes, previousScene, previousSprite
		gdx.TilemapMgr, gdx.TilemapparserMgr, gdx.UiMgr = previousTilemap, previousTilemapparser, previousUI
	})

	for range 2 {
		game := new(bindingTestGame)
		binding, err := bindGame(game, game)
		if err != nil {
			t.Fatal(err)
		}
		// Preparing another real link would panic if the previous reset only
		// cleared the game binding without releasing its backend session.
		binding.link = gde.PrepareLink(gdx.CoreCallbackInfo{})
		t.Cleanup(binding.link.Unlink)
		onReset()
		if game.reset.Load() != 1 || activeGame.Load() != nil {
			t.Fatalf("reset callbacks=%d active binding=%p", game.reset.Load(), activeGame.Load())
		}
	}
}
