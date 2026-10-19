package spx

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/visualfc/gid"
)

func TestGoWithActiveGame(t *testing.T) {
	for _, tc := range []struct {
		name  string
		owner any
		want  any
	}{
		{name: "explicit owner", owner: "sprite", want: "sprite"},
		{name: "default owner", want: "game"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			co := newAPICoroutines(t, "game")

			started := make(chan apiInvocation, 1)
			Go(tc.owner, func(ctx context.Context, owner any) {
				invocation := captureAPIInvocation(co, ctx, owner)
				started <- invocation
				if invocation.thread != nil {
					WaitNextFrame()
				}
			})

			got := awaitAPI(t, started)
			if got.thread == nil {
				t.Fatal("Go did not run its callback in an SPX coroutine")
			}
			if got.owner != tc.want || got.thread.Owner != tc.want {
				t.Fatalf("callback owner = %v, coroutine owner = %v, want %v", got.owner, got.thread.Owner, tc.want)
			}
			if got.ctx != got.thread.Context() || got.ctx.Err() != nil {
				t.Fatal("Go callback did not receive its live coroutine context")
			}

			joinAPIThread(t, co, got.thread, true)
			co.Stop(got.thread)
			joinAPIThread(t, co, got.thread, false)
			if got.ctx.Err() != context.Canceled {
				t.Fatalf("callback context error after stop = %v, want context.Canceled", got.ctx.Err())
			}
		})
	}
}

func TestIsAbortThreadError(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		want  bool
	}{
		{name: "abort", value: coroutine.ErrAbortThread, want: true},
		{name: "nil"},
		{name: "stop this script", value: coroutine.ErrStopThisScript},
		{name: "same message", value: errors.New(coroutine.ErrAbortThread.Error())},
		{name: "wrapped", value: fmt.Errorf("callback: %w", coroutine.ErrAbortThread)},
		{name: "other panic", value: "abort"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := IsAbortThreadError(test.value); got != test.want {
				t.Errorf("IsAbortThreadError(%v) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestExecuteWaitsForCallback(t *testing.T) {
	for _, test := range []struct {
		name              string
		game, owner, want any
	}{
		{name: "native owner", owner: "sprite", want: "sprite"},
		{name: "native nil owner"},
		{name: "active owner", game: "game", owner: "sprite", want: "sprite"},
		{name: "active default owner", game: "game", want: "game"},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := newAPICoroutines(t, test.game)
			started := make(chan apiInvocation, 1)
			returned := make(chan struct{})
			release := apiReleaseGate(t, returned)
			go func() {
				caller := gid.Get()
				Execute(test.owner, func(ctx context.Context, owner any) {
					if test.game == nil && gid.Get() == caller {
						t.Error("native Execute did not start a separate goroutine")
					}
					started <- captureAPIInvocation(co, ctx, owner)
					<-release.channel
				})
				close(returned)
			}()
			got := awaitAPI(t, started)
			if got.owner != test.want {
				t.Fatalf("callback owner = %v, want %v", got.owner, test.want)
			}
			if test.game == nil {
				if got.thread != nil || got.ctx != context.Background() {
					t.Fatal("native Execute did not use a background context outside coroutines")
				}
			} else if got.thread == nil || got.thread.Owner != test.want || got.ctx != got.thread.Context() || got.ctx.Err() != nil {
				t.Fatal("active Execute did not use the owner's live coroutine context")
			}
			select {
			case <-returned:
				t.Fatal("Execute returned while its callback was still blocked")
			default:
			}
			release.open()
			awaitAPI(t, returned)
		})
	}
}

func TestExecuteKeepsCurrentCoroutine(t *testing.T) {
	co := newAPICoroutines(t, "game")
	results := make(chan apiInvocation, 2)
	thread := co.Create("parent", func(coroutine.Thread) {
		for _, owner := range []any{nil, "override"} {
			Execute(owner, func(ctx context.Context, owner any) {
				results <- captureAPIInvocation(co, ctx, owner)
			})
		}
	})
	joinAPIThread(t, co, thread, false)
	for _, want := range []any{nil, "override"} {
		got := awaitAPI(t, results)
		if got.owner != want || got.thread != thread || got.ctx != thread.Context() || thread.Owner != "parent" {
			t.Fatalf("nested Execute changed thread/context/owner: %#v, want owner %v", got, want)
		}
	}
}

func TestGoWithoutActiveGame(t *testing.T) {
	for _, owner := range []any{nil, "sprite"} {
		t.Run(fmt.Sprint(owner), func(t *testing.T) {
			co := newAPICoroutines(t, nil)
			started := make(chan apiInvocation, 1)
			returned, finished := make(chan struct{}), make(chan struct{})
			release := apiReleaseGate(t, returned, finished)
			go func() {
				Go(owner, func(ctx context.Context, owner any) {
					defer close(finished)
					started <- captureAPIInvocation(co, ctx, owner)
					<-release.channel
				})
				close(returned)
			}()
			awaitAPI(t, returned)
			got := awaitAPI(t, started)
			if got.owner != owner || got.thread != nil || got.ctx != context.Background() {
				t.Fatalf("native Go invocation = %#v, want background context and owner %v", got, owner)
			}
			select {
			case <-finished:
				t.Fatal("callback finished before its gate opened")
			default:
			}
			release.open()
			awaitAPI(t, finished)
		})
	}
}

func TestGoResolvesNestedOwner(t *testing.T) {
	for _, test := range []struct {
		name                string
		parent, owner, want any
	}{
		{name: "inherit", parent: "parent", want: "parent"},
		{name: "override", parent: "parent", owner: "child", want: "child"},
		{name: "inherit nil"},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := newAPICoroutines(t, "game")
			started := make(chan apiInvocation, 1)
			parent := co.Create(test.parent, func(coroutine.Thread) {
				Go(test.owner, func(ctx context.Context, owner any) {
					started <- captureAPIInvocation(co, ctx, owner)
					WaitNextFrame()
				})
			})
			got := awaitAPI(t, started)
			if got.thread == nil || got.thread == parent || got.owner != test.want || got.thread.Owner != test.want || got.ctx != got.thread.Context() {
				t.Fatalf("nested Go invocation = %#v, want a new coroutine owned by %v", got, test.want)
			}
			joinAPIThread(t, co, parent, false)
			joinAPIThread(t, co, got.thread, true)
			co.Stop(got.thread)
			joinAPIThread(t, co, got.thread, false)
		})
	}
}

