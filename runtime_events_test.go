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
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

func TestGameRunBootstrapTasksExecutesQueuedHooksOnce(t *testing.T) {
	var g Game

	var got []string
	g.deferBootstrap(func() {
		got = append(got, "first")
	})
	g.deferBootstrap(func() {
		got = append(got, "second")
	})

	g.runBootstrapTasks()
	g.runBootstrapTasks()

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
	}
}

type clickThroughSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	hits map[pkgengine.Object]bool
}

func (s *clickThroughSpriteMgr) CheckCollisionWithPoint(obj pkgengine.Object, point mathf.Vec2, isTrigger bool) bool {
	return s.hits[obj]
}

func setupClickThroughSpriteMgr(t *testing.T, hits map[pkgengine.Object]bool) {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	original := pkgengine.SpriteMgr
	pkgengine.SpriteMgr = &clickThroughSpriteMgr{hits: hits}
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
}

func newClickTestSprite(g *Game, name string, id pkgengine.Object, registerClick bool) *SpriteImpl {
	sprite := &SpriteImpl{}
	sprite.g = g
	sprite.name = name
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(id)
	sprite.scriptEventBindings.init(&g.scriptEvents, sprite)
	if registerClick {
		sprite.OnClick(func() {})
	}
	return sprite
}

func TestFindClickTargetSkipsCoveredSpriteWithoutClickHandler(t *testing.T) {
	setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
		2: true,
	})

	var g Game
	g.initShapeMgr()

	bottom := newClickTestSprite(&g, "bottom", 1, true)
	top := newClickTestSprite(&g, "top", 2, false)
	g.addShape(bottom)
	g.addShape(top)

	selection, ok := g.findClickTarget(mathf.NewVec2(0, 0))
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != bottom {
		t.Fatalf("target = %p, want bottom %p", selection.Target, bottom)
	}
	if selection.SwipeTarget != bottom {
		t.Fatalf("swipe target = %p, want bottom %p", selection.SwipeTarget, bottom)
	}
}

func TestFindClickTargetKeepsTopmostClickableSprite(t *testing.T) {
	setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
		2: true,
	})

	var g Game
	g.initShapeMgr()

	bottom := newClickTestSprite(&g, "bottom", 1, true)
	top := newClickTestSprite(&g, "top", 2, true)
	g.addShape(bottom)
	g.addShape(top)

	selection, ok := g.findClickTarget(mathf.NewVec2(0, 0))
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != top {
		t.Fatalf("target = %p, want top %p", selection.Target, top)
	}
	if selection.SwipeTarget != top {
		t.Fatalf("swipe target = %p, want top %p", selection.SwipeTarget, top)
	}
}
