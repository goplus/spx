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

import "testing"

func TestGameRunBootstrapTasksDrainsNestedCallbacks(t *testing.T) {
	var g Game
	var got []string

	g.deferBootstrap(func() {
		got = append(got, "first")
		g.deferBootstrap(func() {
			got = append(got, "nested")
		})
	})
	g.deferBootstrap(func() {
		got = append(got, "second")
	})

	g.runBootstrapTasks()

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
	g.scriptEventBindings.init(&g.scriptEvents, &g)

	var sprite SpriteImpl
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite)

	g.deferBootstrap(func() {
		g.OnStart(func() {})
	})
	g.deferBootstrap(func() {
		sprite.OnStart(func() {})
	})

	g.runBootstrapTasks()

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
	generation := g.currentBootstrapGeneration()
	var got []string

	if !g.deferBootstrapFor(generation, func() {
		got = append(got, "first")
		g.resetBootstrapState()
		g.deferBootstrapFor(g.currentBootstrapGeneration(), func() {
			got = append(got, "new")
		})
	}) {
		t.Fatal("deferBootstrapFor rejected current generation")
	}
	if !g.deferBootstrapFor(generation, func() {
		got = append(got, "stale")
	}) {
		t.Fatal("deferBootstrapFor rejected current generation")
	}

	g.runBootstrapTasksFor(generation)
	if len(got) != 1 || got[0] != "first" {
		t.Fatalf("old generation drained stale tasks: got %v, want [first]", got)
	}

	g.runBootstrapTasks()
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