func TestExecuteNativeOutsideCoroutine(t *testing.T) {
	newAPICoroutines(t, nil)
	caller := gid.Get()
	called := false
	ExecuteNative(func(ctx context.Context, owner any) {
		called = true
		if ctx != context.Background() || owner != nil || IsInCoroutine() || gid.Get() != caller {
			t.Error("ExecuteNative outside a coroutine must run inline with a background context and nil owner")
		}
	})
	if !called {
		t.Fatal("ExecuteNative returned without invoking its callback")
	}
}

func TestExecuteNativeFromCoroutine(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "completion", true: "cancellation"}[cancel], func(t *testing.T) {
			co := newAPICoroutines(t, "game")
			started := make(chan apiInvocation, 1)
			finished := make(chan struct{})
			release := apiReleaseGate(t, finished)
			resumed := make(chan bool, 1)
			thread := co.Create("sprite", func(coroutine.Thread) {
				ExecuteNative(func(ctx context.Context, owner any) {
					defer close(finished)
					started <- captureAPIInvocation(co, ctx, owner)
					select {
					case <-release.channel:
					case <-ctx.Done():
					}
				})
				resumed <- IsInCoroutine()
			})
			got := awaitAPI(t, started)
			if got.ctx != thread.Context() || got.owner != "sprite" || got.thread != nil || got.ctx.Err() != nil {
				t.Fatalf("native worker invocation = %#v, want caller context/owner outside coroutine", got)
			}
			joinAPIThread(t, co, thread, true)
			progress := make(chan struct{})
			other := co.Create("other", func(coroutine.Thread) { close(progress) })
			awaitAPI(t, progress)
			joinAPIThread(t, co, other, false)
			select {
			case <-resumed:
				t.Fatal("ExecuteNative resumed before native work completed")
			default:
			}
			if cancel {
				co.Stop(thread)
			} else {
				release.open()
			}
			awaitAPI(t, finished)
			joinAPIThread(t, co, thread, false)
			if cancel {
				if got.ctx.Err() != context.Canceled {
					t.Fatalf("worker context error = %v, want cancellation", got.ctx.Err())
				}
				select {
				case <-resumed:
					t.Fatal("canceled script resumed after ExecuteNative")
				default:
				}
			} else if !awaitAPI(t, resumed) {
				t.Fatal("ExecuteNative did not resume its managed caller")
			}
		})
	}
}

