package spx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/core/collision"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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

type colliderSpy struct {
	wrap.SpriteMgrImpl
	clearedShapes         []bool
	collisionCenter       mathf.Vec2
	collisionPoints       []float32
	collisionEnabled      []bool
	collisionLayer        int64
	collisionMask         int64
	collisionFilterWrites int
	triggerCenter         mathf.Vec2
	triggerPoints         []float32
	triggerEnabled        []bool
	triggerLayer          int64
	triggerMask           int64
	triggerLayerWrites    int
	triggerMaskWrites     int
	physicsWrites         int
	rectCalls             int
	rectCenter            mathf.Vec2
	rectSize              mathf.Vec2
	rectIsTrigger         bool
}

type spritePhysicsAPIManager struct {
	colliderSpy
	velocity, impulse mathf.Vec2
	gravity           float64
	floor             bool
	circleRadius      float64
	capsuleSize       mathf.Vec2
	shapeCenter       mathf.Vec2
	shapeTrigger      bool
}

type fakeEdgePhysicsMgr struct {
	stageTouching  int64
	cameraTouching int64
	stageNearest   int64
	cameraNearest  int64
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
		source.doWhenTouchStart(&target.SpriteImpl)
		scheduler.Update()
	}
	want := []string{"single", "single:first", "multiple", "multiple:first", "multiple", "multiple:second"}
	if !slices.Equal(got, want) {
		t.Fatalf("touch callbacks = %v, want %v", got, want)
	}
	if targets := source.physics().touchTargets; len(targets) != 2 || !targets["first"] || !targets["second"] {
		t.Fatalf("touch targets = %v, want first and second", targets)
	}
}

func TestStopIfCurrentCoroutineOutsideCoroutine(t *testing.T) {
	if current := scheduler.Current(); current != nil {
		t.Fatalf("unexpected active coroutine before test: %v", current)
	}

	// Engine-driven destruction can run between coroutine turns. The scheduler
	// deliberately clears Current at that boundary, so this path must be a no-op
	// instead of dereferencing a stale or nil thread.
	(&SpriteImpl{}).stopIfCurrentCoroutine()
}

func TestStopIfCurrentCoroutineExternalCallerDoesNotStopActiveThread(t *testing.T) {
	currentScheduler := setupRuntimeScheduler(t)
	sprite := &SpriteImpl{}
	started := make(chan struct{})
	release := make(chan struct{})
	thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
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
	currentScheduler.Join(thread)
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
		if !sprite.spriteState.isDying || !sprite.isDestroyed() || game.scene.findShapeIndex(&sprite.SpriteImpl) >= 0 {
			t.Fatal("Die failed to complete lifecycle without a live animation proxy")
		}
		sprite.Destroy()
		if !sprite.isDestroyed() {
			t.Fatal("repeat destruction changed lifecycle state")
		}
	}
}

