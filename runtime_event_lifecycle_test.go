package spx

import (
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestEventHandlerNonMatchingTriggerPreservesInvocation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*Game, func())
		fire     func(*Game, bool)
	}{
		{"key", func(g *Game, run func()) { g.OnKey__0(KeySpace, run) }, func(g *Game, match bool) {
			key := KeyA
			if match {
				key = KeySpace
			}
			g.scriptEvents.doWhenKeyPressed(key)
		}},
		{"click", func(g *Game, run func()) { g.OnClick(run) }, func(g *Game, match bool) {
			if match {
				g.scriptEvents.doWhenClick(g)
			} else {
				g.scriptEvents.doWhenClick(&SpriteImpl{g: g})
			}
		}},
		{"backdrop", func(g *Game, run func()) { g.OnBackdrop__1("scene", run) }, func(g *Game, match bool) {
			name := "other"
			if match {
				name = "scene"
			}
			g.scriptEvents.doWhenBackdropChanged(name, false)
		}},
		{"message", func(g *Game, run func()) { g.OnMsg__1("msg", run) }, func(g *Game, match bool) {
			msg := "other"
			if match {
				msg = "msg"
			}
			g.Broadcast__0(msg)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			tc.register(game, func() { started++; gate.Wait(); finished++ })
			tc.fire(game, true)
			co.Update()
			tc.fire(game, false)
			co.Update()
			gate.Open()
			co.Update()
			if started != 1 || finished != 1 {
				t.Fatalf("non-matching trigger affected invocation: started=%d finished=%d", started, finished)
			}
		})
	}
}

func TestKeyHandlerSharesAdmissionAcrossRegisteredKeys(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	gate := co.NewLatch()
	var received []Key
	keys := []Key{KeySpace, KeyA}
	game.OnKey__1(keys, func(key Key) {
		received = append(received, key)
		gate.Wait()
	})
	keys[1] = KeyB
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeyA)
	co.Update()
	if len(received) != 1 || received[0] != KeySpace {
		t.Fatalf("keys did not share one registration: %v", received)
	}
	gate.Open()
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeyB)
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeyA)
	co.Update()
	if len(received) != 2 || received[1] != KeyA {
		t.Fatalf("registration did not retain its original keys: %v", received)
	}
}

func TestEventHandlerRetriggerPolicies(t *testing.T) {
	for _, kind := range []string{"key", "any key", "click", "backdrop", "message"} {
		t.Run(kind, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			var fire func()
			switch kind {
			case "key":
				game.OnKey__0(KeySpace, run)
				fire = func() { game.scriptEvents.doWhenKeyPressed(KeySpace) }
			case "any key":
				game.OnAnyKey(func(Key) { run() })
				fire = func() { game.scriptEvents.doWhenKeyPressed(KeySpace) }
			case "click":
				game.OnClick(run)
				fire = func() { game.scriptEvents.doWhenClick(game) }
			case "backdrop":
				game.OnBackdrop__1("scene", run)
				fire = func() { game.scriptEvents.doWhenBackdropChanged("scene", false) }
			case "message":
				game.OnMsg__1("msg", run)
				fire = func() { game.Broadcast__0("msg") }
			}
			fire()
			co.Update()
			if started != 1 {
				t.Fatalf("first trigger started %d scripts", started)
			}
			fire()
			co.Update()
			want := 2
			if kind == "key" || kind == "any key" {
				want = 1
			}
			if started != want {
				t.Fatalf("retrigger started %d scripts, want %d", started, want)
			}
			gate.Open()
			co.Update()
			if finished != 1 {
				t.Fatalf("finished %d scripts, want 1", finished)
			}
			fire()
			co.Update()
			if started != want+1 || finished != 2 {
				t.Fatalf("after completion: started=%d finished=%d", started, finished)
			}
		})
	}
}

func TestKeyHandlerIdentityIncludesRegistrationAndOwner(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	gate := co.NewLatch()
	started := 0
	run := func() { started++; gate.Wait() }
	game.OnKey__0(KeySpace, run)
	game.OnKey__0(KeySpace, run)
	clone := &SpriteImpl{g: game}
	clone.spriteState.Cloned = true
	clone.scriptEventBindings.bind(&game.scriptEvents, clone)
	clone.OnKey__0(KeySpace, run)
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	if started != 3 {
		t.Fatalf("started %d scripts, want one per registration and owner", started)
	}
	co.StopIf(func(th coroutine.Thread) bool { return th.Obj == clone })
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	if started != 4 {
		t.Fatalf("canceled clone handler did not restart independently: %d", started)
	}
	gate.Open()
	co.Update()
}

// Retriggers may arrive before a handler gets its first slice. Replacing a
// canceled registration must not let its delayed cleanup clear the new one.
func TestEventHandlerQueuedRetriggers(t *testing.T) {
	for _, key := range []bool{false, true} {
		t.Run(map[bool]string{false: "restart", true: "ignore"}[key], func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			fire := func() { game.scriptEvents.doWhenClick(game) }
			if key {
				game.OnKey__0(KeySpace, run)
				fire = func() { game.scriptEvents.doWhenKeyPressed(KeySpace) }
			} else {
				game.OnClick(run)
			}
			parent := co.Create(game, func(coroutine.Thread) int {
				fire()
				fire()
				fire()
				return 0
			})
			co.Join(parent)
			co.Update()
			if started != 1 {
				t.Fatalf("queued retriggers started %d handlers", started)
			}
			fire()
			co.Update()
			want := 2
			if key {
				want = 1
			}
			if started != want {
				t.Fatalf("retrigger started %d, want %d", started, want)
			}
			gate.Open()
			co.Update()
			if finished != 1 {
				t.Fatalf("finished %d handlers", finished)
			}
		})
	}
}
