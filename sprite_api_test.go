package spx

import (
	"reflect"
	"slices"
	"testing"
	"time"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestSetLayerPublicAPI(t *testing.T) {
	spriteType := reflect.TypeOf((*Sprite)(nil)).Elem()
	if _, ok := spriteType.MethodByName("ChangeLayer"); ok {
		t.Fatal("Sprite should not expose ChangeLayer")
	}
	if _, ok := spriteType.MethodByName("SetLayer__1"); !ok {
		t.Fatal("Sprite should keep SetLayer__1 for overload dispatch")
	}

	spriteImplType := reflect.TypeOf((*SpriteImpl)(nil))
	if _, ok := spriteImplType.MethodByName("ChangeLayer"); ok {
		t.Fatal("SpriteImpl should not expose ChangeLayer")
	}
	if _, ok := spriteImplType.MethodByName("SetLayer__1"); !ok {
		t.Fatal("SpriteImpl should keep SetLayer__1 for overload dispatch")
	}

	if XGoo_Sprite_SetLayerWith != ".SetLayerTo,.SetLayer__1" {
		t.Fatalf("XGoo_Sprite_SetLayerWith = %q, want %q", XGoo_Sprite_SetLayerWith, ".SetLayerTo,.SetLayer__1")
	}
	if XGoo_SpriteImpl_SetLayerWith != ".SetLayerTo,.SetLayer__1" {
		t.Fatalf("XGoo_SpriteImpl_SetLayerWith = %q, want %q", XGoo_SpriteImpl_SetLayerWith, ".SetLayerTo,.SetLayer__1")
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

type shiftedSprite struct {
	value int
	SpriteImpl
}

func (*shiftedSprite) Main() {}

func TestSpriteBaseLayoutKeepsLookupAndUserFields(t *testing.T) {
	sprite := &shiftedSprite{value: 7}
	if got := spriteOf(sprite); got != &sprite.SpriteImpl {
		t.Fatalf("spriteOf = %p, want embedded SpriteImpl %p", got, &sprite.SpriteImpl)
	}

	fields := snapshotSpriteUserFields(reflect.ValueOf(sprite).Elem())
	sprite.value = 11
	sprite.spriteState.IsVisible = true
	restoreSpriteUserFields(fields)
	if sprite.value != 7 {
		t.Fatalf("restored user field = %d, want 7", sprite.value)
	}
	if !sprite.spriteState.IsVisible {
		t.Fatal("restoring user fields overwrote SpriteImpl")
	}
}

type spriteWithoutImplementation struct{ Sprite }

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
		if !sprite.spriteState.IsDying || !sprite.isDestroyed() || game.shapeMgr.findShapeIndex(&sprite.SpriteImpl) >= 0 {
			t.Fatal("Die failed to complete lifecycle without a live animation proxy")
		}
		sprite.Destroy()
		if !sprite.isDestroyed() {
			t.Fatal("repeat destruction changed lifecycle state")
		}
	}
}

func TestSpritePenPublicAPIUpdatesDrawingState(t *testing.T) {
	mgr := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.SetPenHue(20)
	sprite.ChangePenHue(10)
	sprite.SetPenShade(40)
	sprite.ChangePenShade(5)
	sprite.SetPenSize(3)
	sprite.ChangePenSize(2)
	sprite.PenDown()
	sprite.PenUp()
	sprite.Stamp()
	if sprite.pen().legacyPenColor.hue != 15 || sprite.pen().legacyPenColor.shade != 45 || sprite.pen().penWidth != 5 || sprite.pen().isPenDown {
		t.Fatalf("pen API state=%+v", sprite.pen().penState)
	}
	if mgr.stampSpriteCalls != 1 {
		t.Fatalf("stamp captures=%d,want1", mgr.stampSpriteCalls)
	}
}

func TestSpriteDialogueDebugModeKeepsPresentationAndAnswer(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	mgr := installAskTestUiMgr(t)
	sprite := newCloneLimitSprite(game, "speaker")
	sprite.Show()
	sprite.SayWith("hello", 0)
	sprite.ThinkWith("thought", 0)
	sprite.QuoteMsgEx("quote", "source", 0)
	if sprite.bubble().textObj.msg != "thought" || sprite.bubble().quoteObj.message != "quote" {
		t.Fatal("debug logging changed dialogue presentation")
	}
	mgr.onAskVisible = func() { game.dialogState.AskPanel.OnCheck("answer") }
	sprite.Ask("question")
	if game.Answer() != "answer" || sprite.bubble().textObj != nil {
		t.Fatal("debug Ask did not store the answer and retire its bubble")
	}
	sprite.bubble().stopAll()
}
