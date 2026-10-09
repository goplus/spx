/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestGameStartBootstrapWithoutEngineThread(t *testing.T) {
	for _, name := range []string{"empty", "tasks", "stale"} {
		t.Run(name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			previousPlatform := pkgengine.PlatformMgr
			pkgengine.PlatformMgr = nil
			t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

			generation := game.bootstrapGeneration()
			var calls atomic.Int32
			if name != "empty" {
				game.queueBootstrap(generation, func() {
					calls.Add(1)
					game.queueBootstrap(generation, func() { calls.Add(1) })
				})
			}
			if name == "stale" {
				game.resetBootstrap()
			}

			queued := make(chan struct{})
			go func() {
				game.startBootstrap(generation)
				game.startBootstrap(generation)
				close(queued)
			}()
			select {
			case <-queued:
			case <-time.After(time.Second):
				t.Fatal("bootstrap scheduling waited for an engine-thread call")
			}
			co.Update()
			if got, want := game.lifecycleState.BootstrapDone.Load(), name != "stale"; got != want {
				t.Fatalf("bootstrap done = %v, want %v", got, want)
			}
			var wantCalls int32
			if name == "tasks" {
				wantCalls = 2
			}
			if got := calls.Load(); got != wantCalls {
				t.Fatalf("bootstrap calls = %d, want %d", got, wantCalls)
			}
		})
	}
}

func TestGameRunBootstrapTasksDrainsNestedCallbacks(t *testing.T) {
	var g Game
	generation := g.bootstrapGeneration()
	var got []string

	g.queueBootstrap(generation, func() {
		got = append(got, "first")
		g.queueBootstrap(generation, func() {
			got = append(got, "nested")
		})
	})
	g.queueBootstrap(generation, func() {
		got = append(got, "second")
	})

	g.runBootstrapTasks(generation)

	want := []string{"first", "second", "nested"}
	if !slices.Equal(got, want) {
		t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
	}
}

func TestGameBootstrapCanOrderGameStartBeforeSpriteStart(t *testing.T) {
	var g Game
	generation := g.bootstrapGeneration()
	g.bindScriptEvents()

	var sprite SpriteImpl
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite)

	g.queueBootstrap(generation, func() {
		g.OnStart(func() {})
	})
	g.queueBootstrap(generation, func() {
		sprite.OnStart(func() {})
	})

	g.runBootstrapTasks(generation)

	got := g.scriptEvents.manager.Snapshot(coreevent.BucketStart)
	if len(got) != 2 {
		t.Fatalf("SnapshotStart len = %d, want 2", len(got))
	}
	if _, ok := got[0].Owner.(*Game); !ok {
		t.Fatalf("first start owner = %T, want *Game", got[0].Owner)
	}
	if _, ok := got[1].Owner.(*SpriteImpl); !ok {
		t.Fatalf("second start owner = %T, want *SpriteImpl", got[1].Owner)
	}
}

func TestGameRunBootstrapTasksStopsDrainingAfterReset(t *testing.T) {
	var g Game
	generation := g.bootstrapGeneration()
	var got []string

	if !g.queueBootstrap(generation, func() {
		got = append(got, "first")
		g.resetBootstrap()
		g.queueBootstrap(g.bootstrapGeneration(), func() {
			got = append(got, "new")
		})
	}) {
		t.Fatal("queueBootstrap rejected current generation")
	}
	if !g.queueBootstrap(generation, func() {
		got = append(got, "stale")
	}) {
		t.Fatal("queueBootstrap rejected current generation")
	}

	g.runBootstrapTasks(generation)
	if len(got) != 1 || got[0] != "first" {
		t.Fatalf("old generation drained stale tasks: got %v, want [first]", got)
	}

	g.runBootstrapTasks(g.bootstrapGeneration())
	want := []string{"first", "new"}
	if !slices.Equal(got, want) {
		t.Fatalf("current generation tasks got %v, want %v", got, want)
	}
}

