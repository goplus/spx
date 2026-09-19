package spx

import (
	"slices"
	"testing"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

type stopAllAudioBackend struct {
	animationAudioBackend
	stopped   int
	destroyed []engine.Object
}

func (b *stopAllAudioBackend) StopAll() {
	b.stopped++
	clear(b.playing)
}

func (b *stopAllAudioBackend) DestroyAudio(obj engine.Object) { b.destroyed = append(b.destroyed, obj) }

func TestStopAllCleansResourcesBeforeAbortingCaller(t *testing.T) {
	for _, target := range []string{"stage", "clone"} {
		t.Run(target, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			game.bindScriptEvents()
			backend := &stopAllAudioBackend{}
			game.soundMgr.Init(backend)
			source := newCloneLimitSprite(game, "source")
			source.transform().x = 42
			source.onInit = func(s *cloneLimitSprite) { s.OnKey__0(KeySpace, func() {}) }
			var clone *SpriteImpl
			doClone(source, nil, func(s *SpriteImpl) { clone = s })
			clone.sound().soundObj = 17
			playback := game.soundMgr.Play(17, "long.wav", false, false, 0, 0, 0)
			gate := gco.NewLatch()
			peerFinished := false
			peer := gco.Create(&source.SpriteImpl, func(coroutine.Thread) int {
				gate.Wait()
				peerFinished = true
				return 0
			})
			gco.JoinYieldedOrDone(peer)
			caller, stopScripts := threadObj(game), game.Stop
			if target == "clone" {
				caller, stopScripts = clone, clone.Stop
			}
			afterStop := false
			stop := gco.Create(caller, func(coroutine.Thread) int {
				stopScripts(AllStop)
				afterStop = true
				return 0
			})
			gco.Join(stop)
			gco.Join(peer)
			if afterStop || peerFinished {
				t.Fatal("stopped script continued")
			}
			if !clone.isDestroyed() || game.shapeMgr.cloneCount != 0 || game.shapeMgr.findShapeIndex(clone) >= 0 {
				t.Fatal("stop all retained clone or its quota")
			}
			if len(game.scriptEvents.manager.Snapshot(coreevent.BucketKeyPressed)) != 0 {
				t.Fatal("clone event registration survived")
			}
			if backend.IsPlaying(playback) || backend.stopped != 1 {
				t.Fatal("stop all left audio playing")
			}
			if len(backend.destroyed) != 1 || backend.destroyed[0] != 17 {
				t.Fatalf("released sound objects = %v", backend.destroyed)
			}
			if source.isDestroyed() || source.transform().x != 42 || len(game.getAllShapes()) != 1 {
				t.Fatal("stop all reset or removed original")
			}
			// Repeated stops must not release resources twice.
			again := gco.Create(game, func(coroutine.Thread) int { game.Stop(AllStop); return 0 })
			gco.Join(again)
			if game.shapeMgr.cloneCount != 0 || len(backend.destroyed) != 1 {
				t.Fatal("repeated stop duplicated cleanup")
			}
		})
	}
}

func TestStopAllCancelsWaitingCloneHandler(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "source")
	gate := gco.NewLatch()
	source.onClone = func(*cloneLimitSprite) {
		gate.Wait()
		t.Error("clone handler continued after stop all")
	}
	var clone *SpriteImpl
	parent := gco.Create(&source.SpriteImpl, func(coroutine.Thread) int {
		doClone(source, nil, func(s *SpriteImpl) { clone = s })
		source.Stop(AllStop)
		return 0
	})
	gco.Join(parent)
	gco.Update()
	if !clone.isDestroyed() || game.shapeMgr.cloneCount != 0 {
		t.Fatal("pending clone survived stop all")
	}
	flushCloneProxyUpdates(game)
	if clone.runtimeState.SyncSprite != nil {
		t.Fatal("stopped clone retained its proxy")
	}
}

func TestStopAllClearsExistingSoundEffectsWithoutAllocating(t *testing.T) {
	for _, target := range []string{"stage", "sprite"} {
		t.Run(target, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			backend := &fakeAudioBackend{pan: 0.5, pitch: 2}
			game.soundMgr.Init(backend)
			source := newCloneLimitSprite(game, "source")
			unused := newCloneLimitSprite(game, "unused")
			if target == "stage" {
				game.audioState.SoundObj = 9
			} else {
				source.sound().soundObj = 9
			}
			stop := co.Create(game, func(coroutine.Thread) int { game.Stop(AllStop); return 0 })
			co.Join(stop)
			if backend.pan != 0 || backend.pitch != 1 || backend.createCalls != 0 {
				t.Fatalf("audio after stop: %+v", backend)
			}
			if unused.sound().soundObj != 0 {
				t.Fatal("stop allocated an unused sprite's sound object")
			}
		})
	}
}

func TestStopAllRemovesEveryCloneFromMixedShapes(t *testing.T) {
	game := setupCloneLimitGame(t)
	left := newCloneLimitSprite(game, "left")
	game.addShape(&struct{}{})
	right := newCloneLimitSprite(game, "right")
	originals := slices.Clone(game.getAllShapes())
	var clones []*SpriteImpl
	for range 3 {
		for _, source := range []*cloneLimitSprite{left, right} {
			doClone(source, nil, func(clone *SpriteImpl) {
				clone.greffUniforms = map[EffectKind]float64{GhostEffect: 50}
				clones = append(clones, clone)
			})
		}
	}
	for range 2 {
		stop := gco.Create(&left.SpriteImpl, func(coroutine.Thread) int { left.Stop(AllStop); return 0 })
		gco.Join(stop)
		if !slices.Equal(game.getAllShapes(), originals) || game.shapeMgr.cloneCount != 0 {
			t.Fatal("stop retained clones or changed original shape order")
		}
		for _, clone := range clones {
			if !clone.isDestroyed() || clone.greffUniforms[GhostEffect] != 0 {
				t.Fatal("stop skipped a clone's cleanup")
			}
		}
	}
}

func TestStopAllClearsGraphicEffects(t *testing.T) {
	var g Game
	g.initShapeMgr()
	g.greffUniforms = map[EffectKind]float64{GhostEffect: 100}

	left := &SpriteImpl{g: &g, name: "left"}
	left.greffUniforms = map[EffectKind]float64{GhostEffect: 100}

	right := &SpriteImpl{g: &g, name: "right"}
	right.greffUniforms = map[EffectKind]float64{
		GhostEffect:      50,
		BrightnessEffect: 25,
	}

	g.addShape(left)
	g.addShape(right)

	g.stopAllResources()

	if got := g.greffUniforms[GhostEffect]; got != 0 {
		t.Fatalf("stage ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := left.greffUniforms[GhostEffect]; got != 0 {
		t.Fatalf("left ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := right.greffUniforms[GhostEffect]; got != 0 {
		t.Fatalf("right ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := right.greffUniforms[BrightnessEffect]; got != 0 {
		t.Fatalf("right brightness effect = %v, want 0 after stop all reset", got)
	}
}
