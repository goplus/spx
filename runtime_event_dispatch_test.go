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
	"testing"
)

// Stamp-driven games broadcast to many clones each frame.
func BenchmarkScratchTargetOrder(b *testing.B) {
	game := &Game{}
	game.initShapeMgr()
	sinks := make([]eventSink, 0, 202)
	for range 100 {
		sprite := &SpriteImpl{g: game}
		game.addShape(sprite)
		sinks = append(sinks, eventSink{Owner: sprite}, eventSink{Owner: sprite})
	}
	sinks = append(sinks, eventSink{Owner: game}, eventSink{Owner: "external"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := sinksInScratchTargetOrder(game, sinks); len(got) != len(sinks) {
			b.Fatal(len(got))
		}
	}
}

func TestScratchTargetOrderPreservesGroupsAndSnapshot(t *testing.T) {
	game := &Game{}
	game.initShapeMgr()
	back, front := &SpriteImpl{g: game}, &SpriteImpl{g: game}
	removed := &SpriteImpl{g: game}
	game.addShape(back)
	game.addShape(front)
	sinks := []eventSink{
		{Owner: game, Handler: "stage-1"},
		{Owner: back, Handler: "back-1"},
		{Owner: removed, Handler: "removed"},
		{Owner: front, Handler: "front-1"},
		{Owner: []int{1}, Handler: "external"},
		{Owner: back, Handler: "back-2"},
		{Owner: game, Handler: "stage-2"},
		{Owner: front, Handler: "front-2"},
		{Owner: &Game{}, Handler: "other-game"},
	}
	snapshot := slices.Clone(sinks)
	ordered := sinksInScratchTargetOrder(game, sinks)
	names := make([]string, len(ordered))
	for i, sink := range ordered {
		names[i] = sink.Handler.(string)
	}
	want := []string{"front-1", "front-2", "back-1", "back-2", "removed", "external", "other-game", "stage-1", "stage-2"}
	if !slices.Equal(names, want) {
		t.Fatalf("order = %v, want %v", names, want)
	}
	if !reflect.DeepEqual(sinks, snapshot) {
		t.Fatal("sorting modified the shared event snapshot")
	}
	ordered[0].Handler = "changed"
	if !reflect.DeepEqual(sinks, snapshot) {
		t.Fatal("ordered result aliases the shared event snapshot")
	}
	// A later broadcast must observe the new layer order, not a cached grouping.
	game.shapeMgr.items[0], game.shapeMgr.items[1] = front, back
	if got := sinksInScratchTargetOrder(game, sinks)[0].Handler; got != "back-1" {
		t.Fatalf("first handler after layer change = %v", got)
	}
}
