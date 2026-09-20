//go:build !js && !pure_engine

package engine

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

func TestExitDuringReloadPreventsActivation(t *testing.T) {
	isolateGameBinding(t)
	setupExecuteTest(t)
	previousExt := gdx.ExtMgr
	recorder := &panicRecordingExt{exits: make(chan int64, 1)}
	gdx.PlatformMgr = resetDirectPlatform{}
	gdx.ExtMgr = recorder
	enginewrap.Init(WaitMainThread)
	t.Cleanup(func() { gdx.ExtMgr = previousExt })

	game := new(bindingTestGame)
	binding, err := bindGame(game, game)
	if err != nil {
		t.Fatal(err)
	}
	err = Reload(game, time.Second, nil, func() error {
		RequestExit(7)
		return nil
	}, func() { t.Error("reload activated after exit was requested") })
	if !errors.Is(err, ErrReloadUnavailable) || binding.loadPhase() != gameStopped {
		t.Fatalf("reload after exit: error=%v, phase=%d", err, binding.loadPhase())
	}
	select {
	case code := <-recorder.exits:
		if code != 7 {
			t.Fatalf("exit code = %d, want 7", code)
		}
	default:
		t.Fatal("reload did not deliver the backend exit request")
	}
	onDestroy()
	onDestroyed()
	if activeGame.Load() != nil || game.destroyed.Load() != 1 {
		t.Fatal("backend teardown did not release the stopped reload")
	}
}

type startupExitPlatform struct {
	gdx.IPlatformMgr
	main       atomic.Uint64
	workerCall chan struct{}
}

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

type startupExitGame struct {
	bindingTestGame
	start    func()
	rendered atomic.Int32
	ended    atomic.Int32
}

func (g *startupExitGame) OnEngineStart()         { g.start() }
func (g *startupExitGame) OnEngineRender(float64) { g.rendered.Add(1) }
func (g *startupExitGame) OnEngineFrameEnd()      { g.ended.Add(1) }

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
				if !co.StopAllAndWait(time.Second) {
					t.Error("startup coroutine did not stop")
				}
				SetCoroutines(previousCo)
				gdx.PlatformMgr, gdx.ExtMgr = previousPlatform, previousExt
			})

			game := new(startupExitGame)
			var started, exitedInFrame atomic.Bool
			game.start = func() {
				co.Create(game, func(coroutine.Thread) int {
					if test.beforeFrame {
						test.exit()
					} else {
						WaitMainThread(func() {
							exitedInFrame.Store(updateBusy.Load())
							test.exit()
						})
					}
					OnGameStarted()
					started.Store(true)
					return 0
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
				onUpdate(1.0 / 30)
				close(updated)
			}()
			select {
			case <-updated:
			case <-time.After(time.Second):
				t.Fatal("first frame did not return after startup exit")
			}
			if !co.StopAllAndWait(time.Second) {
				t.Fatal("startup coroutine did not finish after exit")
			}
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