func TestSpriteTypeKeepsFirstImplementation(t *testing.T) {
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
	if source.owner != &game.Game {
		t.Fatal("loading did not initialize the owner field")
	}
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

func TestMotionInputsTreatNaNAsZero(t *testing.T) {
	tests := []struct {
		name         string
		apply        func(*SpriteImpl)
		wantX, wantY float64
	}{
		{
			name:  "set position",
			apply: func(sprite *SpriteImpl) { sprite.SetXYpos(math.NaN(), math.NaN()) },
			wantX: 0,
			wantY: 0,
		},
		{
			name:  "set x",
			apply: func(sprite *SpriteImpl) { sprite.SetXpos(math.NaN()) },
			wantX: 0,
			wantY: -8,
		},
		{
			name:  "set y",
			apply: func(sprite *SpriteImpl) { sprite.SetYpos(math.NaN()) },
			wantX: 12,
			wantY: 0,
		},
		{
			name:  "change position",
			apply: func(sprite *SpriteImpl) { sprite.ChangeXYpos(math.NaN(), math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
		{
			name:  "change x",
			apply: func(sprite *SpriteImpl) { sprite.ChangeXpos(math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
		{
			name:  "change y",
			apply: func(sprite *SpriteImpl) { sprite.ChangeYpos(math.NaN()) },
			wantX: 12,
			wantY: -8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestTransformSprite(12, -8)
			tt.apply(sprite)

			if gotX, gotY := sprite.getXY(); gotX != tt.wantX || gotY != tt.wantY {
				t.Fatalf("position = (%v, %v), want (%v, %v)", gotX, gotY, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestScratchImmediateGlide(t *testing.T) {
	for _, tt := range []struct {
		name         string
		duration     float64
		x, y         float64
		wantX, wantY float64
	}{
		{name: "zero", duration: 0, x: 30, y: 40, wantX: 30, wantY: 40},
		{name: "negative", duration: -1, x: 30, y: 40, wantX: 30, wantY: 40},
		{name: "NaN duration", duration: math.NaN(), x: 30, y: 40, wantX: 30, wantY: 40},
		{name: "NaN x", x: math.NaN(), y: 40, wantX: 0, wantY: 40},
		{name: "NaN y", x: 30, y: math.NaN(), wantX: 30, wantY: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentScheduler := setupRuntimeScheduler(t)
			sprite := newTestTransformSprite(12, -8)
			finished := false
			thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
				sprite.GlideToXYpos(tt.x, tt.y, tt.duration)
				finished = true
			})
			currentScheduler.JoinFirstSlice(thread)
			if !finished || sprite.Xpos() != tt.wantX || sprite.Ypos() != tt.wantY {
				t.Fatalf("immediate glide: finished=%v position=(%v,%v)", finished, sprite.Xpos(), sprite.Ypos())
			}
			if sprite.animation().getCurTweenState() != nil {
				t.Fatal("immediate glide left an active tween")
			}
		})
	}
}

func TestSpriteGlidePositiveDurationInterpolates(t *testing.T) {
	clock.Start()
	t.Cleanup(func() { clock.Start() })
	currentScheduler := setupRuntimeScheduler(t)
	sprite := newTestTransformSprite(12, -8)
	finished := false
	clock.Update(0.5, 30)
	thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
		sprite.GlideToXYpos(30, 40, 1)
		finished = true
	})
	currentScheduler.Update()
	if finished || sprite.Xpos() != 21 || sprite.Ypos() != 16 {
		t.Fatalf("halfway glide: finished=%v position=(%v,%v)", finished, sprite.Xpos(), sprite.Ypos())
	}
	if sprite.animation().getCurTweenState() == nil {
		t.Fatal("positive-duration glide has no active tween")
	}
	clock.Update(0.5, 30)
	currentScheduler.Update()
	if sprite.Xpos() != 30 || sprite.Ypos() != 40 {
		t.Fatalf("final glide position=(%v,%v), want (30,40)", sprite.Xpos(), sprite.Ypos())
	}
	clock.Update(0, 30)
	currentScheduler.Update()
	if !finished || sprite.animation().getCurTweenState() != nil {
		t.Fatal("glide did not finish and release its tween after the final frame")
	}
	currentScheduler.Join(thread)
}

func TestSpriteDirectionToPosNormalizesAngle(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)

	tests := []struct {
		name string
		x    float64
		y    float64
		want Direction
	}{
		{name: "right", x: 10, y: 0, want: 90},
		{name: "up", x: 0, y: 10, want: 0},
		{name: "left", x: -10, y: 0, want: -90},
		{name: "down", x: 0, y: -10, want: 180},
		{name: "bottom left stays normalized", x: -10, y: -10, want: -135},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sprite.DirectionTo__4(tt.x, tt.y); !(math.Abs(got-tt.want) <= 1e-9) {
				t.Fatalf("DirectionTo__4(%v, %v) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestSpriteTurnToXYposFacesCoordinateTarget(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)

	sprite.TurnToXYpos(-10, -10, nil)

	if got := sprite.Heading(); got != -135 {
		t.Fatalf("Heading() = %v, want -135", got)
	}
}

func TestSpriteHeadingMultipleRevolutions(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.SetHeading(1080.5)
	if got := sprite.Heading(); got != 0.5 {
		t.Fatalf("SetHeading: got %v, want 0.5", got)
	}
	sprite.ChangeHeading(-720.5)
	if got := sprite.Heading(); got != 0 {
		t.Fatalf("ChangeHeading: got %v, want 0", got)
	}
	version := sprite.spriteState.dirtyVersion
	sprite.SetHeading(1080)
	if sprite.spriteState.dirtyVersion != version {
		t.Fatal("equivalent heading dirtied the sprite proxy")
	}
}

func TestSpriteHeadingIgnoresNonFiniteInputs(t *testing.T) {
	for _, input := range []struct {
		name  string
		value float64
	}{
		{"NaN", math.NaN()},
		{"positive infinity", math.Inf(1)},
		{"negative infinity", math.Inf(-1)},
	} {
		for _, action := range []struct {
			name  string
			apply func(*SpriteImpl, Direction)
		}{
			{"set", (*SpriteImpl).SetHeading},
			{"change", (*SpriteImpl).ChangeHeading},
		} {
			t.Run(action.name+"/"+input.name, func(t *testing.T) {
				sprite := newTestTransformSprite(0, 0)
				sprite.SetHeading(45)
				dirty, visual := sprite.spriteState.dirtyVersion, sprite.spriteState.visualVersion

				action.apply(sprite, input.value)

				if sprite.Heading() != 45 || sprite.spriteState.dirtyVersion != dirty ||
					sprite.spriteState.visualVersion != visual {
					t.Fatalf("invalid input changed heading or versions: heading=%v dirty=%v visual=%v",
						sprite.Heading(), sprite.spriteState.dirtyVersion, sprite.spriteState.visualVersion)
				}
				sprite.ChangeHeading(405)
				if sprite.Heading() != 90 {
					t.Fatalf("heading after valid change = %v, want 90", sprite.Heading())
				}
			})
		}
	}
}

func TestSpriteTurnIgnoresNonFiniteInputs(t *testing.T) {
	for _, animation := range []string{"without animation", "with animation"} {
		for _, input := range []struct {
			name  string
			value float64
		}{
			{"NaN", math.NaN()},
			{"positive infinity", math.Inf(1)},
			{"negative infinity", math.Inf(-1)},
		} {
			for _, action := range []struct {
				name  string
				apply func(*SpriteImpl, Direction)
			}{
				{"turn", (*SpriteImpl).Turn__0},
				{"turn to", (*SpriteImpl).TurnTo__2},
			} {
				t.Run(animation+"/"+action.name+"/"+input.name, func(t *testing.T) {
					currentScheduler := setupRuntimeScheduler(t)
					sprite := newTestTransformSprite(0, 0)
					sprite.SetHeading(45)
					version := sprite.spriteState.dirtyVersion
					if animation == "with animation" {
						sprite.animation().shared.animations[StateTurn] = &animationEntry{
							name: StateTurn, config: &coreproject.AniConfig{TurnToDuration: 1},
						}
					}
					finished := false
					thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
						action.apply(sprite, input.value)
						finished = true
					})
					currentScheduler.JoinFirstSlice(thread)
					if !finished || sprite.Heading() != 45 || sprite.spriteState.dirtyVersion != version ||
						sprite.animation().getCurTweenState() != nil {
						t.Fatalf("invalid turn: finished=%v heading=%v dirty=%v tween=%v",
							finished, sprite.Heading(), sprite.spriteState.dirtyVersion, sprite.animation().getCurTweenState())
					}
					currentScheduler.Join(thread)
				})
			}
		}
	}
}

func TestSpriteTurnToDirMultipleRevolutionsUsesShortestAnimation(t *testing.T) {
	t.Cleanup(func() { clock.Start() })
	currentScheduler := setupRuntimeScheduler(t)
	sprite := newTestTransformSprite(0, 0)
	sprite.SetHeading(-10)
	sprite.animation().shared.animations[StateTurn] = &animationEntry{
		name:   StateTurn,
		config: &coreproject.AniConfig{TurnToDuration: 360},
	}

	// At one degree per second, the shortest turn passes through zero after
	// ten seconds and reaches the target after twenty.
	clock.Update(10, 30)
	returned := false
	thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
		sprite.TurnToDir(-710, nil)
		returned = true
	})
	currentScheduler.Update()
	if got := sprite.Heading(); got != 0 {
		t.Fatalf("halfway heading = %v, want 0", got)
	}
	clock.Update(10, 30)
	currentScheduler.Update()
	if got := sprite.Heading(); got != 10 {
		t.Fatalf("final heading = %v, want 10", got)
	}
	clock.Update(0, 30)
	currentScheduler.Update()
	if !returned {
		t.Fatal("TurnToDir did not return after its final animation frame")
	}
	currentScheduler.Join(thread)
}

func TestMotionOptions(t *testing.T) {
	tests := []struct {
		name          string
		opts          *MotionOptions
		wantSpeed     Speed
		wantAnimation SpriteAnimationName
	}{
		{
			name:      "nil uses defaults",
			opts:      nil,
			wantSpeed: 1,
		},
		{
			name: "zero speed keeps default and animation",
			opts: &MotionOptions{
				Animation: "walk",
			},
			wantSpeed:     1,
			wantAnimation: "walk",
		},
		{
			name: "positive speed overrides default",
			opts: &MotionOptions{
				Speed:     2.5,
				Animation: "run",
			},
			wantSpeed:     2.5,
			wantAnimation: "run",
		},
		{
			name: "negative speed falls back to default",
			opts: &MotionOptions{
				Speed:     -3,
				Animation: "run",
			},
			wantSpeed:     1,
			wantAnimation: "run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpeed, gotAnimation := motionOptions(tt.opts)
			if gotSpeed != tt.wantSpeed {
				t.Fatalf("speed = %v, want %v", gotSpeed, tt.wantSpeed)
			}
			if gotAnimation != tt.wantAnimation {
				t.Fatalf("animation = %q, want %q", gotAnimation, tt.wantAnimation)
			}
		})
	}
}

func TestSpriteStepToRandomUsesRandomPositionTarget(t *testing.T) {
	sprite := newTestTransformSprite(1000, 1000)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360

	SetRandomSeed(1)
	defer ResetRandomSeed()
	sprite.StepTo__c(Random)

	gotX, gotY := sprite.getXY()
	if gotX == 1000 && gotY == 1000 {
		t.Fatal("StepTo__c(Random) did not move the sprite")
	}

	minX := float64(-(sprite.g.displayState.WorldWidth >> 1))
	maxX := float64(sprite.g.displayState.WorldWidth - (sprite.g.displayState.WorldWidth >> 1) - 1)
	minY := float64((sprite.g.displayState.WorldHeight >> 1) - (sprite.g.displayState.WorldHeight - 1))
	maxY := float64(sprite.g.displayState.WorldHeight >> 1)
	if gotX < minX || gotX > maxX || gotY < minY || gotY > maxY {
		t.Fatalf(
			"StepTo__c(Random) moved to (%v, %v), want x in [%v, %v], y in [%v, %v]",
			gotX,
			gotY,
			minX,
			maxX,
			minY,
			maxY,
		)
	}
}

func TestFixWorldRange(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		scale          float64
		heading        Direction
		startX, startY float64
		x, y           float64
		wantX, wantY   float64
	}{
		{
			name:  "oversized stays centered",
			width: 480, height: 360, scale: 3,
			startY: 45, y: 45,
			wantY: 45,
		},
		{
			name:  "normal sprite is fenced",
			width: 100, height: 80, scale: 1, heading: 90,
			x: 500, y: 300,
			wantX: 275, wantY: 205,
		},
		{
			name:  "oversized is fenced when fully outside",
			width: 480, height: 360, scale: 3,
			startY: 45, x: 1000, y: 45, heading: 90,
			wantX: 945, wantY: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestTransformSprite(tt.startX, tt.startY)
			sprite.g.displayState.WorldWidth = 480
			sprite.g.displayState.WorldHeight = 360
			sprite.g.displayState.MinWorldX = -240
			sprite.g.displayState.MinWorldY = -180
			sprite.costumes = []*costume{{
				atlas: true,
				frame: costumeFrame{
					Width:            tt.width,
					Height:           tt.height,
					BitmapResolution: 1,
					Center:           mathf.NewVec2(float64(tt.width)/2, float64(tt.height)/2),
				},
			}}
			sprite.costumeIndex = 0
			sprite.runtimeState.scale = tt.scale
			sprite.transform().direction = tt.heading

			x, y := sprite.transform().fixWorldRange(tt.x, tt.y)
			if x != tt.wantX || y != tt.wantY {
				t.Fatalf("fixWorldRange(%v, %v) = (%v, %v), want (%v, %v)", tt.x, tt.y, x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestFixWorldRangeUsesRenderedCostumeBoundsInsteadOfAutoTrigger(t *testing.T) {
	sprite := newTestTransformSprite(-186, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.visualObject.costumes = []*costume{{
		atlas: true,
		frame: costumeFrame{
			Width:            55,
			Height:           54,
			BitmapResolution: 2,
			Center:           mathf.NewVec2(52, 89),
		},
	}}
	sprite.visualObject.costumeIndex = 0
	sprite.runtimeState.scale = 1
	sprite.runtimeState.proxy = &engine.Sprite{}
	sprite.transform().direction = 90
	sprite.physics().triggerCollider.Type = collision.ColliderAuto
	sprite.physics().triggerCollider.Pivot = mathf.NewVec2(0.25, 0)
	sprite.physics().triggerCollider.Params = []float64{29.5, 29}

	gotX, gotY := sprite.transform().fixWorldRange(1000, 0)
	if gotX != 253 || gotY != 0 {
		t.Fatalf("fixWorldRange(1000, 0) = (%v, %v), want (253, 0)", gotX, gotY)
	}
}

func TestFenceBoundsIncludesCostumeRotation(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.visualObject.costumes = []*costume{{
		atlas: true,
		frame: costumeFrame{
			Width:            100,
			Height:           40,
			BitmapResolution: 1,
			Center:           mathf.NewVec2(50, 20),
		},
	}}
	sprite.visualObject.costumeIndex = 0
	sprite.runtimeState.scale = 1
	sprite.transform().direction = 0

	got := sprite.fenceBounds()
	if got == nil {
		t.Fatal("fenceBounds() = nil")
	}
	if !(math.Abs(got.Position.X+20) <= 1e-9 && math.Abs(got.Position.Y+50) <= 1e-9 &&
		math.Abs(got.Size.X-40) <= 1e-9 && math.Abs(got.Size.Y-100) <= 1e-9) {
		t.Fatalf("fenceBounds() = %+v, want position (-20, -50), size (40, 100)", *got)
	}
}

func TestClampSpriteScaleToMaxScale(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.visualObject.costumes = []*costume{{
		atlas: true,
		frame: costumeFrame{
			Width:            480,
			Height:           360,
			BitmapResolution: 1,
			Center:           mathf.NewVec2(240, 180),
		},
	}}
	sprite.visualObject.costumeIndex = 0

	got := sprite.transform().clampSpriteScale(3)

	if got != 1.5 {
		t.Fatalf("clampSpriteScale(3) = %v, want 1.5", got)
	}
}

func TestFractionalCostumeFenceUsesLogicalGeometry(t *testing.T) {
	sprite := newTestTransformSprite(-393, 5)
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.g.displayState.MinWorldX = -240
	sprite.g.displayState.MinWorldY = -180
	sprite.visualObject.costumes = []*costume{newCostume(&coreproject.CostumeConfig{
		ImageWidth: 247.24325, ImageHeight: 476.42829,
		X: 76.0323911558041, Y: 208.79720845345383,
		BitmapResolution: 1,
	})}
	sprite.visualObject.costumeIndex = 0
	sprite.runtimeState.scale = 1
	sprite.transform().direction = 90
	sprite.transform().changeX(-5)
	if got := sprite.Xpos(); got != -396 {
		t.Fatalf("fenced X = %v, want -396", got)
	}
}

func TestNormalizeDirection(t *testing.T) {
	for _, tt := range []struct {
		input, want float64
	}{
		{0, 0}, {90, 90}, {-90, -90}, {180, 180}, {-180, 180},
		{181, -179}, {-181, 179}, {360, 0}, {-360, 0},
		{540, 180}, {-540, 180}, {1080, 0}, {-1080, 0},
		{0.5, 0.5}, {-0.5, -0.5}, {720.5, 0.5}, {-720.5, -0.5},
		{1080.5, 0.5}, {-1080.5, -0.5},
		{math.MaxFloat64, 128}, {-math.MaxFloat64, -128},
		{math.Inf(1), math.Inf(1)}, {math.Inf(-1), math.Inf(-1)},
	} {
		got := normalizeDirection(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDirection(%v) = %v, want %v", tt.input, got, tt.want)
		}
		if again := normalizeDirection(got); again != got {
			t.Errorf("normalization is not idempotent: %v -> %v -> %v", tt.input, got, again)
		}
	}
	if got := normalizeDirection(math.NaN()); !math.IsNaN(got) {
		t.Errorf("normalizeDirection(NaN) = %v, want NaN", got)
	}
}

func TestNormalizeDirectionPeriodicity(t *testing.T) {
	for _, angle := range []float64{-180, -90.5, 0, 90.5, 180} {
		want := normalizeDirection(angle)
		for turns := -10; turns <= 10; turns++ {
			input := angle + float64(turns)*fullCircleDegrees
			if got := normalizeDirection(input); got != want {
				t.Errorf("normalizeDirection(%v) = %v, want %v", input, got, want)
			}
		}
	}
}

func TestNormalizeAngleRangePreservesShortestPathAndHalfTurnDirection(t *testing.T) {
	for _, tt := range []struct {
		from, to, wantDelta float64
	}{
		{-10, -710, 20}, {10, 710, -20},
		{-730, 730, 20}, {730, -730, -20},
		{90, -90, 180}, {-90, 90, -180},
		{0, -180, 180}, {180, 0, -180},
		{-180, 540, 0},
		{0, 1e-14, 1e-14},
		{math.MaxFloat64, -math.MaxFloat64, 104},
		{-math.MaxFloat64, math.MaxFloat64, -104},
	} {
		from, to := normalizeAngleRange(tt.from, tt.to)
		if delta := to - from; delta != tt.wantDelta {
			t.Errorf("normalizeAngleRange(%v, %v) turns %v degrees, want %v", tt.from, tt.to, delta, tt.wantDelta)
		}
	}
	for _, angle := range []float64{math.Inf(-1), math.Inf(1), math.NaN()} {
		from, to := normalizeAngleRange(angle, 90)
		if !math.IsNaN(from) || to != 90 {
			t.Errorf("normalizeAngleRange(%v, 90) = (%v, %v), want (NaN, 90)", angle, from, to)
		}
		from, to = normalizeAngleRange(90, angle)
		if from != 90 || !math.IsNaN(to) {
			t.Errorf("normalizeAngleRange(90, %v) = (%v, %v), want (90, NaN)", angle, from, to)
		}
	}
}

func TestSpriteMotionIgnoresMissingTarget(t *testing.T) {
	for _, action := range []struct {
		name string
		run  func(*cloneLimitSprite)
	}{
		{"step", func(sprite *cloneLimitSprite) { sprite.StepTo__1("missing") }},
		{"glide zero duration", func(sprite *cloneLimitSprite) { sprite.Glide__1("missing", 0) }},
		{"glide nonzero duration", func(sprite *cloneLimitSprite) { sprite.Glide__1("missing", 60) }},
		{"turn", func(sprite *cloneLimitSprite) { sprite.TurnTo__1("missing") }},
	} {
		t.Run(action.name, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			sprite := newCloneLimitSprite(game, "source")
			sprite.transform().x, sprite.transform().y, sprite.transform().direction = 30, 40, 60
			dirtyVersion := sprite.spriteState.dirtyVersion
			finished := false
			thread := scheduler.Create(sprite, func(coroutine.Thread) {
				action.run(sprite)
				finished = true
			})
			scheduler.JoinFirstSlice(thread)

			if !finished || sprite.Xpos() != 30 || sprite.Ypos() != 40 || sprite.Heading() != 60 || sprite.spriteState.dirtyVersion != dirtyVersion {
				t.Fatalf("missing target altered sprite: finished=%v position=(%v,%v) heading=%v dirty=%v", finished, sprite.Xpos(), sprite.Ypos(), sprite.Heading(), sprite.spriteState.dirtyVersion)
			}
		})
	}
}

func TestSpriteTurnWithoutAnimationFinishesImmediately(t *testing.T) {
	for _, mode := range []struct {
		name  string
		debug bool
	}{
		{"normal", false},
		{"debug instructions", true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, action := range []struct {
				name string
				run  func(*cloneLimitSprite)
			}{
				{"named target", func(sprite *cloneLimitSprite) { sprite.TurnToTarget("target", nil) }},
				{"coordinates", func(sprite *cloneLimitSprite) { sprite.TurnToXYpos(-10, -10, nil) }},
				{"direction", func(sprite *cloneLimitSprite) { sprite.TurnToDir(945, nil) }},
			} {
				t.Run(action.name, func(t *testing.T) {
					game := setupCloneLimitGame(t)
					originalDebug := game.debugState.DebugInstr
					game.debugState.DebugInstr = mode.debug
					t.Cleanup(func() { game.debugState.DebugInstr = originalDebug })
					sprite := newCloneLimitSprite(game, "source")
					sprite.SetHeading(60)
					target := newCloneLimitSprite(game, "target")
					target.SetXYpos(-10, -10)
					wantDirty := sprite.spriteState.dirtyVersion + 1

					// Both the initial turn and the unchanged heading must return
					// without waiting; only the initial turn publishes a change.
					for attempt := 0; attempt < 2; attempt++ {
						finished := false
						thread := scheduler.Create(sprite, func(coroutine.Thread) {
							action.run(sprite)
							finished = true
						})
						scheduler.JoinFirstSlice(thread)
						if !finished {
							t.Fatalf("turn %d yielded without an animation", attempt+1)
						}
						scheduler.Join(thread)
						if sprite.Heading() != -135 || sprite.spriteState.dirtyVersion != wantDirty {
							t.Fatalf("turn %d: heading=%v dirty=%v, want -135 and %v", attempt+1, sprite.Heading(), sprite.spriteState.dirtyVersion, wantDirty)
						}
						if sprite.Xpos() != 0 || sprite.Ypos() != 0 {
							t.Fatalf("turn moved sprite to (%v, %v)", sprite.Xpos(), sprite.Ypos())
						}
					}
				})
			}
		})
	}
}

func TestSpriteDistanceToMissingTarget(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	if got := sprite.DistanceTo__1("missing"); got != 10000 {
		t.Fatalf("DistanceTo__1(missing) = %v, want 10000", got)
	}
}

func TestSpriteNamedTargetIgnoresClones(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	sprite.transform().x, sprite.transform().y = 30, 40
	target := newCloneLimitSprite(game, "target")
	clone := createRuntimeClone(&target.SpriteImpl)
	clone.transform().x = 300

	if got := sprite.DistanceTo__1("target"); got != 50 {
		t.Fatalf("DistanceTo__1(target) = %v, want 50", got)
	}
}

func TestSpriteNilTargetIsMissing(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	var target *cloneLimitSprite
	if got := sprite.DistanceTo__0(target); got != 10000 {
		t.Fatalf("DistanceTo__0(nil) = %v, want 10000", got)
	}
}

func TestSpriteDirectionToMissingTargetKeepsHeading(t *testing.T) {
	game := setupCloneLimitGame(t)
	sprite := newCloneLimitSprite(game, "source")
	sprite.transform().direction = 60
	if got := sprite.DirectionTo__1("missing"); got != 60 {
		t.Fatalf("DirectionTo__1(missing) = %v, want 60", got)
	}
}

func TestSpriteMotionOverloadsKeepTargetAndOptionSemantics(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	game.displayState.WorldWidth, game.displayState.WorldHeight = 1, 1
	target := newCloneLimitSprite(game, "target")
	sprite := newCloneLimitSprite(game, "source")
	// A one-pixel world makes Random deterministic without changing the RNG.
	// Remove costumes so movement is not fenced by that small test world.
	sprite.costumes = nil
	options := &MotionOptions{Speed: 2, Animation: "missing"}
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"step sprite", func() { sprite.StepTo__0(target) }},
		{"step mouse", func() { sprite.StepTo__2(Mouse) }},
		{"step xy", func() { sprite.StepTo__3(0, 0) }},
		{"step sprite speed", func() { sprite.StepTo__4(target, 2) }},
		{"step name speed", func() { sprite.StepTo__5("target", 2) }},
		{"step mouse speed", func() { sprite.StepTo__6(Mouse, 2) }},
		{"step xy speed", func() { sprite.StepTo__7(0, 0, 2) }},
		{"step sprite animation", func() { sprite.StepTo__8(target, 2, "missing") }},
		{"step name animation", func() { sprite.StepTo__9("target", 2, "missing") }},
		{"step mouse animation", func() { sprite.StepTo__a(Mouse, 2, "missing") }},
		{"step xy animation", func() { sprite.StepTo__b(0, 0, 2, "missing") }},
		{"step random speed", func() { sprite.StepTo__d(Random, 2) }},
		{"step random animation", func() { sprite.StepTo__e(Random, 2, "missing") }},
		{"step target options", func() { sprite.StepToTarget(target, options) }},
		{"step xy options", func() { sprite.StepToXYpos(0, 0, options) }},
		{"glide sprite", func() { sprite.Glide__0(target, 0) }},
		{"glide mouse", func() { sprite.Glide__2(Mouse, 0) }},
		{"glide random", func() { sprite.Glide__3(Random, 0) }},
		{"glide xy", func() { sprite.Glide__4(0, 0, 0) }},
		{"glide target", func() { sprite.GlideToTarget(target, 0) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite.SetXYpos(-10, -20)
			tt.run()
			if sprite.Xpos() != 0 || sprite.Ypos() != 0 {
				t.Fatalf("position = (%v, %v), want origin", sprite.Xpos(), sprite.Ypos())
			}
		})
	}
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"move float", func() { sprite.Move__0(2) }},
		{"move integer", func() { sprite.Move__1(2) }},
		{"step speed", func() { sprite.Step__1(2, 3) }},
		{"step animation", func() { sprite.Step__2(2, 3, "missing") }},
		{"step options", func() { sprite.StepWith(2, options) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite.SetXYpos(0, 0)
			sprite.SetHeading(90)
			tt.run()
			if !(math.Abs(sprite.Xpos()-2) <= 1e-12 && math.Abs(sprite.Ypos()) <= 1e-12) {
				t.Fatalf("forward movement = (%v, %v), want (2, 0)", sprite.Xpos(), sprite.Ypos())
			}
		})
	}
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"turn", func() { sprite.Turn__0(90) }},
		{"turn speed", func() { sprite.Turn__1(90, 2) }},
		{"turn animation", func() { sprite.Turn__2(90, 2, "missing") }},
		{"turn options", func() { sprite.TurnWith(90, options) }},
		{"turn sprite", func() { sprite.TurnTo__0(target) }},
		{"turn direction", func() { sprite.TurnTo__2(90) }},
		{"turn mouse", func() { sprite.TurnTo__3(Mouse) }},
		{"turn sprite speed", func() { sprite.TurnTo__4(target, 2) }},
		{"turn name speed", func() { sprite.TurnTo__5("target", 2) }},
		{"turn direction speed", func() { sprite.TurnTo__6(90, 2) }},
		{"turn mouse speed", func() { sprite.TurnTo__7(Mouse, 2) }},
		{"turn sprite animation", func() { sprite.TurnTo__8(target, 2, "missing") }},
		{"turn name animation", func() { sprite.TurnTo__9("target", 2, "missing") }},
		{"turn direction animation", func() { sprite.TurnTo__a(90, 2, "missing") }},
		{"turn mouse animation", func() { sprite.TurnTo__b(Mouse, 2, "missing") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite.SetXYpos(-10, 0)
			sprite.SetHeading(0)
			tt.run()
			if sprite.Heading() != 90 {
				t.Fatalf("heading = %v, want 90", sprite.Heading())
			}
		})
	}
	sprite.SetXYpos(0, -10)
	for name, got := range map[string]float64{"mouse": sprite.DistanceTo__2(Mouse), "random": sprite.DistanceTo__3(Random), "target": sprite.DistanceToWith(target)} {
		if got != 10 {
			t.Errorf("%s distance = %v, want 10", name, got)
		}
	}
	for name, got := range map[string]float64{"sprite": sprite.DirectionTo__0(target), "mouse": sprite.DirectionTo__2(Mouse), "random": sprite.DirectionTo__3(Random), "target": sprite.DirectionToWith(target)} {
		if !(math.Abs(got) <= 1e-9) {
			t.Errorf("%s direction = %v, want 0", name, got)
		}
	}
	sprite.SetSize(2)
	sprite.ChangeSize(3)
	if sprite.Size() != 5 {
		t.Fatalf("size = %v, want 5", sprite.Size())
	}
}

func TestTransformComponentScaleFallbacksAndTinyStage(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	transform := sprite.transform()
	if got := transform.clampSpriteScale(3); got != 3 {
		t.Fatalf("unconfigured sprite scale = %v, want 3", got)
	}
	sprite.costumes = []*costume{newCostumeWithSize(0, 0)}
	if got := transform.clampSpriteScale(3); got != 3 {
		t.Fatalf("empty costume scale = %v, want 3", got)
	}
	sprite.costumes = []*costume{newCostumeWithSize(10, 10)}
	sprite.g.displayState.WorldWidth, sprite.g.displayState.WorldHeight = 1, 1
	if got := transform.clampSpriteScale(3); got != 0.5 {
		t.Fatalf("tiny stage scale = %v, want minimum visible scale 0.5", got)
	}
	transform.setSize(-2)
	if sprite.runtimeState.scale != 0.5 {
		t.Fatalf("negative size = %v, want clamped visible size 0.5", sprite.runtimeState.scale)
	}
}

func TestTransformComponentKeepsBottomFenceAndHeadingAtOwnPosition(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.runtimeState.scale = 1
	sprite.costumes = []*costume{newCostumeWithSize(20, 20)}
	sprite.g.displayState.WorldWidth, sprite.g.displayState.WorldHeight = 100, 100
	sprite.g.displayState.MinWorldX, sprite.g.displayState.MinWorldY = -50, -50
	transform := sprite.transform()
	if x, y := transform.fixWorldRange(0, -100); x != 0 || y != -50 {
		t.Fatalf("bottom fenced position = (%v, %v), want (0, -50)", x, y)
	}
	transform.direction = 35
	if got := transform.headingToPosition(0, 0); got != 35 {
		t.Fatalf("direction to current position = %v, want 35", got)
	}
	sprite.g.inputMgr.mousePos = mathf.NewVec2(0, 10)
	if got := transform.directionToTarget(Mouse); !(math.Abs(got) <= 1e-9) {
		t.Fatalf("direction to mouse above sprite = %v, want 0", got)
	}
	transform.bounceOffEdge(edgeAreaStage)
	if transform.direction != 35 {
		t.Fatal("sprite away from edges changed its heading")
	}
}

func TestTransformComponentStepUsesConfiguredAnimationDuration(t *testing.T) {
	t.Cleanup(clock.ClearTimers)
	currentScheduler := setupRuntimeScheduler(t)
	clock.Start()
	sprite := newTestTransformSprite(0, 0)
	animation := &coreproject.AniConfig{StepDuration: 0.001}
	sprite.animation().shared.animations["walk"] = &animationEntry{name: "walk", config: animation}
	done := false
	thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
		sprite.transform().stepToPos(10, 0, 1, "walk")
		done = true
	})
	currentScheduler.JoinFirstSlice(thread)
	if done {
		t.Fatal("configured step animation completed before advancing a frame")
	}
	for range 2 {
		clock.Update(1.0/30, 30)
		currentScheduler.Update()
	}
	if x, y := sprite.getXY(); !done || x != 10 || y != 0 || sprite.animation().getCurTweenState() != nil {
		t.Fatalf("step completion = %v, position=(%v,%v), tween=%v", done, x, y, sprite.animation().getCurTweenState())
	}
	currentScheduler.Join(thread)
}

