//go:build !js

package spx

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type spineTestSpriteMgr struct {
	unsupported bool
	missing     string
	animationLifecycleSpriteMgr
	names  []string
	loops  []bool
	scales []mathf.Vec2
	bounds mathf.Rect2
	loaded bool
	mixes  []float64
	paths  []string
}

func (m *spineTestSpriteMgr) PlayAnim(obj engine.Object, name string, speed float64, loop, reverse bool) {
	m.names = append(m.names, name)
	m.loops = append(m.loops, loop)
	m.animationLifecycleSpriteMgr.PlayAnim(obj, name, speed, loop, reverse)
}
func (m *spineTestSpriteMgr) SetRenderScale(_ engine.Object, scale mathf.Vec2) {
	m.scales = append(m.scales, scale)
}
func (m *spineTestSpriteMgr) SetSpineSkeleton(_ engine.Object, atlas, skeleton string, mix float64) bool {
	m.paths = append(m.paths, atlas, skeleton)
	m.mixes = append(m.mixes, mix)
	return m.loaded
}
func (m *spineTestSpriteMgr) IsSpineSupported() bool { return !m.unsupported }
func (m *spineTestSpriteMgr) HasSpineAnimation(_ engine.Object, name string) bool {
	return name != m.missing
}
func (m *spineTestSpriteMgr) GetSpineBounds(engine.Object) mathf.Rect2 { return m.bounds }
func (m *spineTestSpriteMgr) SetTexture(engine.Object, string)         { panic("Spine loaded a frame texture") }
func (m *spineTestSpriteMgr) SetTextureAtlas(engine.Object, string, mathf.Rect2) {
	panic("Spine loaded an atlas frame")
}

func spineTestConfig(t *testing.T) *coreproject.SpriteConfig {
	t.Helper()
	prepared, err := prepareSpriteConfig(&coreproject.SpriteConfig{
		SpineAnim: &coreproject.SpineAnimConfig{Atlas: "sprites/Hero/hero.atlas", Skeleton: "sprites/Hero/hero.skel", DefaultMix: .2,
			StepDuration: .04, TurnToDuration: 2, AnimMap: map[string]string{"walk": "run", "idle": "stand", "turn": "turn"}},
		FAnimations:      map[string]*coreproject.AniConfig{"wave": {StepDuration: .09}},
		DefaultAnimation: "idle",
	})
	if err != nil {
		t.Fatal(err)
	}
	return &prepared.config
}

func TestSpineAnimationEntriesAndClone(t *testing.T) {
	config := spineTestConfig(t)
	anim := newTestAnimationComponent()
	anim.initialize(anim.sprite, config)
	walk, ok := anim.getAnimation("walk")
	if !ok || walk.StepDuration != .04 || walk.TurnToDuration != 2 {
		t.Fatalf("walk config = %+v, %v", walk, ok)
	}
	wave, ok := anim.getAnimation("wave")
	if !ok || wave.StepDuration != .09 {
		t.Fatalf("wave config = %+v, %v", wave, ok)
	}
	if anim.playbackName("walk") != "run" || anim.playbackName("wave") != "wave" || anim.hasAnim("missing") {
		t.Fatal("incorrect Spine alias lookup")
	}
	anim.curAnimState = &animState{Name: "walk"}
	anim.spineBounds = mathf.Rect2{Size: mathf.NewVec2(12, 20)}
	clone := anim.cloneFor(&SpriteImpl{})
	if clone.shared != anim.shared || clone.curAnimState != nil || clone.spineBounds != (mathf.Rect2{}) {
		t.Fatal("clone did not isolate runtime state")
	}
}

