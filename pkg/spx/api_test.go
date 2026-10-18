package spx

import (
	"context"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
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
			co := coroutine.New(func(report coroutine.PanicReport) {
				t.Errorf("unexpected coroutine panic: %v", report.Value)
			})
			previousGame := engine.GetGame()
			engine.SetGame("game")
			engine.SetCoroutines(co)
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := co.RunAfterStop(ctx, nil); err != nil {
					t.Errorf("stop test coroutines: %v", err)
				}
				engine.SetCoroutines(nil)
				engine.SetGame(previousGame)
			})

			type invocation struct {
				ctx    context.Context
				owner  any
				thread coroutine.Thread
			}
			started := make(chan invocation, 1)
			Go(tc.owner, func(ctx context.Context, owner any) {
				var thread coroutine.Thread
				if IsInCoroutine() {
					thread = co.Current()
				}
				started <- invocation{ctx, owner, thread}
				if thread != nil {
					WaitNextFrame()
				}
			})

			var got invocation
			select {
			case got = <-started:
			case <-time.After(time.Second):
				t.Fatal("Go callback did not start")
			}
			if got.thread == nil {
				t.Fatal("Go did not run its callback in an SPX coroutine")
			}
			if got.owner != tc.want || got.thread.Owner != tc.want {
				t.Fatalf("callback owner = %v, coroutine owner = %v, want %v", got.owner, got.thread.Owner, tc.want)
			}
			if got.ctx != got.thread.Context() || got.ctx.Err() != nil {
				t.Fatal("Go callback did not receive its live coroutine context")
			}

			co.JoinFirstSlice(got.thread)
			co.Stop(got.thread)
			co.Join(got.thread)
			if got.ctx.Err() != context.Canceled {
				t.Fatalf("callback context error after stop = %v, want context.Canceled", got.ctx.Err())
			}
		})
	}
}
