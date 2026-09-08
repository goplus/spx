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
	"fmt"
	"reflect"
	"slices"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type cloneLimitSprite struct {
	SpriteImpl
	onInit  func(*cloneLimitSprite)
	onClone func(*cloneLimitSprite)
}

func (s *cloneLimitSprite) Main() {
	if s.onInit != nil {
		s.onInit(s)
	}
	s.OnCloned__0(func() {
		if s.onClone != nil {
			s.onClone(s)
		}
	})
}

func setupCloneLimitGame(t *testing.T) *Game {
	t.Helper()
	setupCloneSpriteMgr(t)
	game := new(Game)
	game.initShapeMgr()
	original := engine.GetGame()
	engine.SetGame(game)
	t.Cleanup(func() { engine.SetGame(original) })
	setupRuntimeEventScheduler(t)
	return game
}

func newCloneLimitSprite(game *Game, name string) *cloneLimitSprite {
	s := &cloneLimitSprite{}
	s.baseObj.initWithSize(1, 1)
	s.g, s.name, s.sprite = game, name, s
	s.scriptEventBindings.init(&game.scriptEvents, &s.SpriteImpl)
	s.components.initComponents(&s.SpriteImpl, &coreproject.SpriteConfig{})
	s.physics().collisionInfo.Type = physicsColliderNone
	s.physics().triggerInfo.Type = physicsColliderNone
	game.addShape(&s.SpriteImpl)
	return s
}

