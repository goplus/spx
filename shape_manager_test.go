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
)

func newLayerOrderTestGame(names ...string) (*Game, map[string]*SpriteImpl) {
	game := &Game{}
	game.initShapeMgr()
	sprites := make(map[string]*SpriteImpl, len(names))
	for _, name := range names {
		spr := &SpriteImpl{g: game, name: name}
		sprites[name] = spr
		game.addShape(spr)
	}
	game.shapeMgr.updateRenderLayers()
	return game, sprites
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

func TestSpriteLayerActionsMatchScratchOrdering(t *testing.T) {
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
			game.removeShape(sprites[tt.remove])
			assertLayerOrder(t, game, tt.want)
		})
	}
}