func TestTransformComponentDebugVisualOperations(t *testing.T) {
	t.Cleanup(clock.ClearTimers)
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	sprite := newCloneLimitSprite(game, "moving")
	sprite.costumes = nil
	transform := sprite.transform()
	transform.setRotationStyle(LeftRight)
	transform.direction = 45
	transform.bounceOffEdge(edgeAreaStage)
	if transform.rotationStyle != LeftRight || transform.direction != 45 {
		t.Fatal("unbound sprite lost its rotation style or heading")
	}
	clock.Start()
	done := false
	thread := scheduler.Create(sprite, func(coroutine.Thread) {
		transform.glide(5, 7, 0.01)
		done = true
	})
	scheduler.JoinFirstSlice(thread)
	for range 2 {
		clock.Update(1.0/30, 30)
		scheduler.Update()
	}
	if x, y := sprite.getXY(); !done || x != 5 || y != 7 {
		t.Fatalf("debug glide completed=%v, position=(%v,%v), want (5,7)", done, x, y)
	}
	scheduler.Join(thread)
}

func TestPhysicsComponentCloneCopiesFilters(t *testing.T) {
	sprite, mgr := newColliderTestSprite(t)
	sprite.SetCollisionLayer(2)
	sprite.SetCollisionMask(4)
	sprite.SetTriggerLayer(8)
	sprite.SetTriggerMask(16)

	if mgr.collisionLayer != 2 || mgr.collisionMask != 4 || mgr.triggerLayer != 8 || mgr.triggerMask != 16 {
		t.Fatalf("proxy layers = (%d, %d, %d, %d)", mgr.collisionLayer, mgr.collisionMask, mgr.triggerLayer, mgr.triggerMask)
	}

	clone := sprite.physics().cloneFor(&SpriteImpl{g: sprite.g})
	if clone.bodyCollider.Layer != 2 || clone.bodyCollider.Mask != 4 || clone.triggerCollider.Layer != 8 || clone.triggerCollider.Mask != 16 {
		t.Fatalf("clone layers = (%d, %d, %d, %d)", clone.bodyCollider.Layer, clone.bodyCollider.Mask, clone.triggerCollider.Layer, clone.triggerCollider.Mask)
	}

	mgr.collisionLayer, mgr.collisionMask, mgr.triggerLayer, mgr.triggerMask = 0, 0, 0, 0
	clone.bodyCollider.Type = collision.ColliderNone
	clone.triggerCollider.Type = collision.ColliderNone
	clone.initializeProxy(&engine.Sprite{Sprite: pkgengine.Sprite{Id: 2}})
	if mgr.collisionLayer != 2 || mgr.collisionMask != 4 || mgr.triggerLayer != 8 || mgr.triggerMask != 16 {
		t.Fatalf("clone proxy filters = (%d, %d, %d, %d)", mgr.collisionLayer, mgr.collisionMask, mgr.triggerLayer, mgr.triggerMask)
	}
}

func TestTouchSubscriptionsRefreshOnlyTriggerFilters(t *testing.T) {
	for _, byShape := range []bool{false, true} {
		t.Run(fmt.Sprintf("shape=%v", byShape), func(t *testing.T) {
			setupRuntimeScheduler(t)
			sprite, mgr := newColliderTestSprite(t)
			sprite.name = "A"
			game := sprite.g
			game.autoTouchLayers = true
			game.touchByShape = byShape
			game.touchFilters = map[string]*touchFilter{
				"A": {Layer: 1},
				"B": {Layer: 2},
				"C": {Layer: 4},
			}
			game.touchEntries = []touchEntry{{sprite: sprite, filter: game.touchFilters["A"]}}
			sprite.scriptEventBindings.bind(&game.scriptEvents, sprite)
			sprite.physics().initializeProxy(sprite.runtimeState.proxy)
			if mgr.collisionLayer != 0 || mgr.collisionMask != 0 {
				t.Fatal("automatic initialization must clear body filters")
			}
			sprite.SetPhysicsMode(KinematicPhysics)
			if err := sprite.SetColliderShape(false, RectCollider, []float64{4, 5}); err != nil {
				t.Fatal(err)
			}
			sprite.SetCollisionLayer(8)
			sprite.SetCollisionMask(16)
			sprite.SetCollisionEnabled(false)
			sprite.SetTriggerEnabled(false)
			sprite.SetColliderPivot(true, 100, 200)
			sprite.markAutoPhysicsShapesDirty()
			physics := sprite.physics()
			wantCollision, wantTrigger := physics.bodyCollider, physics.triggerCollider
			wantTrigger.Mask = 6
			wantBackend := *mgr
			wantBackend.triggerMask = 6
			wantBackend.triggerLayerWrites++
			wantBackend.triggerMaskWrites++

			sprite.OnTouchStart__3([]SpriteName{"B", "B", "C"}, func(Sprite) {})
			if !reflect.DeepEqual(*mgr, wantBackend) {
				t.Fatalf("subscription must update only trigger filters once: got %+v, want %+v", *mgr, wantBackend)
			}
			if !reflect.DeepEqual(physics.bodyCollider, wantCollision) || !reflect.DeepEqual(physics.triggerCollider, wantTrigger) || !physics.autoShapesDirty {
				t.Fatalf("subscription changed shape state: collision=%v trigger=%v dirty=%v", physics.bodyCollider, physics.triggerCollider, physics.autoShapesDirty)
			}
			sprite.OnTouchStart__3([]SpriteName{"B", "C"}, func(Sprite) {})
			sprite.OnTouchStart__3(nil, func(Sprite) {})
			if !reflect.DeepEqual(*mgr, wantBackend) || len(physics.touchTargets) != 2 {
				t.Fatal("unchanged subscriptions refreshed filters")
			}
		})
	}
}