func TestWaitOutsideCoroutine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for _, test := range []struct {
			name string
			wait func() float64
			want time.Duration
		}{
			{name: "positive", wait: func() float64 { return Wait(0.025) }, want: 25 * time.Millisecond},
			{name: "zero", wait: func() float64 { return Wait(0) }},
			{name: "negative", wait: func() float64 { return Wait(-1) }},
			{name: "next frame", wait: WaitNextFrame, want: 16 * time.Millisecond},
		} {
			start := time.Now()
			got := test.wait()
			if elapsed := time.Since(start); elapsed != test.want || got != test.want.Seconds() {
				t.Errorf("%s: elapsed=%v, returned=%v, want %v", test.name, elapsed, got, test.want)
			}
		}
	})
}

func TestWaitUsesLogicalFrames(t *testing.T) {
	itime.Start(nil)
	t.Cleanup(func() { itime.Start(nil) })
	co := newAPICoroutines(t, "game")
	results := make(chan float64, 2)
	thread := co.Create("waiter", func(coroutine.Thread) {
		results <- Wait(0.25)
		results <- WaitNextFrame()
	})
	joinAPIThread(t, co, thread, true)
	for range 2 {
		itime.Update(0.125, 8)
		co.Update()
		if itime.Frame() == 1 {
			select {
			case <-results:
				t.Fatal("Wait resumed before its logical deadline")
			default:
			}
		}
	}
	if got := awaitAPI(t, results); got != 0.25 {
		t.Fatalf("Wait returned %v, want 0.25 logical seconds", got)
	}
	select {
	case <-results:
		t.Fatal("WaitNextFrame resumed within the same frame")
	default:
	}
	itime.Update(0.0625, 16)
	co.Update()
	if got := awaitAPI(t, results); got != 0.0625 {
		t.Fatalf("WaitNextFrame returned %v, want logical delta 0.0625", got)
	}
	joinAPIThread(t, co, thread, false)
}

type apiInvocation struct {
	ctx    context.Context
	owner  any
	thread coroutine.Thread
}

func captureAPIInvocation(co *coroutine.Coroutines, ctx context.Context, owner any) apiInvocation {
	var thread coroutine.Thread
	if IsInCoroutine() {
		thread = co.Current()
	}
	return apiInvocation{ctx, owner, thread}
}

func newAPICoroutines(t *testing.T, game any) *coroutine.Coroutines {
	t.Helper()
	co := coroutine.New(func(report coroutine.PanicReport) {
		t.Errorf("unexpected coroutine panic: %v", report.Value)
	})
	previousGame := engine.GetGame()
	engine.SetGame(game)
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
		engine.SetCoroutines(nil)
		engine.SetGame(previousGame)
	})
	return co
}

type apiGate struct {
	channel <-chan struct{}
	open    func()
}

func apiReleaseGate(t *testing.T, finished ...<-chan struct{}) apiGate {
	t.Helper()
	ch := make(chan struct{})
	open := sync.OnceFunc(func() { close(ch) })
	t.Cleanup(func() {
		open()
		for _, done := range finished {
			awaitAPI(t, done)
		}
	})
	return apiGate{ch, open}
}

func awaitAPI[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for API callback")
		var zero T
		return zero
	}
}

func joinAPIThread(t *testing.T, co *coroutine.Coroutines, thread coroutine.Thread, firstSlice bool) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		if firstSlice {
			co.JoinFirstSlice(thread)
		} else {
			co.Join(thread)
		}
		close(done)
	}()
	awaitAPI(t, done)
}
