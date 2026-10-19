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
	"math"
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/goplus/spx/v3/internal/ui"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func newLayerOrderTestGame(names ...string) (*Game, map[string]*SpriteImpl) {
	game := &Game{}
	game.initShapeMgr()
	sprites := make(map[string]*SpriteImpl, len(names))
	for _, name := range names {
		spr := &SpriteImpl{g: game, name: name}
		sprites[name] = spr
		game.shapeMgr.add(spr)
	}
	game.shapeMgr.updateRenderLayers()
	return game, sprites
}

func TestShapeManagerInitClearsReferences(t *testing.T) {
	marker := &struct{}{}
	s := shapeManager{
		items:        []Shape{marker},
		tempItems:    []Shape{marker},
		destroyItems: []Shape{marker},
	}
	itemCap, tempCap, destroyCap := cap(s.items), cap(s.tempItems), cap(s.destroyItems)
	s.init()
	for name, shapes := range map[string][]Shape{
		"items": s.items, "tempItems": s.tempItems, "destroyItems": s.destroyItems,
	} {
		if len(shapes) != 0 || shapes[:1][0] != nil {
			t.Fatalf("%s retained a shape reference", name)
		}
	}
	if cap(s.items) != itemCap || cap(s.tempItems) != tempCap || cap(s.destroyItems) != destroyCap {
		t.Fatal("init changed reusable shape capacities")
	}
}

func TestShapeManagerFlushDestroyClearsQueue(t *testing.T) {
	marker := &struct{}{}
	s := shapeManager{destroyItems: []Shape{marker}}
	s.flushDestroy(nil)
	if len(s.destroyItems) != 0 || s.destroyItems[:1][0] != nil {
		t.Fatal("flushDestroy retained a shape reference")
	}
}

func spriteOrderNames(shapes []Shape) []string {
	names := make([]string, 0, len(shapes))
	for _, shape := range shapes {
		if spr, ok := shape.(*SpriteImpl); ok {
			names = append(names, spr.name)
		}
	}
	return names
}

func assertLayerOrder(t *testing.T, game *Game, want []string) {
	t.Helper()
	if got := spriteOrderNames(game.getAllShapes()); !reflect.DeepEqual(got, want) {
		t.Fatalf("sprite order = %v, want %v", got, want)
	}
	wantLayer := firstSpriteLayer
	for _, shape := range game.getAllShapes() {
		spr, ok := shape.(*SpriteImpl)
		if !ok {
			continue
		}
		if got := spr.runtimeState.Layer; got != wantLayer {
			t.Fatalf("sprite %q layer = %d, want %d", spr.name, got, wantLayer)
		}
		wantLayer++
	}
}

func TestSpriteLayerActionsMatchTargetOrdering(t *testing.T) {
	tests := []struct {
		name   string
		target string
		move   func(*SpriteImpl)
		want   []string
	}{
		{name: "front", target: "C", move: func(s *SpriteImpl) { s.SetLayerTo(Front) }, want: []string{"A", "B", "D", "E", "C"}},
		{name: "back", target: "C", move: func(s *SpriteImpl) { s.SetLayerTo(Back) }, want: []string{"C", "A", "B", "D", "E"}},
		{name: "forward one", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Forward, 1) }, want: []string{"A", "B", "D", "C", "E"}},
		{name: "forward two", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Forward, 2) }, want: []string{"A", "B", "D", "E", "C"}},
		{name: "backward one", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Backward, 1) }, want: []string{"A", "C", "B", "D", "E"}},
		{name: "backward two", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Backward, 2) }, want: []string{"C", "A", "B", "D", "E"}},
		{name: "negative forward reverses", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Forward, -1) }, want: []string{"A", "C", "B", "D", "E"}},
		{name: "negative backward reverses", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Backward, -1) }, want: []string{"A", "B", "D", "C", "E"}},
		{name: "zero is no-op", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Forward, 0) }, want: []string{"A", "B", "C", "D", "E"}},
		{name: "forward clamps", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Forward, math.MaxInt) }, want: []string{"A", "B", "D", "E", "C"}},
		{name: "backward clamps", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Backward, math.MaxInt) }, want: []string{"C", "A", "B", "D", "E"}},
		{name: "minimum forward reverses without overflow", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Forward, math.MinInt) }, want: []string{"C", "A", "B", "D", "E"}},
		{name: "minimum backward reverses", target: "C", move: func(s *SpriteImpl) { s.SetLayer__1(Backward, math.MinInt) }, want: []string{"A", "B", "D", "E", "C"}},
		{name: "backmost stays at back", target: "A", move: func(s *SpriteImpl) { s.SetLayerTo(Back) }, want: []string{"A", "B", "C", "D", "E"}},
		{name: "frontmost stays at front", target: "E", move: func(s *SpriteImpl) { s.SetLayerTo(Front) }, want: []string{"A", "B", "C", "D", "E"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, sprites := newLayerOrderTestGame("A", "B", "C", "D", "E")
			tt.move(sprites[tt.target])
			assertLayerOrder(t, game, tt.want)
		})
	}
}

func TestSpriteLayerMovesSkipNonSpriteShapes(t *testing.T) {
	game, sprites := newLayerOrderTestGame("A", "B", "C")
	marker1 := &struct{ name string }{name: "monitor"}
	marker2 := &struct{ name string }{name: "bubble"}
	marker3 := &struct{ name string }{name: "measure"}
	game.shapeMgr.items = []Shape{sprites["A"], marker1, sprites["B"], marker2, sprites["C"], marker3}
	game.shapeMgr.updateRenderLayers()

	sprites["C"].SetLayer__1(Backward, 1)

	assertLayerOrder(t, game, []string{"A", "C", "B"})
	lastMarkerIndex := -1
	for _, marker := range []Shape{marker1, marker2, marker3} {
		idx := game.shapeMgr.findShapeIndex(marker)
		if idx < 0 {
			t.Fatal("non-sprite shape was dropped during layer move")
		}
		if idx <= lastMarkerIndex {
			t.Fatal("non-sprite shapes changed relative order during layer move")
		}
		lastMarkerIndex = idx
	}
}