func TestTouchSubscriptionsPreserveManualFilters(t *testing.T) {
	for _, byShape := range []bool{false, true} {
		t.Run(fmt.Sprintf("shape=%v", byShape), func(t *testing.T) {
			setupRuntimeScheduler(t)
			sprite, mgr := newColliderTestSprite(t)
			sprite.g.touchByShape = byShape
			sprite.scriptEventBindings.bind(&sprite.g.scriptEvents, sprite)
			sprite.SetCollisionLayer(2)
			sprite.SetCollisionMask(4)
			sprite.SetTriggerLayer(8)
			sprite.SetTriggerMask(16)
			physics := sprite.physics()
			wantCollision, wantTrigger := physics.bodyCollider, physics.triggerCollider
			wantBackend := *mgr

			sprite.OnTouchStart__3([]SpriteName{"B", "C"}, func(Sprite) {})
			if !reflect.DeepEqual(*mgr, wantBackend) ||
				!reflect.DeepEqual(physics.bodyCollider, wantCollision) || !reflect.DeepEqual(physics.triggerCollider, wantTrigger) {
				t.Fatal("touch subscription changed manual filters")
			}
			if len(physics.touchTargets) != 2 || !physics.touchTargets["B"] || !physics.touchTargets["C"] {
				t.Fatalf("touch targets = %v, want B and C", physics.touchTargets)
			}
		})
	}
}

func TestPixelSensingNeedsNoTriggerShape(t *testing.T) {
	for _, byShape := range []bool{false, true} {
		sprite, mgr := newColliderTestSprite(t)
		sprite.g.touchByShape = byShape
		for _, trigger := range []bool{false, true} {
			cfg := sprite.physics().colliderConfig(trigger)
			cfg.Type = collision.ColliderNone
			cfg.initializeShape(sprite.runtimeState.proxy, trigger, sprite)
			if err := sprite.SetColliderShape(trigger, collision.ColliderNone, nil); err != nil {
				t.Fatal(err)
			}
		}
		if want := []bool{false, false, true, true}; !slices.Equal(mgr.clearedShapes, want) {
			t.Fatalf("cleared shapes = %v, want %v", mgr.clearedShapes, want)
		}
		if !slices.Equal(mgr.collisionEnabled, []bool{false, false}) {
			t.Fatalf("body collider enabled = %v", mgr.collisionEnabled)
		}
		if want := []bool{!byShape, !byShape}; !slices.Equal(mgr.triggerEnabled, want) {
			t.Fatalf("touchByShape=%v: trigger enabled = %v, want %v", byShape, mgr.triggerEnabled, want)
		}
		sprite.SetTriggerEnabled(false)
		if mgr.triggerEnabled[len(mgr.triggerEnabled)-1] {
			t.Fatal("explicit event disabling was ignored")
		}
	}
}

func TestSetPolygonColliderSyncsScaledShape(t *testing.T) {
	sprite, mgr := newColliderTestSprite(t)
	points := []float64{-1, -2, 3, -4, 5, 6}

	sprite.physics().bodyCollider.Pivot = mathf.NewVec2(2, 3)
	if err := sprite.SetColliderShape(false, PolygonCollider, points); err != nil {
		t.Fatalf("SetColliderShape(collision) failed: %v", err)
	}
	sprite.physics().triggerCollider.Pivot = mathf.NewVec2(-2, 1)
	if err := sprite.SetColliderShape(true, PolygonCollider, points); err != nil {
		t.Fatalf("SetColliderShape(trigger) failed: %v", err)
	}

	wantPoints := []float32{-2, -4, 6, -8, 10, 12}
	if mgr.collisionCenter != mathf.NewVec2(4, 6) || !slices.Equal(mgr.collisionPoints, wantPoints) {
		t.Fatalf("collision polygon = (%v, %v), want (%v, %v)", mgr.collisionCenter, mgr.collisionPoints, mathf.NewVec2(4, 6), wantPoints)
	}
	if !slices.Equal(mgr.collisionEnabled, []bool{true}) {
		t.Fatalf("collision enabled calls = %v, want [true]", mgr.collisionEnabled)
	}
	if mgr.triggerCenter != mathf.NewVec2(-4, 2) || !slices.Equal(mgr.triggerPoints, wantPoints) {
		t.Fatalf("trigger polygon = (%v, %v), want (%v, %v)", mgr.triggerCenter, mgr.triggerPoints, mathf.NewVec2(-4, 2), wantPoints)
	}
	if !slices.Equal(mgr.triggerEnabled, []bool{true}) {
		t.Fatalf("trigger enabled calls = %v, want [true]", mgr.triggerEnabled)
	}
	if !slices.Equal(points, []float64{-1, -2, 3, -4, 5, 6}) {
		t.Fatalf("input points were mutated: %v", points)
	}

	sprite.runtimeState.scale = 3
	sprite.updatePhysicsShapesScale()
	wantRescaledPoints := []float32{-3, -6, 9, -12, 15, 18}
	if !slices.Equal(mgr.collisionPoints, wantRescaledPoints) || !slices.Equal(mgr.triggerPoints, wantRescaledPoints) {
		t.Fatalf("rescaled polygons = (%v, %v), want %v", mgr.collisionPoints, mgr.triggerPoints, wantRescaledPoints)
	}
}

func TestShapeRefreshKeepsCollidersDisabled(t *testing.T) {
	sprite, mgr := newColliderTestSprite(t)
	params := []float64{-1, -2, 3, -4, 5, 6}
	for _, trigger := range []bool{false, true} {
		if err := sprite.SetColliderShape(trigger, PolygonCollider, params); err != nil {
			t.Fatal(err)
		}
	}
	sprite.SetCollisionEnabled(false)
	sprite.SetTriggerEnabled(false)

	sprite.runtimeState.scale = 3
	sprite.updatePhysicsShapesScale()
	sprite.SetColliderPivot(false, 1, 2)
	sprite.SetColliderPivot(true, 3, 4)
	if !slices.Equal(mgr.collisionEnabled, []bool{true, false}) || !slices.Equal(mgr.triggerEnabled, []bool{true, false}) {
		t.Fatalf("shape refresh changed enabled state: collision=%v trigger=%v", mgr.collisionEnabled, mgr.triggerEnabled)
	}

	if err := sprite.SetColliderShape(false, PolygonCollider, params); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mgr.collisionEnabled, []bool{true, false, true}) {
		t.Fatalf("explicit shape change did not enable collision: %v", mgr.collisionEnabled)
	}
}

func TestProxyCollectionRefreshesAutoShapesWithoutTransformChanges(t *testing.T) {
	sprite, mgr := newColliderTestSprite(t)
	sprite.costumes[0].atlas = false
	previous := cachedBounds
	cachedBounds = map[string]mathf.Rect2{
		sprite.costumes[0].path: {Size: mathf.NewVec2(20, 30)},
	}
	t.Cleanup(func() { cachedBounds = previous })
	sprite.physics().triggerCollider.Params = []float64{1, 1}
	sprite.SetTriggerEnabled(false)

	sprite.markAutoPhysicsShapesDirty()
	buffer := engine.NewSpriteSyncBuffer(1)
	sprite.prepareProxyUpdate()
	sprite.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 0 {
		t.Fatalf("auto shape refresh batched %d transforms, want 0", got)
	}
	if !slices.Equal(mgr.triggerEnabled, []bool{false}) || mgr.rectSize != mathf.NewVec2(44, 64) {
		t.Fatalf("costume refresh: enabled=%v size=%v", mgr.triggerEnabled, mgr.rectSize)
	}
}

func TestSetColliderShapeRejectsInvalidParamsAtomically(t *testing.T) {
	sprite, mgr := newColliderTestSprite(t)
	valid := []float64{-1, -2, 3, -4, 5, 6}
	if err := sprite.SetColliderShape(false, PolygonCollider, valid); err != nil {
		t.Fatalf("SetColliderShape(collision) failed: %v", err)
	}
	if err := sprite.SetColliderShape(true, PolygonCollider, valid); err != nil {
		t.Fatalf("SetColliderShape(trigger) failed: %v", err)
	}

	collisionType, collisionParams := sprite.ColliderShape(false)
	triggerType, triggerParams := sprite.ColliderShape(true)
	collisionPoints := slices.Clone(mgr.collisionPoints)
	triggerPoints := slices.Clone(mgr.triggerPoints)
	collisionEnabledCalls := slices.Clone(mgr.collisionEnabled)
	triggerEnabledCalls := slices.Clone(mgr.triggerEnabled)
	collisionParamsStorage := &sprite.physics().bodyCollider.Params[0]
	triggerParamsStorage := &sprite.physics().triggerCollider.Params[0]

	wantErr := "invalid polygon: want at least 3 coordinate pairs, got 5 values"
	if err := sprite.SetColliderShape(false, PolygonCollider, []float64{1, 2, 3, 4, 5}); err == nil || err.Error() != wantErr {
		t.Fatalf("SetColliderShape(collision) error = %v, want %q", err, wantErr)
	}
	if err := sprite.SetColliderShape(true, PolygonCollider, []float64{1, 2, 3, 4, 5}); err == nil || err.Error() != wantErr {
		t.Fatalf("SetColliderShape(trigger) error = %v, want %q", err, wantErr)
	}

	gotCollisionType, gotCollisionParams := sprite.ColliderShape(false)
	if gotCollisionType != collisionType || !slices.Equal(gotCollisionParams, collisionParams) {
		t.Fatalf("collision shape changed after rejection: type=%d params=%v, want type=%d params=%v", gotCollisionType, gotCollisionParams, collisionType, collisionParams)
	}
	gotTriggerType, gotTriggerParams := sprite.ColliderShape(true)
	if gotTriggerType != triggerType || !slices.Equal(gotTriggerParams, triggerParams) {
		t.Fatalf("trigger shape changed after rejection: type=%d params=%v, want type=%d params=%v", gotTriggerType, gotTriggerParams, triggerType, triggerParams)
	}
	if &sprite.physics().bodyCollider.Params[0] != collisionParamsStorage || &sprite.physics().triggerCollider.Params[0] != triggerParamsStorage {
		t.Fatal("rejected shape replaced live parameter storage")
	}
	if !slices.Equal(mgr.collisionPoints, collisionPoints) || !slices.Equal(mgr.triggerPoints, triggerPoints) {
		t.Fatalf("engine proxy points changed after rejection: collision=%v trigger=%v", mgr.collisionPoints, mgr.triggerPoints)
	}
	if !slices.Equal(mgr.collisionEnabled, collisionEnabledCalls) || !slices.Equal(mgr.triggerEnabled, triggerEnabledCalls) {
		t.Fatalf("engine proxy enabled calls changed after rejection: collision=%v trigger=%v", mgr.collisionEnabled, mgr.triggerEnabled)
	}
}

func TestSetColliderShapeCopiesInputParams(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	tests := []struct {
		name      string
		isTrigger bool
		params    []float64
	}{
		{name: "collision", params: []float64{-1, -2, 3, -4, 5, 6}},
		{name: "trigger", isTrigger: true, params: []float64{1, 2, 3, 4, 5, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := slices.Clone(tt.params)
			if err := sprite.SetColliderShape(tt.isTrigger, PolygonCollider, tt.params); err != nil {
				t.Fatalf("SetColliderShape failed: %v", err)
			}
			tt.params[0] = 99
			assertColliderParams(t, sprite, tt.isTrigger, PolygonCollider, want)
		})
	}
}

func TestPhysicsInitializationCopiesShapeParams(t *testing.T) {
	collisionParams := []float64{-1, -2, 3, -4, 5, 6}
	triggerParams := []float64{1, 2, 3, 4, 5, 6}
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		CollisionShapeType:   "polygon",
		CollisionShapeParams: collisionParams,
		TriggerShapeType:     "polygon",
		TriggerShapeParams:   triggerParams,
	})

	collisionParams[0] = 99
	triggerParams[0] = 101
	assertColliderParams(t, sprite, false, PolygonCollider, []float64{-1, -2, 3, -4, 5, 6})
	assertColliderParams(t, sprite, true, PolygonCollider, []float64{1, 2, 3, 4, 5, 6})
}