func TestSpinePlaybackAndDefaultUseNativeNames(t *testing.T) {
	_, anim, _, _ := setupAnimationLifecycle(t)
	mgr := &spineTestSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	anim.initialize(anim.sprite, spineTestConfig(t))
	anim.sprite.runtimeState.scale = .25
	anim.sprite.runtimeState.isCostumeDirty = true
	anim.sprite.AnimateWith("walk", true)
	if len(mgr.names) != 1 || mgr.names[0] != "run" || !mgr.loops[0] {
		t.Fatalf("playback = %v %v", mgr.names, mgr.loops)
	}
	if len(mgr.scales) == 0 || mgr.scales[0] != engine.UniformVec2(.25) || anim.sprite.runtimeState.isCostumeDirty {
		t.Fatal("Spine scale was not synchronized")
	}
	anim.sprite.spriteState.isVisible = true
	anim.sprite.StopAnimation("walk")
	if mgr.pauses != 1 || len(mgr.names) != 2 || mgr.names[1] != "stand" {
		t.Fatalf("stop/default = %d %v", mgr.pauses, mgr.names)
	}
	anim.sprite.AnimateWith("missing", false)
	if len(mgr.names) != 2 {
		t.Fatal("missing animation started playback")
	}
}

func TestSpineWaitAndTweenCancellation(t *testing.T) {
	for _, tween := range []bool{false, true} {
		t.Run(map[bool]string{false: "wait", true: "step"}[tween], func(t *testing.T) {
			current, anim, _, _ := setupAnimationLifecycle(t)
			mgr := &spineTestSpriteMgr{}
			pkgengine.SpriteMgr = mgr
			anim.initialize(anim.sprite, spineTestConfig(t))
			continued := false
			thread := current.Create(anim.sprite, func(coroutine.Thread) {
				if tween {
					anim.sprite.StepTo__b(100, 0, 1, "walk")
				} else {
					anim.sprite.AnimateAndWait("walk")
				}
				continued = true
			})
			current.Update()
			if len(mgr.names) != 1 || mgr.names[0] != "run" || anim.curAnimState == nil {
				t.Fatal("Spine playback did not reach wait")
			}
			current.Stop(thread)
			joinAnimationThread(t, current, thread)
			if continued || anim.curAnimState != nil || len(anim.activeTweenStates) != 0 || mgr.pauses != 1 {
				t.Fatal("Spine cancellation left playback running")
			}
		})
	}
}

func TestSpineProxySetupAndFailure(t *testing.T) {
	_, anim, _, _ := setupAnimationLifecycle(t)
	config := spineTestConfig(t)
	anim.initialize(anim.sprite, config)
	mgr := &spineTestSpriteMgr{loaded: true, bounds: mathf.Rect2{Position: mathf.NewVec2(-10, -50), Size: mathf.NewVec2(20, 50)}}
	pkgengine.SpriteMgr = mgr
	anim.sprite.runtimeState.scale = .5
	original := anim.sprite.costumes[0]
	anim.sprite.initSpineProxy()
	if len(mgr.paths) != 2 || !strings.HasSuffix(mgr.paths[0], "sprites/Hero/hero.atlas") || mgr.mixes[0] != .2 {
		t.Fatalf("load = %v %v", mgr.paths, mgr.mixes)
	}
	if anim.spineBounds != mgr.bounds || anim.sprite.costumes[0] == original || anim.sprite.currentCostume().path != "" {
		t.Fatal("skeleton geometry was not installed independently")
	}
	if w, h := anim.sprite.getCostumeSize(); w != 20 || h != 50 {
		t.Fatalf("size = %v, %v", w, h)
	}
	mgr.loaded = false
	defer func() {
		if recover() == nil {
			t.Fatal("failed skeleton load did not stop initialization")
		}
	}()
	anim.sprite.initSpineProxy()
}

