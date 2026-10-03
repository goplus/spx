package spx

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

type touchEventSprite struct{ SpriteImpl }

func (*touchEventSprite) Main() {}

func newTouchEventSprite(game *Game, name string) *touchEventSprite {
	sprite := &touchEventSprite{}
	sprite.g, sprite.name, sprite.sprite = game, name, sprite
	sprite.scriptEventBindings.bind(&game.scriptEvents, &sprite.SpriteImpl)
	return sprite
}

func TestTouchStartGate(t *testing.T) {
	previous := gco
	gco = nil
	defer func() { gco = previous }()

	game := new(Game)
	game.bindScriptEvents()
	source := newTouchEventSprite(game, "source")
	target := newTouchEventSprite(game, "target")
	source.fireTouchStart(&target.SpriteImpl)
	if source.spriteState.HasOnTouchStart {
		t.Fatal("touch without a handler enabled dispatch")
	}

	calls := 0
	source.addTouchStartHandler(func(Sprite) { calls++ })
	if !source.spriteState.HasOnTouchStart {
		t.Fatal("registered touch handler did not enable dispatch")
	}
	source.fireTouchStart(&target.SpriteImpl)
	if calls != 1 {
		t.Fatalf("registered touch calls = %d, want 1", calls)
	}

	clone := newTouchEventSprite(game, "source")
	clone.SpriteImpl.InitFrom(&source.SpriteImpl)
	clone.fireTouchStart(&target.SpriteImpl)
	if clone.spriteState.HasOnTouchStart || calls != 1 {
		t.Fatal("clone inherited the source handler gate")
	}
	clone.addTouchStartHandler(func(Sprite) { calls++ })
	if !clone.spriteState.HasOnTouchStart {
		t.Fatal("clone touch handler did not enable dispatch")
	}
	clone.fireTouchStart(&target.SpriteImpl)
	if calls != 2 {
		t.Fatalf("clone touch calls = %d, want 2", calls)
	}
	if got := len(game.scriptEvents.manager.Snapshot(coreevent.BucketTouchStart)); got != 2 {
		t.Fatalf("touch handlers = %d, want 2", got)
	}
}

func newPhysicsTriggerTestGame(t *testing.T) *Game {
	t.Helper()
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous })
	game := new(Game)
	game.bindScriptEvents()
	return game
}

func newPhysicsTriggerTestSprite(game *Game, name string) *touchEventSprite {
	sprite := newTouchEventSprite(game, name)
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &engine.Sprite{Target: &sprite.SpriteImpl}
	return sprite
}