func TestGameBootstrapCompletionIgnoresStaleGeneration(t *testing.T) {
	var game Game
	stale := game.bootstrapGeneration()

	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.BootstrapDone.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	game.resetBootstrap()

	if game.markGameStarted(stale) {
		t.Fatal("stale bootstrap generation was marked running")
	}
	if game.completeBootstrap(stale) {
		t.Fatal("stale bootstrap generation was marked done")
	}
	if game.markStartDispatched(stale) {
		t.Fatal("stale bootstrap generation was marked started")
	}
	if game.lifecycleState.IsRunned.Load() || game.lifecycleState.BootstrapDone.Load() || game.lifecycleState.StartDispatched.Load() {
		t.Fatal("stale generation reopened lifecycle gates")
	}
}

type bootstrapAwakeOrderSprite struct {
	SpriteImpl
	peer         *bootstrapAwakeOrderSprite
	sawSelfAwake bool
	sawPeerAwake bool
}

func (s *bootstrapAwakeOrderSprite) Main() {
	s.sawSelfAwake = s.CostumeIndex() == 0
	if s.peer != nil {
		s.sawPeerAwake = s.peer.CostumeIndex() == 0
	}
}

func newBootstrapAwakeOrderSprite(g *Game, name string) *bootstrapAwakeOrderSprite {
	sprite := &bootstrapAwakeOrderSprite{}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	prepareAwakeCostumes(&sprite.SpriteImpl)
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	return sprite
}

func prepareAwakeCostumes(sprite *SpriteImpl) {
	// With no default animation, awake restores the default costume. Start on
	// another costume so lifecycle tests observe that effect instead of a flag.
	sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	sprite.setCostumeIndex(1)
	sprite.spriteState.IsVisible = true
}

func runBootstrapTasksWithScheduler(t *testing.T, game *Game, generation uint64) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		game.runBootstrapTasks(generation)
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("bootstrap tasks did not finish while pumping scheduler")
		}
		gco.Update()
		time.Sleep(time.Millisecond)
	}
}

func TestRunSpriteCallbacksAwakesAllSpritesBeforeMain(t *testing.T) {
	var game Game
	spriteA := newBootstrapAwakeOrderSprite(&game, "SpriteA")
	spriteB := newBootstrapAwakeOrderSprite(&game, "SpriteB")
	spriteA.peer = spriteB
	spriteB.peer = spriteA
	generation := game.bootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(&game).Elem(),
		generation,
	)
	game.runBootstrapTasks(generation)
	if !spriteA.sawSelfAwake || !spriteA.sawPeerAwake {
		t.Fatalf("SpriteA main saw awake state self=%v peer=%v, want both true", spriteA.sawSelfAwake, spriteA.sawPeerAwake)
	}
	if !spriteB.sawSelfAwake || !spriteB.sawPeerAwake {
		t.Fatalf("SpriteB main saw awake state self=%v peer=%v, want both true", spriteB.sawSelfAwake, spriteB.sawPeerAwake)
	}
}

func TestRunSpriteCallbacksRunsSpriteMainsInZOrderUntilFirstYield(t *testing.T) {
	setupRuntimeScheduler(t)
	var game Game
	blocked := make(chan struct{})
	defer close(blocked)
	var spriteASeenThreadCount int64
	var spriteBSeenThreadCount int64
	spriteA := newCollisionLayerOrderSprite(&game, "SpriteA", func() {
		spriteASeenThreadCount = gco.LastThreadID()
		engine.WaitForChan(blocked)
	})
	spriteB := newCollisionLayerOrderSprite(&game, "SpriteB", func() {
		spriteBSeenThreadCount = gco.LastThreadID()
	})
	generation := game.bootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(&game).Elem(),
		generation,
	)
	runBootstrapTasksWithScheduler(t, &game, generation)
	if spriteASeenThreadCount != 1 {
		t.Fatalf("SpriteA saw %d created threads before its first yield, want 1", spriteASeenThreadCount)
	}
	if spriteBSeenThreadCount != 2 {
		t.Fatalf("SpriteB saw %d created threads before its first yield, want 2", spriteBSeenThreadCount)
	}
}

