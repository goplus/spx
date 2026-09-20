package spx

import (
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestIndependentBroadcastTreeDoesNotInheritReceiverTurn(t *testing.T) {
	for _, fromScript := range []bool{false, true} {
		name := "external"
		if fromScript {
			name = "script"
		}
		t.Run(name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			calls := 0
			game.OnMsg__1("cycle", func() {
				calls++
				if calls == 1 {
					game.Broadcast__0("cycle")
				}
			})

			game.Broadcast__0("cycle")
			co.Update()
			if calls != 1 {
				t.Fatalf("nested broadcast calls = %d, want repeated receiver deferred", calls)
			}

			// A new root restarts the deferred receiver without advancing the
			// frame or sharing the first tree's claimed receiver turn.
			if fromScript {
				threads := co.StartBatch([]coroutine.Task{{
					Owner: game,
					Run:   func(coroutine.Thread) { game.Broadcast__0("cycle") },
				}}, coroutine.BatchAsync)
				co.JoinAll(threads)
			} else {
				game.Broadcast__0("cycle")
			}
			co.Update()
			if calls != 2 {
				t.Fatalf("independent broadcast calls = %d, want new root to run in the same frame", calls)
			}
		})
	}
}

func TestMessageExecutionContextReleasedOnCancelAndReset(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	gate := co.NewLatch()
	started, cleaned := 0, 0
	for range 3 {
		game.OnMsg__1("held", func() {
			started++
			defer func() { cleaned++ }()
			gate.Wait()
		})
	}

	game.Broadcast__0("held")
	co.Update()
	if started != 3 {
		t.Fatalf("started receivers = %d, want 3", started)
	}
	if !co.StopAllAndWait(time.Second) {
		t.Fatal("canceled receivers did not finish cleanup")
	}
	if cleaned != 3 || len(game.scriptEvents.messageExecutions) != 0 {
		t.Fatalf("after cancellation: cleaned = %d, retained contexts = %d", cleaned, len(game.scriptEvents.messageExecutions))
	}

	game.scriptEvents.manager.Reset()
	calls := 0
	game.OnMsg__1("held", func() { calls++ })
	game.BroadcastAndWait__0("held")
	if calls != 1 || len(game.scriptEvents.messageExecutions) != 0 {
		t.Fatalf("after reset and completion: calls = %d, retained contexts = %d", calls, len(game.scriptEvents.messageExecutions))
	}
}