func TestPhysicsConfigDimensions(t *testing.T) {
	for _, shape := range []struct {
		name                string
		kind                ColliderShapeType
		one, full, negative mathf.Vec2
	}{
		{"rect", collision.ColliderRect, mathf.Vec2{}, mathf.NewVec2(2, 3), mathf.NewVec2(0, 3)},
		{"auto", collision.ColliderAuto, mathf.Vec2{}, mathf.NewVec2(2, 3), mathf.NewVec2(0, 3)},
		{"none", collision.ColliderNone, mathf.Vec2{}, mathf.NewVec2(2, 3), mathf.NewVec2(0, 3)},
		{"polygon", collision.ColliderPolygon, mathf.Vec2{}, mathf.NewVec2(2, 3), mathf.NewVec2(0, 3)},
		{"unknown", 99, mathf.Vec2{}, mathf.NewVec2(2, 3), mathf.NewVec2(0, 3)},
		{"circle", collision.ColliderCircle, mathf.NewVec2(4, 4), mathf.NewVec2(4, 4), mathf.Vec2{}},
		{"capsule", collision.ColliderCapsule, mathf.Vec2{}, mathf.NewVec2(4, 3), mathf.NewVec2(0, 3)},
	} {
		t.Run(shape.name, func(t *testing.T) {
			for _, tt := range []struct {
				name   string
				params []float64
				want   mathf.Vec2
			}{
				{"nil", nil, mathf.Vec2{}},
				{"empty", []float64{}, mathf.Vec2{}},
				{"one", []float64{2}, shape.one},
				{"two", []float64{2, 3}, shape.full},
				{"extra", []float64{2, 3, 4}, shape.full},
				{"negative", []float64{-2, 3}, shape.negative},
			} {
				t.Run(tt.name, func(t *testing.T) {
					cfg := colliderConfig{Type: shape.kind, Params: tt.params}
					x, y := cfg.getDimensions()
					if got := mathf.NewVec2(x, y); got != tt.want {
						t.Fatalf("dimensions = %v, want %v", got, tt.want)
					}
				})
			}
		})
	}
}

func TestPhysicsApplyRectAndAutoShapes(t *testing.T) {
	for _, shape := range []struct {
		name  string
		kind  ColliderShapeType
		pivot mathf.Vec2
	}{
		{"rect", collision.ColliderRect, mathf.NewVec2(6, 8)},
		{"auto", collision.ColliderAuto, mathf.NewVec2(80, -40)},
	} {
		for _, trigger := range []bool{false, true} {
			for _, params := range [][]float64{nil, {}, {2}, {2, 3}, {-2, 3, 4}} {
				t.Run(fmt.Sprintf("%s/trigger=%t/params=%#v", shape.name, trigger, params), func(t *testing.T) {
					sprite, mgr := newColliderTestSprite(t)
					cfg := colliderConfig{Type: shape.kind, Pivot: mathf.NewVec2(3, 4), Params: params}
					cfg.syncShape(sprite.runtimeState.proxy, trigger, sprite)
					if len(params) < 2 {
						if mgr.rectCalls != 0 {
							t.Fatalf("short params caused %d rect calls", mgr.rectCalls)
						}
						return
					}
					wantSize := mathf.NewVec2(params[0]*2, params[1]*2)
					if mgr.rectCalls != 1 || mgr.rectIsTrigger != trigger || mgr.rectCenter != shape.pivot || mgr.rectSize != wantSize {
						t.Fatalf("rect calls=%d trigger=%t pivot=%v size=%v, want 1/%t/%v/%v", mgr.rectCalls, mgr.rectIsTrigger, mgr.rectCenter, mgr.rectSize, trigger, shape.pivot, wantSize)
					}
				})
			}
		}
	}
}

func TestSpritePhysicsAPIKeepsBackendAndColliderState(t *testing.T) {
	setupImmediateEngineCalls(t)
	sprite, _ := newColliderTestSprite(t)
	mgr := &spritePhysicsAPIManager{floor: true}
	pkgengine.SpriteMgr = mgr
	sprite.SetVelocity(3, 4)
	sprite.AddImpulse(-2, 5)
	sprite.SetGravity(0.25)
	if x, y := sprite.Velocity(); x != 3 || y != 4 || mgr.impulse != mathf.NewVec2(-2, 5) || sprite.Gravity() != 0.25 || !sprite.IsOnFloor() {
		t.Fatal("physics API lost backend values")
	}
	sprite.SetCollisionLayer(2)
	sprite.SetCollisionMask(4)
	sprite.SetCollisionEnabled(true)
	sprite.SetTriggerLayer(8)
	sprite.SetTriggerMask(16)
	sprite.SetTriggerEnabled(false)
	if sprite.CollisionLayer() != 2 || sprite.CollisionMask() != 4 || !sprite.CollisionEnabled() || sprite.TriggerLayer() != 8 || sprite.TriggerMask() != 16 || sprite.TriggerEnabled() {
		t.Fatal("collision/trigger API mixed independent settings")
	}
	for _, trigger := range []bool{false, true} {
		for _, tt := range []struct {
			kind   ColliderShapeType
			params []float64
		}{{collision.ColliderCircle, []float64{3}}, {collision.ColliderCapsule, []float64{3, 8}}} {
			if err := sprite.SetColliderShape(trigger, tt.kind, tt.params); err != nil {
				t.Fatal(err)
			}
			sprite.SetColliderPivot(trigger, 3, 4)
			if x, y := sprite.ColliderPivot(trigger); x != 3 || y != 4 {
				t.Fatalf("collider pivot=(%v,%v),want(3,4)", x, y)
			}
			if mgr.shapeCenter != mathf.NewVec2(6, 8) || mgr.shapeTrigger != trigger {
				t.Fatalf("shape center=%v, trigger=%v", mgr.shapeCenter, mgr.shapeTrigger)
			}
			if tt.kind == collision.ColliderCircle && mgr.circleRadius != 6 {
				t.Fatalf("scaled circle radius=%v,want6", mgr.circleRadius)
			}
			if tt.kind == collision.ColliderCapsule && mgr.capsuleSize != mathf.NewVec2(12, 16) {
				t.Fatalf("scaled capsule=%v,want(12,16)", mgr.capsuleSize)
			}
		}
	}
	sprite.runtimeState.proxy = nil
	sprite.updatePhysicsShapesScale()
}

func TestPhysicsShapeValidation(t *testing.T) {
	for _, tt := range []struct {
		kind    ColliderShapeType
		params  []float64
		wantErr string
	}{
		{collision.ColliderNone, nil, ""}, {collision.ColliderAuto, nil, ""},
		{collision.ColliderRect, []float64{2, 3}, ""},
		{collision.ColliderRect, []float64{2}, "invalid rect parameter count: got 1, want 2"},
		{collision.ColliderCircle, []float64{2}, ""},
		{collision.ColliderCircle, []float64{2, 3}, "invalid circle parameter count: got 2, want 1"},
		{collision.ColliderCapsule, []float64{2, 3}, ""},
		{collision.ColliderCapsule, []float64{2}, "invalid capsule parameter count: got 1, want 2"},
		{collision.ColliderPolygon, []float64{0, 0, 1, 0, 0, 1}, ""},
		{collision.ColliderPolygon, []float64{0, 0, 1, 0}, "invalid polygon: want at least 3 coordinate pairs, got 4 values"},
		{collision.ColliderPolygon, []float64{0, 0, 1, 0, 0, 1, 2}, "invalid polygon: want at least 3 coordinate pairs, got 7 values"},
		{ColliderShapeType(99), nil, "unknown collider shape type: 99"},
	} {
		cfg := colliderConfig{Type: tt.kind, Params: tt.params}
		err := cfg.validateShape()
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("shape %v: %v", cfg, err)
			}
		} else if err == nil || err.Error() != tt.wantErr {
			t.Errorf("shape %v error = %v, want %q", cfg, err, tt.wantErr)
		}
	}
	for name, want := range map[string]PhysicsMode{"kinematic": KinematicPhysics, "dynamic": DynamicPhysics, "static": StaticPhysics, "no": NoPhysics, "unknown": NoPhysics} {
		if got := toPhysicsMode(name); got != want {
			t.Errorf("physics mode %s=%v,want%v", name, got, want)
		}
	}
}

func TestNormalizeEdgeArea(t *testing.T) {
	for _, tt := range []struct {
		area, want string
	}{
		{"", edgeAreaStage},
		{"stage", edgeAreaStage},
		{"STAGE", edgeAreaStage},
		{"camera", edgeAreaCamera},
		{"CaMeRa", edgeAreaCamera},
		{"viewport", edgeAreaCamera},
		{"VIEWPORT", edgeAreaCamera},
		{"unknown", edgeAreaStage},
	} {
		t.Run(tt.area, func(t *testing.T) {
			if got := normalizeEdgeArea(tt.area); got != tt.want {
				t.Fatalf("normalizeEdgeArea(%q) = %q, want %q", tt.area, got, tt.want)
			}
		})
	}
}

func TestSpriteTouchingTargetDefaultsToStageArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageTouching:  touchingScreenLeft,
		cameraTouching: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(0)

	if !sprite.Touching__2(EdgeLeft) {
		t.Fatal("Touching__2(EdgeLeft) = false, want true for default stage area")
	}
	if sprite.Touching__2(EdgeRight) {
		t.Fatal("Touching__2(EdgeRight) = true, want false when only camera boundary is touched")
	}
	if sprite.checkTouchingScreen(touchingScreenRight, edgeAreaViewport) == 0 {
		t.Fatal("checkTouchingScreen(EdgeRight, viewport) = 0, want non-zero")
	}
}

func TestSpriteBounceOffEdgeDefaultsToStageArea(t *testing.T) {
	for _, tt := range []struct {
		name          string
		stage, camera int64
		from, want    Direction
	}{
		{"left", touchingScreenLeft, touchingScreenRight, -90, 90},
		{"right", touchingScreenRight, touchingScreenLeft, 90, -90},
		{"top", touchingScreenTop, touchingScreenBottom, 0, 180},
		{"bottom", touchingScreenBottom, touchingScreenTop, 180, 0},
		{"top diagonal", touchingScreenTop, touchingScreenBottom, 45, 135},
		{"bottom diagonal", touchingScreenBottom, touchingScreenTop, 135, 45},
	} {
		t.Run(tt.name, func(t *testing.T) {
			installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
				stageNearest:  tt.stage,
				cameraNearest: tt.camera,
			})
			sprite := newEdgeTestSprite(tt.from)
			version := sprite.spriteState.dirtyVersion

			sprite.BounceOffEdge()

			if got := sprite.Heading(); math.IsNaN(got) || math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("BounceOffEdge() heading = %v, want %v", got, tt.want)
			}
			if sprite.spriteState.dirtyVersion == version {
				t.Fatal("bounce did not publish the transform change")
			}
		})
	}
}

func TestSpriteBounceOffEdgeWithCameraArea(t *testing.T) {
	installEdgePhysicsMgr(t, &fakeEdgePhysicsMgr{
		stageNearest:  touchingScreenLeft,
		cameraNearest: touchingScreenRight,
	})

	sprite := newEdgeTestSprite(-90)

	sprite.transform().bounceOffEdge(edgeAreaCamera)

	if got := sprite.Heading(); got != -90 {
		t.Fatalf("BounceOffEdge(camera) heading = %v, want -90", got)
	}
}

func TestPhysicsComponentRejectsMalformedConfiguredShapes(t *testing.T) {
	var warnings bytes.Buffer
	spxlog.SetStdoutOutput(&warnings)
	t.Cleanup(func() { spxlog.SetStdoutOutput(os.Stdout) })

	sprite := &SpriteImpl{g: &Game{}, name: "invalid"}
	component := &physicsComponent{}
	component.initialize(sprite, &coreproject.SpriteConfig{
		CollisionShapeType: "rect", CollisionShapeParams: []float64{1},
		TriggerShapeType: "circle", TriggerShapeParams: []float64{1, 2},
	})
	if component.bodyCollider.Type != collision.ColliderNone || component.bodyCollider.Params != nil ||
		component.triggerCollider.Type != collision.ColliderAuto || component.triggerCollider.Params != nil {
		t.Fatalf("invalid shapes were retained: collision=%+v trigger=%+v", component.bodyCollider, component.triggerCollider)
	}
	output := warnings.String()
	if strings.Count(output, "[WARN]") != 2 ||
		!strings.Contains(output, "Invalid collider for sprite invalid: invalid rect parameter count: got 1, want 2; using none") ||
		!strings.Contains(output, "Invalid trigger for sprite invalid: invalid circle parameter count: got 2, want 1; using auto") {
		t.Fatalf("warnings must report each shape's error and fallback once: %q", output)
	}
}

func TestBaseObjGetCostumeAssetPath(t *testing.T) {
	obj := visualObject{
		costumes: []*costume{{
			path: "sprites/Eat/costume1.svg",
		}},
	}

	got := obj.currentCostume().getAssetPath()
	want := engine.ToAssetPath("sprites/Eat/costume1.svg")
	if got != want {
		t.Fatalf("currentCostume().getAssetPath() = %q, want %q", got, want)
	}
}

func TestBaseObjCostumeSelectionRejectsInvalidInputWithoutChangingState(t *testing.T) {
	obj := visualObject{costumes: []*costume{{name: "first"}, {name: "second"}}}
	for _, tt := range []struct {
		value any
		index int
		ok    bool
	}{
		{Prev, 1, true}, {Next, 0, true}, {1.9, 1, true},
		{-1, 1, false}, {2, 1, false}, {struct{}{}, 1, false},
		{"first", 0, true},
	} {
		if got := obj.goSetCostume(tt.value); got != tt.ok || obj.getCostumeIndex() != tt.index {
			t.Fatalf("select %v: ok=%v index=%d, want %v/%d", tt.value, got, obj.getCostumeIndex(), tt.ok, tt.index)
		}
		if obj.getCostumeName() != obj.costumes[tt.index].name {
			t.Fatal("selected costume name does not match its index")
		}
	}
}

