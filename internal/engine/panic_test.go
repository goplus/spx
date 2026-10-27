//go:build !js && !pure_engine

package engine

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	itime "github.com/goplus/spx/v4/internal/time"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type panicRecordingExt struct {
	gdx.IExtMgr
	messages chan string
	exits    chan int64
}

type panicCalls struct {
	gdx.IExtMgr
	messages []string
	exits    []int64
}

type startupExitPlatform struct {
	gdx.IPlatformMgr
	main       atomic.Uint64
	workerCall chan struct{}
}

type startupExitGame struct {
	bindingTestGame
	start    func()
	rendered atomic.Int32
	ended    atomic.Int32
}

func TestCoroutinePanicReachesRuntimeWithCauseAndFaultStack(t *testing.T) {
	co := coroutine.New(OnPanic)
	originalCo, originalPlatform, originalExt := gco, gdx.PlatformMgr, gdx.ExtMgr
	recorder := &panicRecordingExt{messages: make(chan string, 1), exits: make(chan int64, 1)}
	SetCoroutines(co)
	gdx.PlatformMgr = resetDirectPlatform{}
	gdx.ExtMgr = recorder
	enginewrap.Init(WaitMainThread)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
		SetCoroutines(originalCo)
		gdx.PlatformMgr, gdx.ExtMgr = originalPlatform, originalExt
	})

	co.Create("panic-worker", panicWithSentinelCause)
	select {
	case message := <-recorder.messages:
		for _, want := range []string{"panic-worker: panic: sentinel cause", "stack:", "panicWithSentinelCause"} {
			if !strings.Contains(message, want) {
				t.Errorf("runtime panic message %q does not contain %q", message, want)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("coroutine panic did not reach runtime")
	}
	select {
	case code := <-recorder.exits:
		if code != 1 {
			t.Errorf("panic exit code = %d, want 1", code)
		}
	case <-time.After(time.Second):
		t.Fatal("coroutine panic did not request exit")
	}
}

func TestPanicAdaptersReportCauseAndHonorExitPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		run     func()
		message string
		exit    bool
	}{
		{"defer without panic", func() { defer DeferPanic("worker", "trace", true) }, "", false},
		{"check without panic", func() { defer CheckPanic() }, "", false},
		{"recover without exit", func() {
			defer DeferPanic("worker", "trace", false)
			panic("boom")
		}, "worker: panic: boom\nstack:\ntrace", false},
		{"recover with exit", func() {
			defer DeferPanic("worker", "trace", true)
			panic("boom")
		}, "worker: panic: boom\nstack:\ntrace", true},
		{"callback panic", func() {
			defer CheckPanic()
			panic("callback failed")
		}, "panic: callback failed", true},
		{"formatted panic", func() { Panicf("resource %s failed: %d", "cat", 7) }, "resource cat failed: 7", true},
		{"coroutine creation stack", func() {
			OnPanic(coroutine.PanicReport{Name: "worker", Stack: "fault", CreationStack: "creator", Value: "boom"})
		}, "worker: panic: boom\nstack:\nfault\ncreated at:\ncreator", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateGameBinding(t)
			previousCo, previousExt := gco, gdx.ExtMgr
			t.Cleanup(func() {
				SetCoroutines(previousCo)
				gdx.ExtMgr = previousExt
				enginewrap.Init(WaitMainThread)
			})
			SetCoroutines(nil)
			enginewrap.Init(func(call func()) { call() })
			recorder := &panicCalls{}
			gdx.ExtMgr = recorder
			binding, err := bindGame(new(bindingTestGame), nil)
			if err != nil {
				t.Fatal(err)
			}
			tc.run()
			var wantMessages []string
			if tc.message != "" {
				wantMessages = []string{tc.message}
			}
			if !slices.Equal(recorder.messages, wantMessages) {
				t.Errorf("panic messages = %q, want %q", recorder.messages, wantMessages)
			}
			wantPhase := gameRunning
			var wantExits []int64
			if tc.exit {
				wantPhase, wantExits = gameStopped, []int64{1}
			}
			if !slices.Equal(recorder.exits, wantExits) || binding.loadPhase() != wantPhase || activeGame.Load() != binding {
				t.Fatalf("exit policy changed: exits=%v, want %v; phase=%d, want %d; binding=%p", recorder.exits, wantExits, binding.loadPhase(), wantPhase, activeGame.Load())
			}
		})
	}
}

