package spx

import (
	"reflect"
	"slices"
	"testing"
	"time"

	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
)

type shiftedSprite struct {
	value int
	SpriteImpl
}

type spriteWithoutImplementation struct{ Sprite }

type layoutExtraBaseSprite struct {
	SpriteImpl
	owner *Game
	Extra SpriteImpl
	value int
}

func TestSpriteInitFromSelfPreservesStageCloneFamily(t *testing.T) {
	template := &SpriteImpl{}
	stage := &SpriteImpl{original: template}

	stage.InitFrom(stage)

	if got := stage.originalSprite(); got != stage {
		t.Fatalf("self initialization changed clone family from stage %p to %p", stage, got)
	}
	var descendant SpriteImpl
	descendant.InitFrom(stage)
	if got := descendant.originalSprite(); got != stage {
		t.Fatalf("descendant clone belongs to %p, want stage %p", got, stage)
	}
}

func TestSpriteTouchStartOverloadsFilterNamedTargets(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "source")
	first := newCloneLimitSprite(game, "first")
	second := newCloneLimitSprite(game, "second")
	other := newCloneLimitSprite(game, "other")
	var got []string
	source.OnTouchStart__0("first", func() { got = append(got, "single") })
	source.OnTouchStart__1("first", func(s Sprite) { got = append(got, "single:"+spriteOf(s).Name()) })
	source.OnTouchStart__2([]SpriteName{"first", "second"}, func() { got = append(got, "multiple") })
	source.OnTouchStart__3([]SpriteName{"first", "second"}, func(s Sprite) { got = append(got, "multiple:"+spriteOf(s).Name()) })
	for _, target := range []*cloneLimitSprite{other, first, second} {
		source.fireTouchStart(&target.SpriteImpl)
		gco.Update()
	}
	want := []string{"single", "single:first", "multiple", "multiple:first", "multiple", "multiple:second"}
	if !slices.Equal(got, want) {
		t.Fatalf("touch callbacks = %v, want %v", got, want)
	}
	if targets := source.physics().collisionTargets; len(targets) != 2 || !targets["first"] || !targets["second"] {
		t.Fatalf("collision targets = %v, want first and second", targets)
	}
}

func TestStopIfCurrentCoroutineOutsideCoroutine(t *testing.T) {
	if current := gco.Current(); current != nil {
		t.Fatalf("unexpected active coroutine before test: %v", current)
	}

	// Engine-driven destruction can run between coroutine turns. The scheduler
	// deliberately clears Current at that boundary, so this path must be a no-op
	// instead of dereferencing a stale or nil thread.
	(&SpriteImpl{}).stopIfCurrentCoroutine()
}

func TestStopIfCurrentCoroutineExternalCallerDoesNotStopActiveThread(t *testing.T) {
	co := setupRuntimeScheduler(t)
	sprite := &SpriteImpl{}
	started := make(chan struct{})
	release := make(chan struct{})
	thread := co.Create(sprite, func(coroutine.Thread) {
		close(started)
		<-release
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not start")
	}

	// This is an engine-side call from outside the managed coroutine. It must
	// not mistake the scheduler's Current value for the caller's coroutine.
	sprite.stopIfCurrentCoroutine()
	if thread.Stopped() {
		t.Fatal("external caller unexpectedly stopped the active coroutine")
	}
	close(release)
	co.Join(thread)
}

func TestSpriteBaseLayoutKeepsLookupAndUserFields(t *testing.T) {
	sprite := &shiftedSprite{value: 7}
	if got := spriteOf(sprite); got != &sprite.SpriteImpl {
		t.Fatalf("spriteOf = %p, want embedded SpriteImpl %p", got, &sprite.SpriteImpl)
	}

	fields := snapshotSpriteUserFields(reflect.ValueOf(sprite).Elem())
	sprite.value = 11
	sprite.spriteState.isVisible = true
	restoreSpriteUserFields(fields)
	if sprite.value != 7 {
		t.Fatalf("restored user field = %d, want 7", sprite.value)
	}
	if !sprite.spriteState.isVisible {
		t.Fatal("restoring user fields overwrote SpriteImpl")
	}
}

func TestSpriteLifecycleAndAnimationOverloadsInDebugMode(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	if spriteOf(&spriteWithoutImplementation{}) != nil {
		t.Fatal("sprite without an embedded implementation resolved unexpectedly")
	}
	for _, bound := range []bool{false, true} {
		sprite := newCloneLimitSprite(game, "dying")
		if bound {
			sprite.animation().shared.animBindings = map[string]string{StateDie: "death"}
			sprite.animation().shared.animations["death"] = &animationEntry{name: "death", config: &coreproject.AniConfig{}}
		}
		sprite.Animate__0("missing")
		sprite.Animate__1("missing", true)
		sprite.AnimateAndWait("missing")
		sprite.StopAnimation("missing")
		sprite.Die()
		if !sprite.spriteState.isDying || !sprite.isDestroyed() || game.shapeMgr.findShapeIndex(&sprite.SpriteImpl) >= 0 {
			t.Fatal("Die failed to complete lifecycle without a live animation proxy")
		}
		sprite.Destroy()
		if !sprite.isDestroyed() {
			t.Fatal("repeat destruction changed lifecycle state")
		}
	}
}

func TestSpriteLayoutKeepsFirstImplementation(t *testing.T) {
	game := setupStageLoadTestGame(t, gameTestFS{
		"sprites/extra/index.json": `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}]}`,
	})
	source := &layoutExtraBaseSprite{}
	gamer := reflect.ValueOf(&game.Game).Elem()
	prepared, err := prepareSprite(game.fs, "extra", reflect.TypeOf(source).Elem(), gamer)
	if err != nil {
		t.Fatal(err)
	}
	game.loadPreparedSprite(source, "extra", gamer, &prepared)
	if spriteOf(source) != &source.SpriteImpl {
		t.Fatal("lookup did not select the first SpriteImpl field")
	}
	source.value = 7
	state := snapshotSpriteUserFields(reflect.ValueOf(source).Elem())
	source.value, source.Extra.name = 9, "extra"
	restoreSpriteUserFields(state)
	if source.value != 7 || source.Extra.name != "extra" {
		t.Fatal("snapshot did not exclude all SpriteImpl fields")
	}
	var clone layoutExtraBaseSprite
	copySprite(reflect.ValueOf(&clone).Elem(), source)
	if !clone.IsCloned() || !clone.Extra.IsCloned() || clone.Extra.original != &source.Extra {
		t.Fatal("cloning did not invoke InitFrom for both SpriteImpl fields")
	}
}

func (*shiftedSprite) Main() {}

func (*layoutExtraBaseSprite) Main() {}