func TestCloneLimitSharedAcrossSprites(t *testing.T) {
	game := setupCloneLimitGame(t)
	a := newCloneLimitSprite(game, "A")
	b := newCloneLimitSprite(game, "B")
	game.addShape(&struct{}{}) // Non-sprite shapes do not use clone slots.
	var initialized, started int
	for _, s := range []*cloneLimitSprite{a, b} {
		s.onInit = func(*cloneLimitSprite) { initialized++ }
		s.onClone = func(*cloneLimitSprite) { started++ }
	}
	for range 150 {
		a.Clone__0()
		b.Clone__0()
	}
	before := append([]Shape(nil), game.getAllShapes()...)
	a.Clone__1("ignored")
	b.CloneWith(nil)
	if initialized != 300 || started != 300 {
		t.Fatalf("initialized %d and started %d clones, want 300 each", initialized, started)
	}
	if !reflect.DeepEqual(before, game.getAllShapes()) {
		t.Fatal("rejected clone changed the active shapes or layer order")
	}
	if game.shapeMgr.cloneCount != 300 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("clone counts = %d active, %d pending", game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
	// All clones are hidden; hidden clones must still count towards the limit.
	for _, shape := range before {
		if s, ok := shape.(*SpriteImpl); ok && s.Visible() {
			t.Fatal("test unexpectedly created a visible sprite")
		}
	}
}

func TestCloneLimitReleasesSlotOnDeletionAndReset(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	var started int
	source.onClone = func(*cloneLimitSprite) { started++ }
	for range 300 {
		source.Clone__0()
	}
	source.DeleteThisClone() // Originals must not release a slot.
	source.Clone__0()
	if started != 300 {
		t.Fatal("DeleteThisClone on an original released a slot")
	}
	clone := game.getAllShapes()[0].(*SpriteImpl)
	clone.DeleteThisClone()
	game.removeShape(clone) // Removing an already deleted shape is a no-op.
	source.Clone__0()       // Reuse before the deferred native deletion flush.
	source.Clone__0()
	if started != 301 || game.shapeMgr.cloneCount != 300 {
		t.Fatalf("after deletion: started %d, active %d; want 301, 300", started, game.shapeMgr.cloneCount)
	}
	game.shapeMgr.reset()
	source = newCloneLimitSprite(game, "Restarted")
	source.onClone = func(*cloneLimitSprite) { started++ }
	for range 301 {
		source.Clone__0()
	}
	if started != 601 || game.shapeMgr.cloneCount != 300 {
		t.Fatalf("after reset: started %d, active %d; want 601, 300", started, game.shapeMgr.cloneCount)
	}
}

func TestCloneLimitBoundsRecursiveCloneHandlers(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Recursive")
	var started, continued int
	done := make(chan struct{})
	source.onClone = func(s *cloneLimitSprite) {
		started++
		if started <= 300 { // Keep the regression bounded even without the fix.
			s.Clone__0()
		}
		continued++
		if continued == 300 {
			close(done)
		}
	}
	source.Clone__0()
	updateRuntimeEventSchedulerUntil(t, gco, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	if started != 300 || continued != 300 {
		t.Fatalf("recursive chain: started %d, continued %d; want 300 each", started, continued)
	}
	if len(game.getAllShapes()) != 301 {
		t.Fatalf("shape count = %d, want 301", len(game.getAllShapes()))
	}
	for i, shape := range game.getAllShapes() {
		if got := shape.(*SpriteImpl).runtimeState.Layer; got != firstSpriteLayer+i {
			t.Fatalf("shape %d layer = %d, want %d", i, got, firstSpriteLayer+i)
		}
	}
}

func TestCloneLimitReservesSlotDuringInitialization(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	for range 299 {
		source.Clone__0()
	}
	var initialized int
	source.onInit = func(*cloneLimitSprite) {
		initialized++
		if initialized == 1 {
			source.Clone__0()
		}
	}
	source.Clone__0()
	if initialized != 1 || game.shapeMgr.cloneCount != 300 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("initialized %d, active %d, pending %d; want 1, 300, 0", initialized, game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
}

func TestCloneLimitReleasesFailedInitialization(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	sentinel := new(int)
	source.onInit = func(*cloneLimitSprite) { panic(sentinel) }
	func() {
		defer func() {
			if got := recover(); got != sentinel {
				t.Fatalf("panic = %v, want initialization failure", got)
			}
		}()
		source.Clone__0()
	}()
	source.onInit = nil
	for range 300 {
		source.Clone__0()
	}
	if game.shapeMgr.cloneCount != 300 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("failed initialization leaked a slot: active %d, pending %d", game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
}

func TestCloneLimitDeletionDuringHandlerReleasesSlot(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	var started int
	source.onClone = func(s *cloneLimitSprite) {
		started++
		s.DeleteThisClone()
	}
	for range 301 {
		source.Clone__0()
	}
	if started != 301 || game.shapeMgr.cloneCount != 0 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("started %d, active %d, pending %d; want 301, 0, 0", started, game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
}

func TestCloneLimitPreservesScratchLayerOrderAfterRecycling(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	clones := make([]*SpriteImpl, 0, 300)
	want := make([]string, 0, 301)
	for i := range 300 {
		name := fmt.Sprintf("Clone%d", i)
		doClone(source, nil, func(clone *SpriteImpl) {
			clone.name = name
			clones = append(clones, clone)
		})
		want = append(want, name)
	}
	// Scratch puts newer clones immediately behind the source, ahead of older ones.
	want = append(want, "Source")
	assertLayerOrder(t, game, want)
	clones[150].DeleteThisClone()
	want = slices.Delete(want, 150, 151)
	assertLayerOrder(t, game, want)

	source.SetLayerTo(Back)
	want = append([]string{"Source"}, want[:len(want)-1]...)
	assertLayerOrder(t, game, want)
	var descendant *SpriteImpl
	doClone(clones[0].sprite, nil, func(clone *SpriteImpl) {
		clone.name = "Descendant"
		descendant = clone
	})
	if descendant == nil {
		t.Fatal("deleted clone slot was not reusable")
	}
	// A clone of a clone goes behind its immediate parent, using the live order.
	want = slices.Insert(want, 1, "Descendant")
	assertLayerOrder(t, game, want)
	descendant.SetLayerTo(Front)
	want = append(slices.Delete(want, 1, 2), "Descendant")
	assertLayerOrder(t, game, want)
	descendant.SetLayer__1(Backward, 2)
	want = slices.Insert(want[:len(want)-1], len(want)-3, "Descendant")
	assertLayerOrder(t, game, want)
	source.Clone__0() // Full again: must not disturb layer order.
	assertLayerOrder(t, game, want)

	flushCloneProxyUpdates(game)
	mgr := pkgengine.SpriteMgr.(*spyCloneSpriteMgr)
	for _, shape := range game.getAllShapes() {
		sprite := shape.(*SpriteImpl)
		if sprite.runtimeState.SyncSprite == nil {
			continue
		}
		if got := mgr.zIndex(sprite.runtimeState.SyncSprite.GetId()); got != int64(sprite.runtimeState.Layer) {
			t.Fatalf("%s rendered z-index = %d, want %d", sprite.name, got, sprite.runtimeState.Layer)
		}
	}
}

func TestCloneLayerOrderHonorsOnClonedMoves(t *testing.T) {
	for _, test := range []struct {
		name   string
		action layerAction
		want   []string
	}{
		{"front", Front, []string{"Back", "Source", "Front", "Clone"}},
		{"back", Back, []string{"Clone", "Back", "Source", "Front"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			newCloneLimitSprite(game, "Back")
			source := newCloneLimitSprite(game, "Source")
			newCloneLimitSprite(game, "Front")
			source.onClone = func(clone *cloneLimitSprite) {
				clone.SetLayerTo(test.action)
			}
			doClone(source, nil, func(clone *SpriteImpl) { clone.name = "Clone" })
			assertLayerOrder(t, game, test.want)
		})
	}
}