func TestDispatchPhysicsTriggersFiltersPairs(t *testing.T) {
	game := newPhysicsTriggerTestGame(t)
	source := newPhysicsTriggerTestSprite(game, "source")
	target := newPhysicsTriggerTestSprite(game, "target")
	hidden := newPhysicsTriggerTestSprite(game, "hidden")
	hidden.spriteState.IsVisible = false
	dying := newPhysicsTriggerTestSprite(game, "dying")
	dying.spriteState.IsDying = true
	noHandler := newPhysicsTriggerTestSprite(game, "no-handler")
	var nilSprite *SpriteImpl
	typedNil := &engine.Sprite{Target: nilSprite}
	wrongType := &engine.Sprite{Target: "not a sprite"}
	emptyTarget := &engine.Sprite{}
	src, dst := source.runtimeState.SyncSprite, target.runtimeState.SyncSprite

	var calls []Sprite
	for _, sprite := range []*touchEventSprite{source, target, hidden, dying} {
		sprite.addTouchStartHandler(func(other Sprite) { calls = append(calls, other) })
	}
	var logs bytes.Buffer
	spxlog.SetStdoutOutput(&logs)
	t.Cleanup(func() { spxlog.SetStdoutOutput(os.Stdout) })
	for _, tt := range []struct {
		name     string
		src, dst *engine.Sprite
		invalid  bool
		touch    bool
	}{
		{"valid", src, dst, false, true},
		{"nil source", nil, dst, true, false},
		{"nil destination", src, nil, true, false},
		{"both proxies nil", nil, nil, true, false},
		{"wrong source type", wrongType, dst, true, false},
		{"wrong destination type", src, wrongType, true, false},
		{"both types wrong", wrongType, wrongType, true, false},
		{"nil source target", emptyTarget, dst, true, false},
		{"nil destination target", src, emptyTarget, true, false},
		{"typed nil source and invalid destination", typedNil, wrongType, true, false},
		{"invalid source and typed nil destination", wrongType, typedNil, true, false},
		{"hidden source and invalid destination", hidden.runtimeState.SyncSprite, wrongType, true, false},
		{"hidden source", hidden.runtimeState.SyncSprite, dst, false, false},
		{"hidden destination", src, hidden.runtimeState.SyncSprite, false, false},
		{"dying source", dying.runtimeState.SyncSprite, dst, false, false},
		{"dying destination", src, dying.runtimeState.SyncSprite, false, false},
		{"hidden source short circuits typed nil destination", hidden.runtimeState.SyncSprite, typedNil, false, false},
		{"dying source short circuits typed nil destination", dying.runtimeState.SyncSprite, typedNil, false, false},
		{"no source handler", noHandler.runtimeState.SyncSprite, dst, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls, logs = nil, bytes.Buffer{}
			game.triggerEvents = []engine.TriggerEvent{{Src: tt.src, Dst: tt.dst}, {Src: src, Dst: dst}}
			game.dispatchPhysicsTriggers()
			want := []Sprite{target} // Processing must continue after a skipped pair.
			if tt.touch {
				want = append(want, target)
			}
			if !slices.Equal(calls, want) {
				t.Fatalf("touches = %v, want %v", calls, want)
			}
			const invalid = "[INFO] [SPX] Physics error: unexpected trigger pair - invalid sprite types\n"
			if tt.invalid {
				if !strings.HasSuffix(logs.String(), invalid) || strings.Count(logs.String(), "\n") != 1 {
					t.Fatalf("log = %q, want one invalid-pair info message", logs.String())
				}
			} else if logs.Len() != 0 {
				t.Fatalf("log = %q, want none", logs.String())
			}
			if len(game.triggerEvents) != 0 || game.triggerEvents[:1][0] != (engine.TriggerEvent{}) {
				t.Fatal("normal dispatch retained a trigger pair")
			}
		})
	}
}

func TestDispatchPhysicsTriggersOrderAndBufferReuse(t *testing.T) {
	game := newPhysicsTriggerTestGame(t)
	first := newPhysicsTriggerTestSprite(game, "first")
	second := newPhysicsTriggerTestSprite(game, "second")
	last := newPhysicsTriggerTestSprite(game, "last")
	var calls []string
	for _, sprite := range []*touchEventSprite{first, second, last} {
		sprite.addTouchStartHandler(func(other Sprite) {
			calls = append(calls, sprite.name+":"+spriteOf(other).name)
		})
	}
	firstPair := engine.TriggerEvent{Src: first.runtimeState.SyncSprite, Dst: second.runtimeState.SyncSprite}
	backing := make([]engine.TriggerEvent, 4)
	backing[0] = firstPair
	backing[1] = engine.TriggerEvent{Src: second.runtimeState.SyncSprite, Dst: last.runtimeState.SyncSprite}
	backing[2] = engine.TriggerEvent{Src: first.runtimeState.SyncSprite, Dst: last.runtimeState.SyncSprite}
	backing[3] = firstPair // Capacity beyond the active batch is not cleared.
	game.triggerEvents = backing[:3]
	game.dispatchPhysicsTriggers()
	if want := []string{"first:second", "second:last", "first:last"}; !slices.Equal(calls, want) {
		t.Fatalf("touch order = %v, want %v", calls, want)
	}
	if len(game.triggerEvents) != 0 || cap(game.triggerEvents) != len(backing) || &game.triggerEvents[:1][0] != &backing[0] {
		t.Fatal("dispatch did not retain the empty reusable buffer")
	}
	for _, pair := range backing[:3] {
		if pair != (engine.TriggerEvent{}) {
			t.Fatal("dispatch retained a processed pair")
		}
	}
	if backing[3] != firstPair {
		t.Fatal("dispatch cleared beyond the active batch")
	}
	game.triggerEvents = append(game.triggerEvents, firstPair)
	game.dispatchPhysicsTriggers()
	if len(calls) != 4 || calls[3] != "first:second" || backing[0] != (engine.TriggerEvent{}) {
		t.Fatal("reused buffer did not dispatch and clear its next batch")
	}
	game.dispatchPhysicsTriggers()
	if len(calls) != 4 {
		t.Fatal("empty dispatch repeated a touch")
	}
}

