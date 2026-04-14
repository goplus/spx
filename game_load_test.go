package spx

import (
	"reflect"
	"testing"

	coreproject "github.com/goplus/spx/v2/internal/core/project"
)

type cameraFollowOverrideGame struct {
	Game
}

type cameraFollowOverrideSprite struct {
	SpriteImpl
	followTarget SpriteName
}

func (s *cameraFollowOverrideSprite) Main() {
	if s.followTarget != "" {
		s.g.Camera.Follow__1(s.followTarget)
	}
}

func newCameraFollowOverrideSprite(g *Game, name string, followTarget SpriteName) *cameraFollowOverrideSprite {
	sprite := &cameraFollowOverrideSprite{followTarget: followTarget}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	return sprite
}

func TestRunSpriteCallbacksKeepsManualCameraFollowLast(t *testing.T) {
	game := &cameraFollowOverrideGame{}
	game.initShapeMgr()
	game.camera = &cameraImpl{g: &game.Game}
	game.Camera = game.camera

	spriteA := newCameraFollowOverrideSprite(&game.Game, "SpriteA", "")
	spriteB := newCameraFollowOverrideSprite(&game.Game, "SpriteB", "SpriteB")
	game.addShape(spriteOf(spriteA))
	game.addShape(spriteOf(spriteB))

	generation := game.currentBootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{Camera: &coreproject.CameraConfig{On: "SpriteA"}},
		reflect.ValueOf(game).Elem(),
		generation,
	)
	game.runBootstrapTasksFor(generation)

	followTarget, ok := game.camera.followTarget.(*SpriteImpl)
	if !ok {
		t.Fatalf("camera follow target type = %T, want *SpriteImpl", game.camera.followTarget)
	}
	if followTarget != spriteOf(spriteB) {
		t.Fatalf("camera follow target = %q, want %q", followTarget.name, spriteOf(spriteB).name)
	}
}