func TestCaptureFailureStopsFrameAndReportsExit(t *testing.T) {
	isolateGameBinding(t)
	setupExecuteTest(t)
	isolateCaptureState(t)
	resetInputState()
	resetTriggerEvents()
	itime.Start(nil)
	t.Cleanup(func() { resetInputState(); resetTriggerEvents(); itime.Start(nil) })
	previousExt := gdx.ExtMgr
	t.Cleanup(func() { gdx.ExtMgr = previousExt; enginewrap.Init(WaitMainThread) })
	gdx.PlatformMgr = resetDirectPlatform{}
	enginewrap.Init(WaitMainThread)
	recorder := &panicCalls{}
	gdx.ExtMgr = recorder
	var steps []string
	game := &frameRecordingGame{step: func(name string) { steps = append(steps, name) }}
	binding, err := bindGame(game, game)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("capture disk full")
	SetCaptureHandler(func(request CaptureRequest) error {
		steps = append(steps, "capture:"+request.Name)
		return failure
	})
	if err := EnqueueCapture("frame.png", nil); err != nil {
		t.Fatal(err)
	}
	onUpdate(0.25)
	want := []string{"before", "update", "render", "capture:frame.png"}
	if !slices.Equal(steps, want) {
		t.Fatalf("failed frame stages = %v, want %v", steps, want)
	}
	if !slices.Equal(recorder.messages, []string{failure.Error()}) || !slices.Equal(recorder.exits, []int64{1}) {
		t.Fatalf("capture failure reports = %v, exits = %v", recorder.messages, recorder.exits)
	}
	if binding.loadPhase() != gameStopped || HasPendingCaptures() {
		t.Fatal("capture failure did not stop the game and drain its batch")
	}
	onUpdate(0.25)
	if !slices.Equal(steps, want) || len(recorder.messages) != 1 || len(recorder.exits) != 1 {
		t.Fatal("stopped frame repeated work or reported the failure twice")
	}
}

func TestStartupExitReturnsFirstFrameWithoutPublishingSuccess(t *testing.T) {
	for _, test := range []struct {
		name        string
		exit        func()
		code        int64
		beforeFrame bool
	}{
		{"failure", func() { Panic("startup failed") }, 1, false},
		{"exit", func() { RequestExit(7) }, 7, false},
		{"exit-before-frame", func() { RequestExit(7) }, 7, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateGameBinding(t)
			co := coroutine.New(OnPanic)
			previousCo, previousPlatform, previousExt := gco, gdx.PlatformMgr, gdx.ExtMgr
			platform := &startupExitPlatform{workerCall: make(chan struct{}, 1)}
			recorder := &panicRecordingExt{messages: make(chan string, 1), exits: make(chan int64, 1)}
			SetCoroutines(co)
			gdx.PlatformMgr, gdx.ExtMgr = platform, recorder
			enginewrap.Init(WaitMainThread)
			t.Cleanup(func() {
				co.RunAfterStop(nil)
				SetCoroutines(previousCo)
				gdx.PlatformMgr, gdx.ExtMgr = previousPlatform, previousExt
			})

			game := new(startupExitGame)
			var started, inFrame, exitedInFrame atomic.Bool
			game.start = func() {
				co.Create(game, func(coroutine.Thread) {
					if test.beforeFrame {
						test.exit()
					} else {
						WaitMainThread(func() {
							exitedInFrame.Store(inFrame.Load())
							test.exit()
						})
					}
					started.Store(true)
				})
			}
			binding, err := bindGame(game, game)
			if err != nil {
				t.Fatal(err)
			}

			updated := make(chan struct{})
			go func() {
				platform.main.Store(gid.Get())
				onStart()
				if test.beforeFrame {
					// Queue the exit request before the first frame.
					<-platform.workerCall
				}
				inFrame.Store(true)
				onUpdate(1.0 / 30)
				inFrame.Store(false)
				close(updated)
			}()
			waitForEngineSignal(t, updated, time.Second, "first frame did not return after startup exit")
			co.RunAfterStop(nil)
			if !test.beforeFrame && !exitedInFrame.Load() {
				t.Fatal("startup exit did not run inside the first frame")
			}
			if started.Load() {
				t.Fatal("startup published success after exit")
			}
			if game.rendered.Load() != 0 || game.ended.Load() != 0 {
				t.Fatalf("render/frame-end callbacks after exit = %d/%d", game.rendered.Load(), game.ended.Load())
			}
			if phase := binding.loadPhase(); phase != gameStopped {
				t.Fatalf("phase after exit = %v, want stopped", phase)
			}
			select {
			case code := <-recorder.exits:
				if code != test.code {
					t.Fatalf("exit code = %d, want %d", code, test.code)
				}
			default:
				t.Fatal("startup did not request backend exit")
			}
			onDestroy()
			onDestroyed()
			if activeGame.Load() != nil || game.destroyed.Load() != 1 {
				t.Fatalf("teardown left binding %p and %d destroy callbacks", activeGame.Load(), game.destroyed.Load())
			}
		})
	}
}

func (r *panicRecordingExt) OnRuntimePanic(message string) { r.messages <- message }

func (r *panicRecordingExt) RequestExit(code int64) { r.exits <- code }

func (r *panicCalls) OnRuntimePanic(message string) { r.messages = append(r.messages, message) }

func (r *panicCalls) RequestExit(code int64) { r.exits = append(r.exits, code) }

func (p *startupExitPlatform) IsMainThread() bool {
	if p.main.Load() == gid.Get() {
		return true
	}
	select {
	case p.workerCall <- struct{}{}:
	default:
	}
	return false
}

func (*startupExitPlatform) SetTimeScale(float64) {}

func (g *startupExitGame) OnEngineStart() { g.start() }

func (g *startupExitGame) OnEngineRender(float64) { g.rendered.Add(1) }

func (g *startupExitGame) OnEngineFrameEnd() { g.ended.Add(1) }

func panicWithSentinelCause(coroutine.Thread) {
	panic("sentinel cause")
}
