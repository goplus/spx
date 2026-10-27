//go:build !js

package spx

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	itime "github.com/goplus/spx/v4/internal/time"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type animationLifecycleSpriteMgr struct {
	pkgengine.ISpriteMgr
	playing  bool
	velocity mathf.Vec2
	pauses   int
	play     func()
}

type animationRegistrationResMgr struct {
	pkgengine.IResMgr
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func TestAnimationCancellationCleansNativeResources(t *testing.T) {
	for _, blocking := range []bool{false, true} {
		for _, shutdown := range []bool{false, true} {
			name := "tween"
			if blocking {
				name = "blocking"
			}
			if shutdown {
				name += "/shutdown"
			}
			t.Run(name, func(t *testing.T) {
				co, anim, sprites, audio := setupAnimationLifecycle(t)
				continued := false
				thread := co.Create(anim.sprite, func(coroutine.Thread) {
					if blocking {
						anim.sprite.AnimateAndWait("walk")
					} else {
						anim.doTween("walk", anim.shared.animations["walk"].config, tweenParams{
							aniType: coreproject.AniTypeMove, duration: 10, moveTo: mathf.NewVec2(100, 0),
						})
					}
					continued = true
				})
				co.Update()
				state := anim.curAnimState
				if !blocking {
					state = anim.getCurTweenState()
				}
				if state == nil || state.OnPlayAudioPlaybackID == 0 {
					t.Fatal("playback did not reach its first wait")
				}
				id := state.OnPlayAudioPlaybackID
				state.OnPlayAudioRestartPending = true
				if shutdown {
					co.RunAfterStop(func() {
						if audio.playing[id] || sprites.velocity != (mathf.Vec2{}) {
							t.Error("shutdown callback ran before resource cleanup")
						}
					})
				} else {
					stopper := co.Create(anim.sprite, func(coroutine.Thread) { anim.sprite.Stop(OtherScriptsInSprite) })
					co.Update()
					co.Join(stopper)
				}
				joinAnimationThread(t, co, thread)
				if continued || !state.IsCanceled || state.OnPlayAudioRestartPending || state.OnPlayAudioPlaybackID != 0 {
					t.Fatalf("canceled playback continued=%v, state=%+v", continued, state)
				}
				if anim.sprite.isDestroyed() || anim.sprite.runtimeState.IsAnimating || len(anim.activeTweenStates) != 0 || anim.curAnimState != nil {
					t.Fatal("cancellation left animation state or destroyed the sprite")
				}
				if audio.playing[id] || len(audio.stops) != 1 || audio.stops[0] != id || sprites.velocity != (mathf.Vec2{}) {
					t.Fatalf("resources after cancellation: stops=%v, velocity=%v", audio.stops, sprites.velocity)
				}
				if sprites.playing || sprites.pauses != 1 {
					t.Fatal("canceled playback was not paused")
				}
			})
		}
	}
}

func TestAnimationInitializationFailureReleasesAudio(t *testing.T) {
	co, anim, sprites, audio := setupAnimationLifecycle(t)
	failure := &struct{ message string }{"playback initialization failed"}
	sprites.play = func() { panic(failure) }
	var recovered any
	thread := co.Create(anim.sprite, func(coroutine.Thread) {
		defer func() { recovered = recover() }()
		anim.sprite.AnimateAndWait("walk")
	})
	co.Update()
	joinAnimationThread(t, co, thread)
	if recovered != failure {
		t.Fatalf("panic = %v, want original failure", recovered)
	}
	if anim.curAnimState != nil || anim.sprite.runtimeState.IsAnimating || len(audio.stops) != 1 || audio.playing[1] {
		t.Fatalf("failed initialization left playback=%v, audio stops=%v", anim.curAnimState, audio.stops)
	}
}

func TestCanceledTweenDoesNotStopReplacementVelocity(t *testing.T) {
	co, anim, sprites, _ := setupAnimationLifecycle(t)
	start := func(distance float64) coroutine.Thread {
		thread := co.Create(anim.sprite, func(coroutine.Thread) {
			anim.doTween("missing", nil, tweenParams{aniType: coreproject.AniTypeMove, duration: 10, moveTo: mathf.NewVec2(distance, 0)})
		})
		co.Update()
		return thread
	}
	first := start(100)
	second := start(200)
	co.Stop(first)
	co.Update()
	joinAnimationThread(t, co, first)
	if sprites.velocity.X != 20 || len(anim.activeTweenStates) != 1 {
		t.Fatalf("old tween changed replacement velocity: %v", sprites.velocity)
	}
	co.Stop(second)
	co.Update()
	joinAnimationThread(t, co, second)
	if sprites.velocity != (mathf.Vec2{}) {
		t.Fatalf("final velocity = %v", sprites.velocity)
	}
}

func TestAnimationCleanupSkipsDestroyedProxy(t *testing.T) {
	co, anim, sprites, audio := setupAnimationLifecycle(t)
	thread := co.Create(anim.sprite, func(coroutine.Thread) { anim.sprite.AnimateAndWait("walk") })
	co.Update()
	state := anim.curAnimState
	anim.sprite.markDestroyed()
	co.Stop(thread)
	joinAnimationThread(t, co, thread)
	if sprites.pauses != 0 {
		t.Fatal("cleanup called the destroyed proxy")
	}
	if !state.IsCanceled || state.OnPlayAudioPlaybackID != 0 || audio.playing[1] || anim.sprite.runtimeState.IsAnimating {
		t.Fatal("destroyed playback retained local state or audio")
	}
}

func TestBlockingAnimationCancellationPreservesReplacement(t *testing.T) {
	co, anim, sprites, _ := setupAnimationLifecycle(t)
	first := co.Create(anim.sprite, func(coroutine.Thread) { anim.sprite.AnimateAndWait("walk") })
	co.Update()
	second := co.Create(anim.sprite, func(coroutine.Thread) { anim.sprite.AnimateAndWait("walk") })
	co.Update()
	replacement := anim.curAnimState
	co.Stop(first)
	joinAnimationThread(t, co, first)
	if anim.curAnimState != replacement || replacement.IsCanceled || !anim.sprite.runtimeState.IsAnimating || !sprites.playing {
		t.Fatal("old waiter changed replacement playback")
	}
	co.Stop(second)
	joinAnimationThread(t, co, second)
}

func TestAnimationCancellationDuringInitializationReleasesCapturedAudio(t *testing.T) {
	co, anim, sprites, audio := setupAnimationLifecycle(t)
	continued := false
	thread := co.Create(anim.sprite, func(me coroutine.Thread) {
		// Cancel while the engine call is running, after it acquired audio.
		sprites.play = func() { co.Stop(me) }
		anim.sprite.AnimateAndWait("walk")
		continued = true
	})
	joinAnimationThread(t, co, thread)
	if continued || anim.curAnimState != nil || len(audio.stops) != 1 || audio.playing[1] || sprites.playing {
		t.Fatalf("canceled initialization continued=%v, state=%v, stops=%v", continued, anim.curAnimState, audio.stops)
	}
}

// TestAnimationConditionReplacementSurvivesQueuedCompletion exercises the native
// scheduling phases rather than assigning curAnimState directly.
func TestAnimationConditionReplacementSurvivesQueuedCompletion(t *testing.T) {
	co, anim, _, audio := setupAnimationLifecycle(t)
	itime.ClearTimers()
	itime.Start(nil)
	t.Cleanup(itime.ClearTimers)
	game := anim.sprite.g
	game.bindScriptEvents()
	game.shapeMgr.items = []Shape{anim.sprite}
	if !co.TryRunFromEngine(anim.sprite, func() { anim.sprite.AnimateWith("walk", false) }) {
		t.Fatal("fixture did not use native engine dispatch")
	}
	old := anim.curAnimState
	if old == nil {
		t.Fatal("original public playback did not start")
	}

	// This is the existing logic loop's WaitNextFrame -> processLogicFrame order.
	logic := co.Create(game, func(coroutine.Thread) {
		engine.WaitNextFrame()
		game.processLogicFrame(nil, nil)
	})
	co.JoinFirstSlice(logic)
	anim.sprite.handleAnimationFinished()

	var replacement *animState
	var playbackID int64
	game.OnCond(func() bool { return true }, func() {
		anim.sprite.AnimateWith("walk", false)
		replacement = anim.curAnimState
		if replacement != nil {
			playbackID = replacement.OnPlayAudioPlaybackID
		}
	})
	// Actual engine phase order: sample, advance clock, dispatch condition,
	// then update coroutine jobs including the waiting logic loop.
	co.RunBetweenScripts(game.scriptEvents.sampleConditions)
	itime.Update(1.0/60.0, 60)
	withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
	co.Update()
	co.Join(logic)
	if replacement == nil || replacement == old {
		t.Fatalf("condition did not retain a distinct playback: old=%p replacement=%p current=%p", old, replacement, anim.curAnimState)
	}
	if anim.curAnimState != replacement || replacement.IsCanceled || !audio.playing[playbackID] {
		t.Fatalf("completion canceled OnCond same-name replacement: current=%p replacement=%p canceled=%v audioPlaying=%v", anim.curAnimState, replacement, replacement.IsCanceled, audio.playing[playbackID])
	}
}

func TestClonedAnimationEntryRegistersOnceConcurrently(t *testing.T) {
	setupImmediateEngineCalls(t)
	resources := &animationRegistrationResMgr{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	originalResMgr := pkgengine.ResMgr
	pkgengine.ResMgr = resources
	t.Cleanup(func() { pkgengine.ResMgr = originalResMgr })

	frame := newCostumeWithSize(1, 1)
	frame.frame.BitmapResolution = 4
	frame.path = "walk.png"
	sprite := &SpriteImpl{name: "TestSprite"}
	sprite.costumes = []*costume{frame}
	original := &animationComponent{}
	prepared, err := prepareSpriteConfig(&coreproject.SpriteConfig{
		Costumes: []*coreproject.CostumeConfig{{Name: "default"}},
		FAnimations: map[string]*coreproject.AniConfig{
			"walk": {FrameFrom: 0, FrameTo: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	original.initialize(sprite, &prepared.config)
	clone := original.cloneFor(&SpriteImpl{name: sprite.name})

	originalEntry := original.shared.animations["walk"]
	cloneEntry := clone.shared.animations["walk"]
	if originalEntry != cloneEntry {
		t.Fatal("clone did not share the original animation entry")
	}

	start := make(chan struct{})
	started := make(chan struct{}, 2)
	results := make(chan int, 2)
	for _, entry := range []*animationEntry{originalEntry, cloneEntry} {
		go func() {
			<-start
			started <- struct{}{}
			results <- entry.ensureRegistered()
		}()
	}
	close(start)
	<-started
	<-started
	<-resources.entered
	close(resources.release)

	for range 2 {
		if got := <-results; got != 4 {
			t.Errorf("bitmap resolution = %d, want 4", got)
		}
	}
	if got := resources.calls.Load(); got != 1 {
		t.Fatalf("animation registrations = %d, want 1", got)
	}
}

func TestStopAnimationOnlyStopsMatchingLivePlayback(t *testing.T) {
	for _, mode := range []string{"empty", "unknown", "not playing", "other animation", "unbound", "matching"} {
		t.Run(mode, func(t *testing.T) {
			_, anim, sprites, _ := setupAnimationLifecycle(t)
			state := &animState{Name: "walk"}
			anim.curAnimState = state
			name := "walk"
			switch mode {
			case "empty":
				name = ""
			case "unknown":
				name = "missing"
			case "not playing":
				anim.curAnimState = nil
			case "other animation":
				state.Name = "other"
			case "unbound":
				anim.sprite.runtimeState.SyncSprite = nil
			}
			anim.stopAnimation(name)
			if mode == "matching" {
				if !state.IsCanceled || anim.curAnimState != nil || sprites.pauses != 1 {
					t.Fatal("matching animation was not canceled and paused")
				}
			} else if state.IsCanceled || sprites.pauses != 0 {
				t.Fatal("unrelated or unbound animation was paused")
			}
		})
	}
}

func TestDefaultAnimationPlaybackAndCancellationDuringStart(t *testing.T) {
	co, anim, sprites, _ := setupAnimationLifecycle(t)
	anim.sprite.spriteState.IsVisible = true
	anim.shared.defaultAnimation = "walk"
	anim.playDefaultAnim()
	if !anim.defaultAnimActive || !sprites.playing {
		t.Fatal("configured default animation did not start")
	}
	anim.sprite.spriteState.IsVisible = false
	sprites.play = func() { anim.cancelAnimState(anim.curAnimState) }
	finished := false
	thread := co.Create(anim.sprite, func(coroutine.Thread) {
		anim.playAnimation("walk", false, true)
		finished = true
	})
	co.Update()
	joinAnimationThread(t, co, thread)
	if !finished || anim.curAnimState == nil || !anim.curAnimState.IsCanceled || anim.sprite.runtimeState.IsAnimating {
		t.Fatal("cancellation during start did not release the blocking animation")
	}
}

func (m *animationLifecycleSpriteMgr) PlayAnim(engine.Object, string, float64, bool, bool) {
	m.playing = true
	if m.play != nil {
		m.play()
	}
}

func (m *animationLifecycleSpriteMgr) IsPlayingAnim(engine.Object) bool { return m.playing }

func (m *animationLifecycleSpriteMgr) PauseAnim(engine.Object) { m.playing = false; m.pauses++ }

func (m *animationLifecycleSpriteMgr) SetRenderScale(engine.Object, mathf.Vec2) {}

func (m *animationLifecycleSpriteMgr) SetVelocity(_ engine.Object, velocity mathf.Vec2) {
	m.velocity = velocity
}

func (m *animationRegistrationResMgr) CreateAnimation(string, string, string, int64, bool) {
	if m.calls.Add(1) == 1 {
		close(m.entered)
	}
	<-m.release
}

func setupAnimationLifecycle(t *testing.T) (*coroutine.Coroutines, *animationComponent, *animationLifecycleSpriteMgr, *animationAudioBackend) {
	t.Helper()
	platform := &eventDispatchPlatform{}
	platform.useCurrentAsMainThread()
	originalPlatform, originalSprites := pkgengine.PlatformMgr, pkgengine.SpriteMgr
	sprites := &animationLifecycleSpriteMgr{}
	pkgengine.PlatformMgr, pkgengine.SpriteMgr = platform, sprites
	t.Cleanup(func() { pkgengine.PlatformMgr, pkgengine.SpriteMgr = originalPlatform, originalSprites })
	co := setupRuntimeScheduler(t)
	enginewrap.Init(engine.WaitMainThread)
	anim := newTestAnimationComponent()
	anim.sprite.scriptEventBindings.bind(&scriptEventRegistry{}, anim.sprite)
	anim.sprite.spriteState.IsVisible = false
	anim.sprite.g.physicsEnabled = true
	anim.sprite.physics().physicsMode = DynamicPhysics
	initTestMotionComponents(anim.sprite, 0, 0)
	audio := &animationAudioBackend{}
	initTestAnimationAudio(anim, audio)
	entry := &animationEntry{name: "walk", config: &coreproject.AniConfig{OnPlay: &coreproject.ActionConfig{Play: "walk"}}}
	entry.loadOnce.Do(func() {})
	anim.shared.animations[entry.name] = entry
	return co, anim, sprites, audio
}

func joinAnimationThread(t *testing.T, co *coroutine.Coroutines, thread coroutine.Thread) {
	t.Helper()
	done := make(chan struct{})
	go func() { co.Join(thread); close(done) }()
	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("animation thread did not finish")
		}
		co.Update()
	}
}