func TestSpineAutoColliderKeepsBoundsAndCustomShape(t *testing.T) {
	sprite, mgr := newPolygonColliderTestSprite(t)
	sprite.components.animation = &animationComponent{sprite: sprite, shared: &sharedAnimationData{spine: &coreproject.SpineAnimConfig{}},
		spineBounds: mathf.Rect2{Position: mathf.NewVec2(-10, -50), Size: mathf.NewVec2(20, 50)}}
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	shape := &sprite.physics().collisionInfo
	shape.Type = physicsColliderAuto
	shape.syncShape(sprite.runtimeState.syncSprite, false, sprite)
	if mgr.rectSize != mathf.NewVec2(40, 100) || mgr.rectCenter != mathf.NewVec2(-6, 42) {
		t.Fatalf("auto collider = %+v %+v", mgr.rectSize, mgr.rectCenter)
	}
	shape.Type, shape.Pivot, shape.Params = physicsColliderRect, mathf.NewVec2(2, 3), []float64{7, 9}
	shape.syncShape(sprite.runtimeState.syncSprite, false, sprite)
	if mgr.rectSize != mathf.NewVec2(14, 18) || mgr.rectCenter != mathf.NewVec2(4, 6) {
		t.Fatalf("custom collider = %+v %+v", mgr.rectSize, mgr.rectCenter)
	}
}

func TestSpineRenderBoundsRespectOriginScaleAndFlip(t *testing.T) {
	sprite := newRenderOffsetTestSprite()
	sprite.runtimeState.scale = 2
	sprite.costumes = []*costume{newCostumeWithSize(20, 50)}
	sprite.components.animation = &animationComponent{sprite: sprite, shared: &sharedAnimationData{spine: &coreproject.SpineAnimConfig{}},
		spineBounds: mathf.Rect2{Position: mathf.NewVec2(10, -50), Size: mathf.NewVec2(20, 50)}}
	sprite.transform().x, sprite.transform().y = 0, 0
	sprite.transform().pivot = mathf.NewVec2(3, 4)
	sprite.transform().rotationStyle = LeftRight
	sprite.transform().direction = -90
	bounds, ok := sprite.renderBounds()
	if !ok || math.Abs(bounds.Position.X+54) > 1e-8 || math.Abs(bounds.Position.Y+8) > 1e-8 || bounds.Size != mathf.NewVec2(40, 100) {
		t.Fatalf("render bounds = %+v, %v", bounds, ok)
	}
	if got := sprite.transform().clampSpriteScale(.01); got != .01 {
		t.Fatalf("Spine size clamped to %v", got)
	}
}

func TestSpineProxyRejectsMissingAnimationBeforePlayback(t *testing.T) {
	_, anim, _, _ := setupAnimationLifecycle(t)
	anim.initialize(anim.sprite, spineTestConfig(t))
	mgr := &spineTestSpriteMgr{loaded: true, missing: "run"}
	// A rejected native play must never accidentally wait on a previous loop.
	mgr.playing = true
	pkgengine.SpriteMgr = mgr
	defer func() {
		err := recover()
		if err == nil || !strings.Contains(fmt.Sprint(err), `references missing Spine animation "run"`) {
			t.Fatalf("initialization error = %v", err)
		}
		if len(mgr.names) != 0 || anim.curAnimState != nil {
			t.Fatal("invalid skeleton animation started playback")
		}
	}()
	anim.sprite.initSpineProxy()
}

func TestSpineDisabledRuntimeReportsBuildOption(t *testing.T) {
	_, anim, _, _ := setupAnimationLifecycle(t)
	anim.initialize(anim.sprite, spineTestConfig(t))
	mgr := &spineTestSpriteMgr{unsupported: true}
	pkgengine.SpriteMgr = mgr
	defer func() {
		err := recover()
		if err == nil || !strings.Contains(fmt.Sprint(err), "SPX_SPINE=1") {
			t.Fatalf("disabled runtime error = %v", err)
		}
		if len(mgr.paths) != 0 || len(mgr.names) != 0 {
			t.Fatal("disabled runtime tried to load or play a skeleton")
		}
	}()
	anim.sprite.initSpineProxy()
}