func TestDispatchPhysicsTriggersRechecksTouchability(t *testing.T) {
	for _, dying := range []bool{false, true} {
		t.Run(fmt.Sprintf("dying=%v", dying), func(t *testing.T) {
			game := newPhysicsTriggerTestGame(t)
			source := newPhysicsTriggerTestSprite(game, "source")
			target := newPhysicsTriggerTestSprite(game, "target")
			last := newPhysicsTriggerTestSprite(game, "last")
			var calls []Sprite
			source.addTouchStartHandler(func(other Sprite) {
				calls = append(calls, other)
				if dying {
					target.spriteState.IsDying = true
				} else {
					target.spriteState.IsVisible = false
				}
			})
			target.addTouchStartHandler(func(Sprite) { t.Error("untouchable source fired a callback") })
			src, dst := source.runtimeState.SyncSprite, target.runtimeState.SyncSprite
			game.triggerEvents = []engine.TriggerEvent{
				{Src: src, Dst: dst},
				{Src: dst, Dst: src},
				{Src: src, Dst: dst},
				{Src: src, Dst: last.runtimeState.SyncSprite},
			}
			game.dispatchPhysicsTriggers()
			if want := []Sprite{target, last}; !slices.Equal(calls, want) {
				t.Fatalf("touches = %v, want target then last", calls)
			}
		})
	}
}

func TestDispatchPhysicsTriggersPanicRetainsBatch(t *testing.T) {
	for _, name := range []string{"callback", "typed nil source", "typed nil destination", "typed nil source before hidden destination"} {
		t.Run(name, func(t *testing.T) {
			game := newPhysicsTriggerTestGame(t)
			source := newPhysicsTriggerTestSprite(game, "source")
			target := newPhysicsTriggerTestSprite(game, "target")
			panicking := newPhysicsTriggerTestSprite(game, "panicking")
			hidden := newPhysicsTriggerTestSprite(game, "hidden")
			hidden.spriteState.IsVisible = false
			var nilSprite *SpriteImpl
			typedNil := &engine.Sprite{Target: nilSprite}
			calls := 0
			source.addTouchStartHandler(func(Sprite) { calls++ })
			panicValue := new(int)
			panicking.addTouchStartHandler(func(Sprite) { panic(panicValue) })
			src, dst := source.runtimeState.SyncSprite, target.runtimeState.SyncSprite
			pair := engine.TriggerEvent{Src: panicking.runtimeState.SyncSprite, Dst: dst}
			switch name {
			case "typed nil source":
				pair.Src = typedNil
			case "typed nil destination":
				pair.Src, pair.Dst = src, typedNil
			case "typed nil source before hidden destination":
				pair.Src, pair.Dst = typedNil, hidden.runtimeState.SyncSprite
			}
			game.triggerEvents = []engine.TriggerEvent{{Src: src, Dst: dst}, pair, {Src: src, Dst: dst}}
			before := slices.Clone(game.triggerEvents)
			backing := &game.triggerEvents[0]
			defer func() {
				value := recover()
				if value == nil || name == "callback" && value != panicValue {
					t.Fatalf("panic = %v, want the original panic", value)
				}
				if calls != 1 {
					t.Fatalf("completed touches = %d, want only the first pair", calls)
				}
				if !slices.Equal(game.triggerEvents, before) || &game.triggerEvents[0] != backing {
					t.Fatal("panic cleared or replaced the uncompleted batch")
				}
			}()
			game.dispatchPhysicsTriggers()
		})
	}
}