func TestBaseObjInitialCostumeIndexFallsBackToFirst(t *testing.T) {
	for _, index := range []int{-1, 2} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			cfg := &coreproject.CostumeConfig{Name: "first", ImageWidth: 20, ImageHeight: 10}
			var sprite, stage visualObject
			sprite.init([]*coreproject.CostumeConfig{cfg}, index)
			stage.initBackdrops([]*coreproject.BackdropConfig{{CostumeConfig: *cfg}}, index)
			for _, obj := range []*visualObject{&sprite, &stage} {
				if obj.costumeIndex != 0 || obj.getCostumeName() != "first" || !obj.runtimeState.isCostumeDirty {
					t.Fatalf("initial costume = %d/%q, dirty=%v", obj.costumeIndex, obj.getCostumeName(), obj.runtimeState.isCostumeDirty)
				}
			}
		})
	}
}

func TestBaseObjInitializesSingleCostumeSetWithRect(t *testing.T) {
	config := &coreproject.SpriteConfig{
		CostumeIndex: 9,
		CostumeSet: &coreproject.CostumeSet{
			Path: "hero.png", ImageWidth: 100, ImageHeight: 40, Nx: 2,
			BitmapResolution: 2, FaceRight: 90,
			Rect: &coreproject.CostumeSetRect{X: 20, Y: 10, W: 40, H: 20},
		},
	}
	layout, err := coreproject.PrepareCostumeLayout(config)
	if err != nil {
		t.Fatal(err)
	}
	var obj visualObject
	obj.initSpriteCostumes(config, layout)
	if !obj.runtimeState.isCostumeSet || len(obj.costumes) != 2 || obj.costumeIndex != 0 {
		t.Fatalf("costume set = %v, frames = %d, initial = %d", obj.runtimeState.isCostumeSet, len(obj.costumes), obj.costumeIndex)
	}
	for i, costume := range obj.costumes {
		if costume.frame.PosX != 20+i*20 || costume.frame.PosY != 10 || costume.frame.Width != 20 || costume.frame.Height != 20 || costume.frame.BitmapResolution != 2 || costume.faceRight != 90 {
			t.Fatalf("frame %d = %+v", i, costume)
		}
	}
}

func TestBaseObjGraphicEffectsAccumulateBeforeProxyExists(t *testing.T) {
	var obj visualObject
	obj.changeGraphicEffect(ColorEffect, 25)
	obj.changeGraphicEffect(ColorEffect, -75)
	if obj.graphicEffects[ColorEffect] != -50 {
		t.Fatalf("accumulated color effect = %v, want -50", obj.graphicEffects[ColorEffect])
	}
	if got := normalizeEffectValue(ColorEffect, -50); got != 0.75 {
		t.Fatalf("wrapped color effect = %v, want 0.75", got)
	}
	if got := normalizeEffectValue(EffectKind(999), 42); got != 42 {
		t.Fatalf("unknown effect = %v, want unchanged 42", got)
	}
	obj.setMaterialParams("color", 0.75, true)
	obj.setMaterialParamsVec4("remap", mathf.Vec4{X: 1}, false)
	if obj.runtimeState.hasShader {
		t.Fatal("unbound material update created a shader")
	}
}

func TestBaseObjGraphicEffectLimits(t *testing.T) {
	for _, tt := range []struct {
		name                string
		kind                EffectKind
		value, delta        float64
		wantSet, wantChange float64
	}{
		{"brightness below minimum", BrightnessEffect, -125, 5, -100, -95},
		{"brightness above maximum", BrightnessEffect, 125, -5, 100, 95},
		{"brightness crosses minimum", BrightnessEffect, -95, -10, -95, -100},
		{"brightness crosses maximum", BrightnessEffect, 95, 10, 95, 100},
		{"brightness fractional", BrightnessEffect, -12.5, -0.5, -12.5, -13},
		{"ghost below minimum", GhostEffect, -25, 5, 0, 5},
		{"ghost above maximum", GhostEffect, 125, -5, 100, 95},
		{"ghost crosses minimum", GhostEffect, 5, -10, 5, 0},
		{"ghost crosses maximum", GhostEffect, 95, 10, 95, 100},
		{"ghost fractional", GhostEffect, 12.5, 0.5, 12.5, 13},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var obj visualObject
			obj.setGraphicEffect(tt.kind, tt.value)
			if got := obj.graphicEffects[tt.kind]; got != tt.wantSet {
				t.Errorf("effect after set = %v, want %v", got, tt.wantSet)
			}
			obj.changeGraphicEffect(tt.kind, tt.delta)
			if got := obj.graphicEffects[tt.kind]; got != tt.wantChange {
				t.Errorf("effect after change = %v, want %v", got, tt.wantChange)
			}
		})
	}
}

func TestBaseObjOtherGraphicEffectsRemainUnbounded(t *testing.T) {
	for _, kind := range []EffectKind{ColorEffect, FishEyeEffect, WhirlEffect, PixelateEffect, MosaicEffect} {
		t.Run(kind.String(), func(t *testing.T) {
			var obj visualObject
			obj.setGraphicEffect(kind, 1000)
			if got := obj.graphicEffects[kind]; got != 1000 {
				t.Fatalf("effect after set = %v, want 1000", got)
			}
			obj.changeGraphicEffect(kind, -2000)
			if got := obj.graphicEffects[kind]; got != -1000 {
				t.Fatalf("effect after change = %v, want -1000", got)
			}
		})
	}
}

func TestInitSpriteCostumesConsumesPreparedLayout(t *testing.T) {
	assetPath := costumeAssetPath("sprites/hero.png")
	costumeSizeCache.Store(costumeSizeKey{path: assetPath}, mathf.NewVec2(20, 10))
	t.Cleanup(func() { costumeSizeCache.Delete(costumeSizeKey{path: assetPath}) })

	config := &coreproject.SpriteConfig{
		CostumeMPSet: &coreproject.CostumeMPSet{
			Path:             "sprites/hero.png",
			FaceRight:        90,
			BitmapResolution: 2,
			Parts: []coreproject.CostumeSetPart{
				{Nx: 1, Rect: coreproject.CostumeSetRect{W: 10, H: 10}},
				{
					Nx:    2,
					Rect:  coreproject.CostumeSetRect{W: 20, H: 10},
					Items: []coreproject.CostumeSetItem{{NamePrefix: "run", N: 2}},
				},
			},
		},
		CostumeIndex: 2,
	}
	layout, err := coreproject.PrepareCostumeLayout(config)
	if err != nil {
		t.Fatal(err)
	}

	var obj visualObject
	obj.initSpriteCostumes(config, layout)

	names := make([]string, len(obj.costumes))
	positions := make([]int, len(obj.costumes))
	for i, costume := range obj.costumes {
		names[i] = costume.name
		positions[i] = costume.frame.PosX
		if !costume.isAtlas() {
			t.Fatalf("costume %d is not an atlas frame", i)
		}
	}
	if want := []string{"0", "run0", "run1"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("costume names = %v, want %v", names, want)
	}
	if want := []int{0, 0, 10}; !reflect.DeepEqual(positions, want) {
		t.Fatalf("costume frame positions = %v, want %v", positions, want)
	}
	if obj.costumeIndex != 2 {
		t.Fatalf("costume index = %d, want 2", obj.costumeIndex)
	}
}

func TestToBitmapResolution(t *testing.T) {
	if got := normalizeBitmapResolution(0); got != defaultBitmapResolution {
		t.Fatalf("normalizeBitmapResolution(0) = %d, want %d", got, defaultBitmapResolution)
	}
	if got := normalizeBitmapResolution(2); got != 2 {
		t.Fatalf("normalizeBitmapResolution(2) = %d, want 2", got)
	}
}

func TestDefaultAtlasUV(t *testing.T) {
	if got := defaultAtlasUV(); got != mathf.NewVec4(0, 0, fullUVRange, fullUVRange) {
		t.Fatalf("defaultAtlasUV() = %v, want full UV range", got)
	}
}

func TestCalculateAtlasUV(t *testing.T) {
	got := calculateAtlasUV(10, 20, 30, 40, mathf.NewVec2(100, 200))
	want := mathf.NewVec4(0.1, 0.1, 0.3, 0.2)
	if got != want {
		t.Fatalf("calculateAtlasUV() = %v, want %v", got, want)
	}
}

func TestResolveImageSize(t *testing.T) {
	got := resolveCostumeImageSize(32, 16, "ignored", func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called when config size is present")
		return mathf.Vec2{}
	})
	if got != (mathf.Vec2{X: 32, Y: 16}) {
		t.Fatalf("resolveCostumeImageSize configured = %v, want {32 16}", got)
	}

	fallbackCalled := false
	got = resolveCostumeImageSize(0, 0, "path", func(path string) mathf.Vec2 {
		fallbackCalled = true
		if path != "path" {
			t.Fatalf("fallback path = %q, want %q", path, "path")
		}
		return mathf.NewVec2(8, 4)
	})
	if !fallbackCalled {
		t.Fatal("fallback was not called")
	}
	if got != mathf.NewVec2(8, 4) {
		t.Fatalf("resolveCostumeImageSize fallback = %v, want {8 4}", got)
	}
}

func TestNewSizedFrame(t *testing.T) {
	got := newSizedCostumeFrame(10, 20)
	if got.Width != 10 || got.Height != 20 {
		t.Fatalf("newSizedCostumeFrame size = (%d, %d), want (10, 20)", got.Width, got.Height)
	}
	if got.Center != mathf.NewVec2(5, 10) {
		t.Fatalf("newSizedCostumeFrame center = %v, want {5 10}", got.Center)
	}
}

