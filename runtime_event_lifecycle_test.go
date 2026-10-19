package spx

import (
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
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

func TestTargetEventHandlersStayWithTheirOwner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*SpriteImpl, func())
		fire     func(*SpriteImpl, *SpriteImpl)
	}{
		{"click", func(s *SpriteImpl, run func()) { s.OnClick(run) }, func(s, _ *SpriteImpl) {
			s.doWhenClick(s)
		}},
		{"swipe", func(s *SpriteImpl, run func()) { s.OnSwipe__0(90, run) }, func(s, _ *SpriteImpl) {
			s.doWhenSwipe(90, s)
		}},
		{"cloned", func(s *SpriteImpl, run func()) { s.OnCloned__0(run) }, func(s, _ *SpriteImpl) {
			s.doWhenCloned(s, nil)
		}},
		{"touch", func(s *SpriteImpl, run func()) { s.addTouchStartHandler(func(Sprite) { run() }) }, func(s, other *SpriteImpl) {
			s.fireTouchStart(other)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			first := &newTouchEventSprite(game, "first").SpriteImpl
			second := &newTouchEventSprite(game, "second").SpriteImpl
			var calls [2]int
			tc.register(first, func() { calls[0]++ })
			tc.register(second, func() { calls[1]++ })
			tc.fire(first, second)
			co.Update()
			if calls != [2]int{1, 0} {
				t.Fatalf("first target: calls=%v, want [1 0]", calls)
			}
			tc.fire(second, first)
			co.Update()
			if calls != [2]int{1, 1} {
				t.Fatalf("second target: calls=%v, want [1 1]", calls)
			}
		})
	}
}

func TestEventHandlerRetriggerPolicies(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		wantStarted  int
		wantFinished int
	}{
		{"key", 1, 1},
		{"any key", 1, 1},
		{"click", 2, 1},
		{"backdrop", 2, 1},
		{"any backdrop", 2, 1},
		{"message", 2, 1},
		{"any message", 2, 1},
		{"timer", 2, 2},
		{"swipe", 2, 2},
		{"cloned", 2, 2},
		{"touch", 2, 2},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			var fire func()
			switch tc.kind {
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
			case "any backdrop":
				game.OnBackdrop__0(func(BackdropName) { run() })
				fire = func() { game.scriptEvents.doWhenBackdropChanged("scene", false) }
			case "message":
				game.OnMsg__1("msg", run)
				fire = func() { game.Broadcast__0("msg") }
			case "any message":
				game.OnMsg__0(func(MsgName, any) { run() })
				fire = func() { game.Broadcast__0("msg") }
			case "timer":
				itime.OnReload()
				t.Cleanup(itime.OnReload)
				game.OnTimer(0, run)
				fire = func() { game.scriptEvents.doWhenTimer(0) }
			case "swipe":
				game.OnSwipe__0(90, run)
				fire = func() { game.scriptEvents.doWhenSwipe(90, game) }
			case "cloned":
				sprite := newTouchEventSprite(game, "clone")
				sprite.OnCloned__0(run)
				fire = func() { sprite.doWhenCloned(&sprite.SpriteImpl, nil) }
			case "touch":
				sprite := newTouchEventSprite(game, "source")
				other := newTouchEventSprite(game, "target")
				sprite.addTouchStartHandler(func(Sprite) { run() })
				fire = func() { sprite.fireTouchStart(&other.SpriteImpl) }
			}
			fire()
			co.Update()
			if started != 1 || finished != 0 {
				t.Fatalf("first trigger: started=%d finished=%d, want 1/0", started, finished)
			}
			fire()
			co.Update()
			if started != tc.wantStarted || finished != 0 {
				t.Fatalf("retrigger: started=%d finished=%d, want %d/0", started, finished, tc.wantStarted)
			}
			gate.Open()
			co.Update()
			if finished != tc.wantFinished {
				t.Fatalf("finished %d scripts, want %d", finished, tc.wantFinished)
			}
			fire()
			co.Update()
			if started != tc.wantStarted+1 || finished != tc.wantFinished+1 {
				t.Fatalf("after completion: started=%d finished=%d, want %d/%d", started, finished, tc.wantStarted+1, tc.wantFinished+1)
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
	co.StopIf(func(th coroutine.Thread) bool { return th.Owner == clone })
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
			parent := co.Create(game, func(coroutine.Thread) {
				fire()
				fire()
				fire()
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
