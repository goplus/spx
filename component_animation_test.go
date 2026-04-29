package spx

import (
	"testing"

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
)

func newTestAnimationComponent() *animationComponent {
	sprite := &SpriteImpl{
		g:    &Game{},
		name: "TestSprite",
	}
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.spriteState.IsVisible = true
	sprite.spriteState.DefaultCostumeIndex = 0
	sprite.costumes = []*costume{newCostumeWithSize(1, 1)}

	anim := &animationComponent{
		componentBase: componentBase{sprite: sprite},
		shared: &sharedAnimationData{
			defaultAnimation:  "idle",
			animations:        map[SpriteAnimationName]*coreproject.AniConfig{},
			animBindings:      map[string]string{},
			animationWrappers: map[SpriteAnimationName]*animationWrapper{},
		},
		donedAnimations: make([]string, 0),
	}
	sprite.components.animation = anim
	return anim
}

func TestStopCurrentAnimStateDoesNotClearReplacement(t *testing.T) {
	anim := newTestAnimationComponent()
	oldState := &animState{Name: "walk"}
	replacement := &animState{Name: "walk"}
	anim.curAnimState = replacement

	if anim.stopCurrentAnimState(oldState) {
		t.Fatal("stopCurrentAnimState returned true for a stale state")
	}
	if anim.curAnimState != replacement {
		t.Fatal("stopCurrentAnimState cleared the replacement animation state")
	}
	if !oldState.IsCanceled {
		t.Fatal("old animation state was not canceled")
	}
	if replacement.IsCanceled {
		t.Fatal("replacement animation state was canceled")
	}
}

func TestPlayDefaultAnimIfIdleSkipsActiveDefaultAnimation(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.defaultAnimActive = true

	anim.playDefaultAnimIfIdle()

	if !anim.defaultAnimActive {
		t.Fatal("playDefaultAnimIfIdle restarted or cleared the active default animation")
	}
}

func TestCleanupTweenWithoutPlaybackKeepsActiveAnimation(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	activeAnim := &animState{Name: "wave"}
	tween := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curAnimState = activeAnim
	anim.curTweenState = tween

	anim.cleanupTween(tween, nil, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != nil {
		t.Fatal("cleanupTween did not clear the completed tween state")
	}
	if anim.curAnimState != activeAnim {
		t.Fatal("cleanupTween replaced an unrelated active animation")
	}
	if anim.sprite.costumeIndex != 1 {
		t.Fatalf("costumeIndex = %d, want active animation costume 1", anim.sprite.costumeIndex)
	}
}

func TestCleanupTweenWithoutPlaybackRestoresDefaultWhenIdle(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	tween := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curTweenState = tween

	anim.cleanupTween(tween, nil, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != nil {
		t.Fatal("cleanupTween did not clear the completed tween state")
	}
	if anim.sprite.costumeIndex != 0 {
		t.Fatalf("costumeIndex = %d, want default costume 0", anim.sprite.costumeIndex)
	}
}

func TestCleanupTweenRestoresDefaultForOwnedPlayback(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	playback := &animState{Name: StateGlide}
	tween := &animState{
		Name:    StateGlide,
		AniType: coreproject.AniTypeGlide,
	}
	anim.curAnimState = playback
	anim.curTweenState = tween

	anim.cleanupTween(tween, playback, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != nil {
		t.Fatal("cleanupTween did not clear the completed tween state")
	}
	if anim.curAnimState != nil {
		t.Fatal("cleanupTween did not clear its own animation playback state")
	}
	if anim.sprite.costumeIndex != 0 {
		t.Fatalf("costumeIndex = %d, want default costume 0", anim.sprite.costumeIndex)
	}
}