func TestHiddenSpritesStillOccupyLayers(t *testing.T) {
	game, sprites := newLayerOrderTestGame("A", "B", "C", "D", "E")
	sprites["D"].spriteState.IsVisible = false

	sprites["C"].SetLayer__1(Forward, 1)

	assertLayerOrder(t, game, []string{"A", "B", "D", "C", "E"})
}

func TestRemovingSpriteReindexesRemainingLayersAbovePen(t *testing.T) {
	tests := []struct {
		name   string
		remove string
		want   []string
	}{
		{name: "back", remove: "A", want: []string{"B", "C"}},
		{name: "middle", remove: "B", want: []string{"A", "C"}},
		{name: "front", remove: "C", want: []string{"A", "B"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game, sprites := newLayerOrderTestGame("A", "B", "C")
			game.shapeMgr.removeShape(sprites[tt.remove])
			assertLayerOrder(t, game, tt.want)
		})
	}
}

type shapeUpdateSpy struct {
	calls int
	delta float64
}

func (s *shapeUpdateSpy) onUpdate(delta float64) { s.calls++; s.delta = delta }

type shapeLayerSortManager struct{ pkgengine.IExtMgr }

func (*shapeLayerSortManager) SetLayerSorterMode(int64) {}

func TestShapeManagerUpdatesCustomShapesAndMonitors(t *testing.T) {
	spy := &shapeUpdateSpy{}
	panel := &monitorPanelSpy{}
	monitor := &Monitor{visible: true, isDirty: true, panel: panel, eval: func() ui.MonitorValue { return ui.MonitorValue{Text: "42"} }}
	manager := shapeManager{}
	manager.flushActivate([]Shape{&SpriteImpl{}, &textBubble{}, &quoterBubble{}, monitor, spy, &struct{}{}})
	if spy.calls != 1 || spy.delta != itime.DeltaTime() || panel.value.Text != "42" || monitor.isDirty {
		t.Fatalf("shape update calls=%d delta=%v monitor=%+v", spy.calls, spy.delta, panel)
	}
}

func TestShapeManagerCachesBoundedNamesAndWrapsBubbleIDs(t *testing.T) {
	manager := shapeManager{nextTextBubbleLayoutID: ^uint64(0)}
	bubble := &textBubble{}
	manager.add(bubble)
	if bubble.layoutID != 1 {
		t.Fatalf("wrapped layout id=%d,want1", bubble.layoutID)
	}
	for _, name := range []string{"missing1", "missing2", "missing3"} {
		if len(manager.spritesNamed(name)) != 0 {
			t.Fatal("missing name found sprites")
		}
	}
	if len(manager.named) != 1 || manager.named["missing3"] != nil {
		t.Fatalf("name miss cache grew without bound: %v", manager.named)
	}
}

func TestShapeManagerLayerSortPreservesAutomaticOrder(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := pkgengine.ExtMgr
	pkgengine.ExtMgr = &shapeLayerSortManager{}
	mode := "none"
	if engine.HasLayerSortMethod() {
		mode = "vertical"
	}
	t.Cleanup(func() { _ = engine.SetLayerSortMode(mode); pkgengine.ExtMgr = previous })
	game, sprites := newLayerOrderTestGame("a", "b")
	game.shapeMgr.goBackLayers(&SpriteImpl{}, 1)
	if err := engine.SetLayerSortMode("vertical"); err != nil {
		t.Fatal(err)
	}
	sprites["a"].runtimeState.Layer = 17
	game.shapeMgr.goBackLayers(sprites["a"], -1)
	game.shapeMgr.updateRenderLayers()
	if got := spriteOrderNames(game.shapeMgr.items); !reflect.DeepEqual(got, []string{"a", "b"}) || sprites["a"].runtimeState.Layer != 17 {
		t.Fatalf("manual layering overrode automatic order: %v", got)
	}
}

func TestShapeManagerCloningDeletedSourceStopsScript(t *testing.T) {
	co := setupRuntimeScheduler(t)
	manager := shapeManager{}
	reached := false
	thread := co.Create(nil, func(coroutine.Thread) { manager.addClonedShape(&SpriteImpl{}, &SpriteImpl{}); reached = true })
	co.Join(thread)
	if reached || len(manager.items) != 0 {
		t.Fatal("cloning removed source continued or inserted a clone")
	}
}

func TestShapeManagerRelayoutsChangedBubbleInput(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	mgr.zoom = mathf.NewVec2(1, 1)
	previousScale := engine.WindowScale()
	engine.SetWindowScale(1)
	t.Cleanup(func() { engine.SetWindowScale(previousScale) })
	sprite := newRenderOffsetTestSprite()
	sprite.spriteState.IsVisible = true
	sprite.g.displayState.WindowWidth, sprite.g.displayState.WindowHeight = 480, 360
	bubble := &textBubble{bubbleBase: bubbleBase{sprite: sprite}, layoutID: 1, panel: &ui.UiSay{}, content: ui.NewSayBubbleContent("hello", ui.StyleSay)}
	manager := shapeManager{textBubbles: []*textBubble{bubble}}
	manager.layoutTextBubbles()
	previous := bubble.layout
	sprite.SetXYpos(80, 60)
	manager.layoutTextBubbles()
	if bubble.layout.SameInput(previous) {
		t.Fatal("moving bubble owner did not refresh cached layout input")
	}
}
