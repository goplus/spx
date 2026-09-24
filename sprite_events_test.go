package spx

import (
	"testing"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
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