func TestNewAtlasFrame(t *testing.T) {
	tests := []struct {
		name              string
		frameCount, index int
		rect              [4]float64
		width             int
		height            int
		posX, posY        int
	}{
		{"full image", 5, 2, [4]float64{}, 20, 20, 40, 0},
		{"zero columns", 0, 0, [4]float64{}, 100, 20, 0, 0},
		{"negative columns", -2, 0, [4]float64{}, 100, 20, 0, 0},
		{"fractional region", 3, 1, [4]float64{7.5, 3.5, 31.5, 11.5}, 10, 11, 17, 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			img := &costumeSetImage{
				width: 100, height: 20, path: "path", frameCount: test.frameCount,
				rect: coreproject.CostumeSetRect{X: test.rect[0], Y: test.rect[1], W: test.rect[2], H: test.rect[3]},
			}
			got := newAtlasCostumeFrame(img, test.index, 2, func(string) mathf.Vec2 {
				t.Fatal("fallback should not be called")
				return mathf.Vec2{}
			})
			want := costumeFrame{
				Width: test.width, Height: test.height, BitmapResolution: 2,
				ImageSize: mathf.NewVec2(100, 20),
				Center:    mathf.NewVec2(float64(test.width)/2, float64(test.height)/2),
				PosX:      test.posX, PosY: test.posY,
				AtlasUVRect: mathf.NewVec4(float64(test.posX)/100, float64(test.posY)/20, float64(test.width)/100, float64(test.height)/20),
			}
			if got != want {
				t.Fatalf("newAtlasCostumeFrame() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestNewStandaloneFrame(t *testing.T) {
	got := newStandaloneCostumeFrame(&coreproject.CostumeConfig{ImageWidth: 32, ImageHeight: 16, Path: "path"}, func(string) mathf.Vec2 {
		t.Fatal("fallback should not be called")
		return mathf.Vec2{}
	})
	if got.Width != 32 || got.Height != 16 {
		t.Fatalf("newStandaloneCostumeFrame size = (%d, %d), want (32, 16)", got.Width, got.Height)
	}
	if got.BitmapResolution != defaultBitmapResolution {
		t.Fatalf("newStandaloneCostumeFrame bitmap resolution = %d, want %d", got.BitmapResolution, defaultBitmapResolution)
	}
	if got.Center != mathf.NewVec2(16, 8) {
		t.Fatalf("newStandaloneCostumeFrame center = %v, want {16 8}", got.Center)
	}
}

func TestCostumeFrameFallback(t *testing.T) {
	calls := 0
	fallback := func(path string) mathf.Vec2 {
		calls++
		if path != "image.svg" {
			t.Fatalf("fallback path = %q, want image.svg", path)
		}
		return mathf.NewVec2(31.5, 17.5)
	}
	img := costumeSetImage{path: "image.svg", frameCount: -1}
	atlas := newAtlasCostumeFrame(&img, 1, 0, fallback)
	if atlas.Width != 31 || atlas.Height != 17 || atlas.PosX != 31 || atlas.BitmapResolution != 0 || img.frameCount != -1 {
		t.Fatalf("atlas fallback = %+v, columns = %d", atlas, img.frameCount)
	}
	config := coreproject.CostumeConfig{Path: "image.svg"}
	standalone := newStandaloneCostumeFrame(&config, fallback)
	if standalone.ImageSize != mathf.NewVec2(31.5, 17.5) || standalone.Center != mathf.NewVec2(15.75, 8.75) || standalone.BitmapResolution != 1 || config.BitmapResolution != 0 {
		t.Fatalf("standalone fallback = %+v, configured resolution = %d", standalone, config.BitmapResolution)
	}
	if calls != 2 {
		t.Fatalf("fallback calls = %d, want 2", calls)
	}
}

func TestSpriteSetCostume(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		costume                 SpriteCostumeName
		defaultIndex, wantIndex int
		wantVersion             uint64
		wantDirty               bool
	}{
		{name: "missing name is no-op", costume: "missing", wantIndex: 1},
		{name: "valid name updates state", costume: "idle", defaultIndex: 1, wantVersion: 1, wantDirty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite := newTestRenderSprite()
			sprite.spriteState.defaultCostumeIndex = tt.defaultIndex

			sprite.setCostume(tt.costume)

			if got := sprite.costumeIndex; got != tt.wantIndex {
				t.Errorf("costumeIndex = %d, want %d", got, tt.wantIndex)
			}
			if got := sprite.spriteState.defaultCostumeIndex; got != 0 {
				t.Errorf("DefaultCostumeIndex = %d, want 0", got)
			}
			if got := uint64(sprite.spriteState.dirtyVersion); got != tt.wantVersion {
				t.Errorf("DirtyVersion = %d, want %d", got, tt.wantVersion)
			}
			if got := sprite.runtimeState.isCostumeDirty; got != tt.wantDirty {
				t.Errorf("IsCostumeDirty = %v, want %v", got, tt.wantDirty)
			}
		})
	}
}

func TestResolveCostumeIndex(t *testing.T) {
	sprite := newTestRenderSprite()

	tests := []struct {
		name    string
		costume string
		want    int
	}{
		{name: "name", costume: "run", want: 1},
		{name: "zero", costume: "0", want: -1},
		{name: "in range", costume: "2", want: 1},
		{name: "wrap", costume: "3", want: 0},
		{name: "wrap to zero", costume: "4", want: 1},
		{name: "negative whole cycle", costume: "-1", want: 0},
		{name: "negative remainder", costume: "-2", want: -1},
		{name: "explicit positive sign", costume: "+2", want: 1},
		{name: "minimum integer wraps subtraction", costume: strconv.Itoa(math.MinInt), want: 1},
		{name: "maximum integer", costume: strconv.Itoa(math.MaxInt), want: 0},
		{name: "integer overflow", costume: strconv.Itoa(math.MaxInt) + "0", want: -1},
		{name: "fraction is not an integer", costume: "2.5", want: -1},
		{name: "whitespace is not trimmed", costume: " 2 ", want: -1},
		{name: "compound signs are not accepted", costume: "--2", want: -1},
		{name: "invalid", costume: "missing", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sprite.ResolveCostumeIndex(tt.costume); got != tt.want {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want %d", tt.costume, got, tt.want)
			}
		})
	}
}

func TestResolveCostumeIndexPrefersNames(t *testing.T) {
	for _, name := range []string{"1", "0", "-2", " 2 ", strconv.Itoa(math.MaxInt) + "0"} {
		t.Run(name, func(t *testing.T) {
			sprite := newTestRenderSprite()
			sprite.costumes[1].name = name
			if got := sprite.ResolveCostumeIndex(name); got != 1 {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want named costume at 1", name, got)
			}
		})
	}
}

func TestResolveCostumeIndexWithNoCostumes(t *testing.T) {
	sprite := &SpriteImpl{}

	for _, tt := range []struct {
		costume string
		want    int
	}{
		{"3", 2},
		{"0", -1},
		{"-2", -3},
		{strconv.Itoa(math.MinInt), math.MaxInt},
		{"missing", -1},
	} {
		t.Run(tt.costume, func(t *testing.T) {
			if got := sprite.ResolveCostumeIndex(tt.costume); got != tt.want {
				t.Fatalf("ResolveCostumeIndex(%q) = %d, want %d", tt.costume, got, tt.want)
			}
		})
	}
}

func TestGetCostumeRenderOffsetUsesPivot(t *testing.T) {
	costume := newBackdropCostume(&coreproject.BackdropConfig{
		CostumeConfig: coreproject.CostumeConfig{
			ImageWidth: 200, ImageHeight: 120, BitmapResolution: 2,
			X: 100, Y: 60,
		},
		Pivot: mathf.NewVec2(10, -5),
	})

	x, y := getCostumeRenderOffset(costume, costume.pivot, 1, 1)
	if x != -10 || y != 5 {
		t.Fatalf("getCostumeRenderOffset = (%v, %v), want (-10, 5)", x, y)
	}
}

func TestCostumeRenderAnchorConvertsAssetCoordinatesToSPX(t *testing.T) {
	costume := newCostume(&coreproject.CostumeConfig{
		ImageWidth: 201.5, ImageHeight: 121.25, BitmapResolution: 2,
		X: 80, Y: 40,
	})

	want := mathf.NewVec2(-10.375, 10.3125)
	if got := costume.renderAnchorInSPX(); got != want {
		t.Fatalf("renderAnchorInSPX = %v, want %v", got, want)
	}
	if width, height := costume.sizeInSPX(); width != 100.75 || height != 60.625 {
		t.Fatalf("logical size = (%v, %v), want (100.75, 60.625)", width, height)
	}
	if costume.isAtlas() {
		t.Fatal("standalone costume is marked as an atlas")
	}
}

func TestCostumeCenterInAssetNormalizesStaleCenterOnEmptyFrame(t *testing.T) {
	// An empty vector costume is imported as a 1x1 frame, but the source project can
	// still declare a rotation center sized for its original canvas.
	empty := costumeFrame{Width: 1, Height: 1, BitmapResolution: 1}
	if got := costumeCenterInAsset(empty, mathf.NewVec2(240, 180)); got != (mathf.NewVec2(0.5, 0.5)) {
		t.Fatalf("empty frame center = %v, want {0.5 0.5}", got)
	}
	costume := newCostume(&coreproject.CostumeConfig{
		ImageWidth: 1, ImageHeight: 1, BitmapResolution: 1, X: 240, Y: 180,
	})
	if got := costume.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want {0 0}", got)
	}

	filled := costumeFrame{Width: 960, Height: 720, BitmapResolution: 2}
	if got := costumeCenterInAsset(filled, mathf.NewVec2(480, 360)); got != (mathf.NewVec2(480, 360)) {
		t.Fatalf("filled frame center = %v, want {480 360}", got)
	}

	// A 1xN costume has real pixels and may anchor at an end rather than its center.
	thin := costumeFrame{Width: 1, Height: 200, BitmapResolution: 1}
	if got := costumeCenterInAsset(thin, mathf.NewVec2(0.5, 200)); got != (mathf.NewVec2(0.5, 200)) {
		t.Fatalf("thin frame center = %v, want {0.5 200}", got)
	}
}

func TestSizedCostumeUsesImageCenter(t *testing.T) {
	costume := newCostumeWithSize(7, 5)
	if width, height := costume.displaySize(); width != 7 || height != 5 {
		t.Fatalf("sized costume = (%d, %d), want (7, 5)", width, height)
	}
	if got := costume.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want centered origin", got)
	}
	if costume.isAtlas() {
		t.Fatal("sized costume is marked as an atlas")
	}
}

func TestAtlasCostumePreservesFrameGeometry(t *testing.T) {
	c := newCostumeFromSet("frame1", &costumeSetImage{
		path: "atlas.png", width: 256, height: 128, frameCount: 2,
		rect: coreproject.CostumeSetRect{X: 16, Y: 8, W: 86, H: 43},
	}, 90, 1, 2)
	obj := visualObject{costumes: []*costume{c}}
	if !c.isAtlas() {
		t.Fatal("atlas costume is marked as a standalone image")
	}
	if got, want := obj.getCostumeAtlasRegion(), mathf.NewRect2(59, 8, 43, 43); got != want {
		t.Fatalf("atlas region = %v, want %v", got, want)
	}
	if got, want := c.frame.AtlasUVRect, mathf.NewVec4(59.0/256, 8.0/128, 43.0/256, 43.0/128); got != want {
		t.Fatalf("atlas UV remap = %v, want %v", got, want)
	}
	if width, height := c.displaySize(); width != 21 || height != 21 {
		t.Fatalf("integer size = (%d, %d), want (21, 21)", width, height)
	}
	if width, height := c.sizeInSPX(); width != 21.5 || height != 21.5 {
		t.Fatalf("logical size = (%v, %v), want (21.5, 21.5)", width, height)
	}
	if got := c.renderAnchorInSPX(); got != (mathf.Vec2{}) {
		t.Fatalf("renderAnchorInSPX = %v, want centered origin", got)
	}
}

func TestWorldRenderOffsetAppliesRootFlipAndRotation(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.scale = 2
	sprite.transform().rotationStyle = LeftRight
	sprite.transform().direction = -30

	x, y := getWorldRenderOffset(sprite)
	if x != -74 || y != -48 {
		t.Fatalf("left-right world render offset = (%v, %v), want (-74, -48)", x, y)
	}

	sprite.runtimeState.scale = 1
	sprite.transform().rotationStyle = Normal
	sprite.transform().direction = 45
	x, y = getWorldRenderOffset(sprite)
	wantX := 61 / math.Sqrt2
	wantY := 13 / math.Sqrt2
	if !(math.Abs(x-wantX) <= 1e-9 && math.Abs(y-wantY) <= 1e-9) {
		t.Fatalf("rotated world render offset = (%v, %v), want (%v, %v)", x, y, wantX, wantY)
	}
}

func TestLegacyLeftRightRotationStyleFlipsNegativeHeading(t *testing.T) {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		Heading:       -9,
		RotationStyle: "leftRight",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.costumes = []*costume{{frame: costumeFrame{BitmapResolution: 1}}}

	rotation, scaleX, scaleY := renderRotationAndFlip(sprite)
	if rotation != 0 || scaleX != -1 || scaleY != 1 {
		t.Fatalf("legacy leftRight transform = (%v, %v, %v), want (0, -1, 1)", rotation, scaleX, scaleY)
	}
}

func TestPhysicsShapePivotSeparatesAutoAndExplicitShapes(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.scale = 2

	config := colliderConfig{Pivot: mathf.NewVec2(3, 4), Type: collision.ColliderRect}
	if got, want := config.shapePivot(sprite), mathf.NewVec2(6, 8); got != want {
		t.Fatalf("explicit shape pivot = %v, want %v", got, want)
	}

	config.Type = collision.ColliderAuto
	if got, want := config.shapePivot(sprite), mathf.NewVec2(80, -40); got != want {
		t.Fatalf("auto shape pivot = %v, want %v", got, want)
	}
}

func TestSpriteRenderingOverloadsAndDebugPreserveState(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	installAskTestUiMgr(t)
	sprite := newCloneLimitSprite(game, "actor")
	sprite.costumes = []*costume{{name: "first"}, {name: "second"}, {name: "third"}}
	for _, tt := range []struct {
		run  func()
		want string
	}{
		{func() { sprite.SetCostume__1(1) }, "second"},
		{func() { sprite.SetCostume__2(2) }, "third"},
		{func() { sprite.SetCostume__3(Next) }, "first"},
	} {
		tt.run()
		if sprite.CostumeName() != tt.want {
			t.Fatalf("costume=%q, want %q", sprite.CostumeName(), tt.want)
		}
	}
	if sprite.ResolveCostumeIndex("second") != 1 {
		t.Fatal("debug costume lookup changed indexing")
	}
	sprite.SetGraphicEffect(GhostEffect, 20)
	sprite.ChangeGraphicEffect(GhostEffect, 30)
	if sprite.graphicEffects[GhostEffect] != 50 {
		t.Fatalf("ghost effect = %v, want 50", sprite.graphicEffects[GhostEffect])
	}
	sprite.ClearGraphicEffects()
	if sprite.graphicEffects[GhostEffect] != 0 {
		t.Fatalf("effects remained after clear: %v", sprite.graphicEffects)
	}
	sprite.Show()
	sprite.Say__0("visible bubble")
	sprite.Hide()
	if sprite.Visible() || sprite.bubble().textObj != nil {
		t.Fatal("hiding sprite did not hide and clear its bubble")
	}
	sprite.SetLayer__0(Front)
	if game.scene.findShapeIndex(&sprite.SpriteImpl) != len(game.scene.items)-1 {
		t.Fatal("front layer overload did not place sprite last")
	}
}

func TestSpriteGhostFadeOutAfterExtendedFadeIn(t *testing.T) {
	// CoalDestroyer's Play button keeps fading in until clicked, then fades out.
	// See https://github.com/goplus/sb2xbp/issues/860.
	var sprite SpriteImpl
	sprite.SetGraphicEffect(GhostEffect, 100)
	for range 60 {
		sprite.ChangeGraphicEffect(GhostEffect, -5)
	}
	if got := sprite.graphicEffects[GhostEffect]; got != 0 {
		t.Errorf("ghost after extended fade-in = %v, want 0", got)
	}
	for range 20 {
		sprite.ChangeGraphicEffect(GhostEffect, 5)
	}
	if !sprite.isFullyGhosted() {
		t.Fatalf("sprite remains visible after fade-out: ghost = %v", sprite.graphicEffects[GhostEffect])
	}
}

func TestAnimationPayloadBoundsError(t *testing.T) {
	tests := []struct {
		name     string
		from, to int
		costumes []*costume
		want     string
	}{
		{"empty", -1, 0, nil, "createAnimation: no costumes configured"},
		{"negative from", -1, 0, []*costume{{}}, "createAnimation: frame index out of bounds (from=-1, to=0, costumes=1)"},
		{"negative to", 0, -1, []*costume{{}}, "createAnimation: frame index out of bounds (from=0, to=-1, costumes=1)"},
		{"from past end", 1, 0, []*costume{{}}, "createAnimation: frame index out of bounds (from=1, to=0, costumes=1)"},
		{"to past end", 0, 1, []*costume{{}}, "createAnimation: frame index out of bounds (from=0, to=1, costumes=1)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, isAtlas := range []bool{false, true} {
				got, bitmap, err := buildAnimationPayloadJSON(test.costumes, test.from, test.to, isAtlas)
				if err == nil || err.Error() != test.want || got != "" || bitmap != 0 {
					t.Fatalf("buildAnimationPayloadJSON(atlas=%v) = (%q, %d, %v), want empty/0/%q", isAtlas, got, bitmap, err, test.want)
				}
			}
		})
	}
}

