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
	"sync/atomic"
	"testing"
	"time"

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
	if len(got) != len(want) {
		t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
	}
	for i, item := range want {
		if got[i] != item {
			t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
		}
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

	got := g.scriptEvents.manager.SnapshotStart()
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
	if len(got) != len(want) {
		t.Fatalf("current generation tasks got %v, want %v", got, want)
	}
	for i, item := range want {
		if got[i] != item {
			t.Fatalf("current generation tasks got %v, want %v", got, want)
		}
	}
}

func TestGameBootstrapCompletionIgnoresStaleGeneration(t *testing.T) {
	var game Game
	stale := game.bootstrapGeneration()

	game.lifecycleState.BootstrapDone.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	game.resetBootstrap()

	if game.completeBootstrap(stale) {
		t.Fatal("stale bootstrap generation was marked done")
	}
	if game.markStartDispatched(stale) {
		t.Fatal("stale bootstrap generation was marked started")
	}
	if game.lifecycleState.BootstrapDone.Load() || game.lifecycleState.StartDispatched.Load() {
		t.Fatal("stale generation reopened lifecycle gates")
	}
}