func TestRunBootstrapMainUntilYieldReleasesFollowingBootstrapTasks(t *testing.T) {
	setupRuntimeScheduler(t)
	var game Game
	blocked := make(chan struct{})
	defer close(blocked)
	stageStarted := make(chan struct{})
	stageResumed := make(chan struct{})
	followingTaskRan := make(chan struct{})
	generation := game.bootstrapGeneration()
	game.queueBootstrap(generation, func() {
		runMainUntilYield(&game, func() {
			close(stageStarted)
			engine.WaitForChan(blocked)
			close(stageResumed)
		})
	})
	game.queueBootstrap(generation, func() {
		close(followingTaskRan)
	})
	runBootstrapTasksWithScheduler(t, &game, generation)
	select {
	case <-stageStarted:
	default:
		t.Fatal("stage Main did not start")
	}
	select {
	case <-followingTaskRan:
	default:
		t.Fatal("following bootstrap task did not run after stage Main yielded")
	}
	select {
	case <-stageResumed:
		t.Fatal("stage Main resumed before its wait was released")
	default:
	}
}

func TestRunSpriteCallbacksAllowsOnStartAfterMainFirstYield(t *testing.T) {
	setupRuntimeScheduler(t)
	var game Game
	game.initEventQueueState()
	game.events = make(chan event, eventBufferSize)
	blocked := make(chan struct{})
	defer close(blocked)
	started := make(chan struct{})
	var spriteA *collisionLayerOrderSprite
	spriteA = newCollisionLayerOrderSprite(&game, "SpriteA", func() {
		spriteA.OnStart(func() {
			close(started)
		})
		engine.WaitForChan(blocked)
	})
	spriteB := newCollisionLayerOrderSprite(&game, "SpriteB", nil)
	generation := game.bootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(&game).Elem(),
		generation,
	)
	runBootstrapTasksWithScheduler(t, &game, generation)
	game.completeBootstrap(generation)
	game.dispatchStartEventIfNeeded()
	gco.Update()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("OnStart did not run after Main yielded once")
	}
}

func TestBootstrapStateTaskSnapshots(t *testing.T) {
	var state gameBootstrapState
	generation := state.bootstrapGeneration()
	var got []string
	if state.queueBootstrap(generation, nil) || state.queueBootstrap(generation+1, func() {}) {
		t.Fatal("accepted a nil task or a task for a different generation")
	}
	state.queueBootstrap(generation, func() { got = append(got, "first") })
	if tasks := state.takeBootstrapTasks(generation + 1); len(tasks) != 0 {
		t.Fatal("a different generation consumed current tasks")
	}
	first := state.takeBootstrapTasks(generation)
	state.queueBootstrap(generation, func() { got = append(got, "second") })
	for _, task := range first {
		task()
	}
	if !slices.Equal(got, []string{"first"}) {
		t.Fatalf("snapshot = %v, want [first]", got)
	}
	for _, task := range state.takeBootstrapTasks(generation) {
		task()
	}
	if !slices.Equal(got, []string{"first", "second"}) {
		t.Fatalf("task order = %v, want [first second]", got)
	}
	if len(state.takeBootstrapTasks(generation)) != 0 {
		t.Fatal("consumed tasks were returned again")
	}
}

func TestBootstrapStateConcurrentClaim(t *testing.T) {
	var state gameBootstrapState
	generation := state.bootstrapGeneration()
	if state.claimBootstrap(generation + 1) {
		t.Fatal("claimed a different generation")
	}
	var claims atomic.Int32
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if state.claimBootstrap(generation) {
				claims.Add(1)
			}
		}()
	}
	workers.Wait()
	if got := claims.Load(); got != 1 {
		t.Fatalf("successful claims = %d, want 1", got)
	}
	if !state.isCurrentBootstrap(generation) || state.isCurrentBootstrap(generation+1) {
		t.Fatal("claiming bootstrap changed the generation")
	}
}