func TestAnimationPayloadAtlasIgnoresNormalMetadata(t *testing.T) {
	costumes := []*costume{
		{path: "sprites/base.png", frame: costumeFrame{Width: 90, Height: 100}},
		{
			path: "sprites/other.png",
			frame: costumeFrame{
				PosX: 10, PosY: 20, Width: 30, Height: 40,
				BitmapResolution: 8, Center: mathf.NewVec2(math.NaN(), 0), ImageSize: mathf.NewVec2(0, math.Inf(1)),
			},
		},
		{path: "sprites/another.png", frame: costumeFrame{PosX: 50, PosY: 60, Width: 70, Height: 80}},
	}
	const base = `{"x":0,"y":0,"w":90,"h":100,"offset":[0,0]}`
	const first = `{"x":10,"y":20,"w":30,"h":40,"offset":[0,0]}`
	const second = `{"x":50,"y":60,"w":70,"h":80,"offset":[0,0]}`
	tests := []struct {
		name     string
		from, to int
		frames   string
	}{
		{"single", 1, 1, "[" + first + "]"},
		{"forward", 1, 2, "[" + first + "," + second + "]"},
		{"reverse", 2, 1, "[" + second + "," + first + "]"},
		{"reverse through zero", 2, 0, "[" + second + "," + first + "," + base + "]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := buildAnimationPayloadJSON(costumes, test.from, test.to, true)
			want := `{"base_path":` + strconv.Quote(engine.ToAssetPath(costumes[0].path)) + `,"frames":` + test.frames + `,"max_bitmap":1}`
			if err != nil || got != want || bitmap != 1 {
				t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want (%q, 1, nil)", got, bitmap, err, want)
			}
		})
	}
}

func TestAnimationPayloadRejectsNonFiniteNormalOffset(t *testing.T) {
	got, bitmap, err := buildAnimationPayloadJSON([]*costume{{frame: costumeFrame{Center: mathf.NewVec2(math.NaN(), 0)}}}, 0, 0, false)
	var unsupported *json.UnsupportedValueError
	if !errors.As(err, &unsupported) || !strings.HasPrefix(err.Error(), "createAnimation: failed to marshal animation payload:") || got != "" || bitmap != 0 {
		t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want empty/0/wrapped JSON error", got, bitmap, err)
	}
}

func TestAnimationPayloadNormalPreservesDescriptorGeometry(t *testing.T) {
	costumes := []*costume{
		{
			path: "sprites/fractional.svg",
			frame: costumeFrame{
				ImageSize: mathf.NewVec2(55.75, 54.25), Center: mathf.NewVec2(52, 89),
				// Normal payloads use logical ImageSize, not atlas-only geometry.
				Width: 999, Height: 999, PosX: 10, PosY: 20,
				AtlasUVRect: mathf.NewVec4(math.NaN(), 0, 0, 0),
			},
		},
		{
			path: "sprites/centered.png",
			frame: costumeFrame{
				BitmapResolution: 2, ImageSize: mathf.NewVec2(20, 30), Center: mathf.NewVec2(10, 15),
			},
		},
	}
	first := `{"path":` + strconv.Quote(engine.ToAssetPath(costumes[0].path)) + `,"offset":[24.125,-61.875],"bitmap":1}`
	second := `{"path":` + strconv.Quote(engine.ToAssetPath(costumes[1].path)) + `,"offset":[0,-0],"bitmap":2}`
	for _, test := range []struct {
		name     string
		from, to int
		frames   string
		bitmap   int
	}{
		{"single", 0, 0, "[" + first + "]", 1},
		{"forward", 0, 1, "[" + first + "," + second + "]", 2},
		{"reverse", 1, 0, "[" + second + "," + first + "]", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, bitmap, err := buildAnimationPayloadJSON(costumes, test.from, test.to, false)
			want := `{"frames":` + test.frames + `,"max_bitmap":` + strconv.Itoa(test.bitmap) + `}`
			if err != nil || got != want || bitmap != test.bitmap {
				t.Fatalf("buildAnimationPayloadJSON() = (%q, %d, %v), want (%q, %d, nil)", got, bitmap, err, want, test.bitmap)
			}
		})
	}
}

func (*shiftedSprite) Main() {}

func (*layoutExtraBaseSprite) Main() {}

func (m *colliderSpy) ClearColliderShape(_ pkgengine.Object, trigger bool) {
	m.clearedShapes = append(m.clearedShapes, trigger)
}

func (m *colliderSpy) SetCollisionLayer(_ pkgengine.Object, layer int64) {
	m.collisionLayer = layer
	m.collisionFilterWrites++
}

func (m *colliderSpy) SetCollisionMask(_ pkgengine.Object, mask int64) {
	m.collisionMask = mask
	m.collisionFilterWrites++
}

func (m *colliderSpy) SetTriggerLayer(_ pkgengine.Object, layer int64) {
	m.triggerLayer = layer
	m.triggerLayerWrites++
}

func (m *colliderSpy) SetTriggerMask(_ pkgengine.Object, mask int64) {
	m.triggerMask = mask
	m.triggerMaskWrites++
}

func (m *colliderSpy) SetGravityScale(pkgengine.Object, float64) {
	m.physicsWrites++
}

func (m *colliderSpy) SetPhysicsMode(pkgengine.Object, int64) {
	m.physicsWrites++
}

func (m *colliderSpy) SetColliderPolygon(_ pkgengine.Object, center mathf.Vec2, points pkgengine.Array) {
	m.collisionCenter = center
	m.collisionPoints = slices.Clone(points.([]float32))
}

func (m *colliderSpy) SetCollisionEnabled(_ pkgengine.Object, enabled bool) {
	m.collisionEnabled = append(m.collisionEnabled, enabled)
}

func (m *colliderSpy) SetTriggerPolygon(_ pkgengine.Object, center mathf.Vec2, points pkgengine.Array) {
	m.triggerCenter = center
	m.triggerPoints = slices.Clone(points.([]float32))
}

func (m *colliderSpy) SetTriggerEnabled(_ pkgengine.Object, enabled bool) {
	m.triggerEnabled = append(m.triggerEnabled, enabled)
}

func (m *colliderSpy) SetTriggerRect(_ pkgengine.Object, center mathf.Vec2, size mathf.Vec2) {
	m.rectCalls++
	m.rectCenter, m.rectSize, m.rectIsTrigger = center, size, true
}

func (m *colliderSpy) SetColliderRect(_ pkgengine.Object, center mathf.Vec2, size mathf.Vec2) {
	m.rectCalls++
	m.rectCenter, m.rectSize, m.rectIsTrigger = center, size, false
}

func (m *spritePhysicsAPIManager) GetVelocity(pkgengine.Object) mathf.Vec2 { return m.velocity }

func (m *spritePhysicsAPIManager) SetVelocity(_ pkgengine.Object, v mathf.Vec2) { m.velocity = v }

func (m *spritePhysicsAPIManager) AddImpulse(_ pkgengine.Object, v mathf.Vec2) { m.impulse = v }

func (m *spritePhysicsAPIManager) GetGravity(pkgengine.Object) float64 { return m.gravity }

func (m *spritePhysicsAPIManager) SetGravity(_ pkgengine.Object, v float64) { m.gravity = v }

func (m *spritePhysicsAPIManager) IsOnFloor(pkgengine.Object) bool { return m.floor }

func (m *spritePhysicsAPIManager) GetCollisionLayer(pkgengine.Object) int64 { return m.collisionLayer }

func (m *spritePhysicsAPIManager) GetCollisionMask(pkgengine.Object) int64 { return m.collisionMask }

func (m *spritePhysicsAPIManager) IsCollisionEnabled(pkgengine.Object) bool {
	return m.collisionEnabled[len(m.collisionEnabled)-1]
}

func (m *spritePhysicsAPIManager) GetTriggerLayer(pkgengine.Object) int64 { return m.triggerLayer }

func (m *spritePhysicsAPIManager) GetTriggerMask(pkgengine.Object) int64 { return m.triggerMask }

func (m *spritePhysicsAPIManager) IsTriggerEnabled(pkgengine.Object) bool {
	return m.triggerEnabled[len(m.triggerEnabled)-1]
}

func (m *spritePhysicsAPIManager) SetColliderCircle(_ pkgengine.Object, c mathf.Vec2, r float64) {
	m.shapeCenter = c
	m.circleRadius = r
	m.shapeTrigger = false
}

func (m *spritePhysicsAPIManager) SetTriggerCircle(_ pkgengine.Object, c mathf.Vec2, r float64) {
	m.shapeCenter = c
	m.circleRadius = r
	m.shapeTrigger = true
}

func (m *spritePhysicsAPIManager) SetColliderCapsule(_ pkgengine.Object, c, s mathf.Vec2) {
	m.shapeCenter = c
	m.capsuleSize = s
	m.shapeTrigger = false
}

func (m *spritePhysicsAPIManager) SetTriggerCapsule(_ pkgengine.Object, c, s mathf.Vec2) {
	m.shapeCenter = c
	m.capsuleSize = s
	m.shapeTrigger = true
}

func (f *fakeEdgePhysicsMgr) Raycast(from, to mathf.Vec2, collisionMask int64) pkgengine.Object {
	return 0
}

func (f *fakeEdgePhysicsMgr) CheckCollision(from, to mathf.Vec2, collisionMask int64, collideWithAreas, collideWithBodies bool) bool {
	return false
}

func (f *fakeEdgePhysicsMgr) CheckTouchedCameraBoundaries(obj pkgengine.Object) int64 {
	return f.cameraTouching
}

func (f *fakeEdgePhysicsMgr) CheckTouchedCameraBoundary(obj pkgengine.Object, boardType int64) bool {
	return f.cameraTouching&boardType != 0
}

func (f *fakeEdgePhysicsMgr) CheckNearestTouchedCameraBoundary(obj pkgengine.Object) int64 {
	return f.cameraNearest
}

func (f *fakeEdgePhysicsMgr) CheckTouchedStageBoundaries(obj pkgengine.Object) int64 {
	return f.stageTouching
}

func (f *fakeEdgePhysicsMgr) CheckTouchedStageBoundary(obj pkgengine.Object, boardType int64) bool {
	return f.stageTouching&boardType != 0
}

func (f *fakeEdgePhysicsMgr) CheckNearestTouchedStageBoundary(obj pkgengine.Object) int64 {
	return f.stageNearest
}

func (f *fakeEdgePhysicsMgr) SetCollisionSystemType(bool) {}

func (f *fakeEdgePhysicsMgr) SetGlobalGravity(float64) {}

func (f *fakeEdgePhysicsMgr) GetGlobalGravity() float64 {
	return 0
}

func (f *fakeEdgePhysicsMgr) SetGlobalFriction(float64) {}

func (f *fakeEdgePhysicsMgr) GetGlobalFriction() float64 {
	return 0
}

func (f *fakeEdgePhysicsMgr) SetGlobalAirDrag(float64) {}

func (f *fakeEdgePhysicsMgr) GetGlobalAirDrag() float64 {
	return 0
}

func (f *fakeEdgePhysicsMgr) CheckCollisionRect(pos, size mathf.Vec2, collisionMask int64) pkgengine.Array {
	return nil
}

func (f *fakeEdgePhysicsMgr) CheckCollisionCircle(pos mathf.Vec2, radius float64, collisionMask int64) pkgengine.Array {
	return nil
}

func (f *fakeEdgePhysicsMgr) RaycastWithDetails(from, to mathf.Vec2, ignoreSprites pkgengine.Array, collisionMask int64, collideWithAreas, collideWithBodies bool) pkgengine.Array {
	return nil
}

func newTestTransformSprite(x, y float64) *SpriteImpl {
	sprite := &SpriteImpl{
		g:    &Game{},
		name: "TestSprite",
	}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		X:             x,
		Y:             y,
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	return sprite
}

func newColliderTestSprite(t *testing.T) (*SpriteImpl, *colliderSpy) {
	t.Helper()
	original := pkgengine.SpriteMgr
	mgr := &colliderSpy{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})

	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.scale = 2
	sprite.runtimeState.proxy = &engine.Sprite{
		Sprite: pkgengine.Sprite{Id: 1},
	}
	return sprite, mgr
}

func assertColliderParams(t *testing.T, sprite *SpriteImpl, isTrigger bool, wantType ColliderShapeType, want []float64) {
	t.Helper()
	typ, got := sprite.ColliderShape(isTrigger)
	if typ != wantType || !slices.Equal(got, want) {
		t.Fatalf("shape(trigger=%t) = type %d params %v, want type %d params %v", isTrigger, typ, got, wantType, want)
	}
}

func installEdgePhysicsMgr(t *testing.T, mgr pkgengine.IPhysicsMgr) {
	t.Helper()

	setupImmediateEngineCalls(t)

	original := pkgengine.PhysicsMgr
	pkgengine.PhysicsMgr = mgr
	t.Cleanup(func() {
		pkgengine.PhysicsMgr = original
	})
}

func newEdgeTestSprite(direction Direction) *SpriteImpl {
	sprite := newTestTransformSprite(0, 0)
	sprite.spriteState.isVisible = true
	sprite.transform().direction = direction
	sprite.runtimeState.proxy = &engine.Sprite{}
	sprite.runtimeState.proxy.SetId(42)
	return sprite
}

func newTestRenderSprite() *SpriteImpl {
	return &SpriteImpl{
		name: "TestSprite",
		visualObject: visualObject{
			costumes: []*costume{
				{name: "idle"},
				{name: "run"},
			},
			costumeIndex: 1,
		},
	}
}

func newRenderOffsetTestSprite() *SpriteImpl {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.runtimeState.scale = 1
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.costumes = []*costume{{
		atlas: true,
		frame: costumeFrame{
			Center:           mathf.NewVec2(10, 20),
			BitmapResolution: 1,
			Width:            100,
			Height:           80,
		},
	}}
	return sprite
}
