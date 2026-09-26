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

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
)

// Stamp-driven games broadcast to many clones each frame.
func BenchmarkTargetOrder(b *testing.B) {
	game := &Game{}
	game.initShapeMgr()
	sinks := make([]eventSink, 0, 202)
	for range 100 {
		sprite := &SpriteImpl{g: game}
		game.shapeMgr.add(sprite)
		sinks = append(sinks, eventSink{Owner: sprite}, eventSink{Owner: sprite})
	}
	sinks = append(sinks, eventSink{Owner: game}, eventSink{Owner: "external"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := sinksInTargetOrder(game, sinks); len(got) != len(sinks) {
			b.Fatal(len(got))
		}
	}
}

func BenchmarkTargetDispatch(b *testing.B) {
	previous := gco
	gco = nil
	defer func() { gco = previous }()
	for _, scenario := range []struct {
		name           string
		count, matches int
	}{
		{"1/missing", 1, 0},
		{"1/present", 1, 1},
		{"100/missing", 100, 0},
		{"100/present", 100, 1},
		{"300/missing", 300, 0},
		{"300/present", 300, 1},
		{"300/mixed-64", 300, 64},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			var registry scriptEventRegistry
			owner := &SpriteImpl{}
			for i := range scenario.count {
				sink := eventSink{Owner: &SpriteImpl{}}
				if i >= scenario.count-scenario.matches {
					sink.Owner = owner
					if scenario.matches > 1 && i%2 == 0 {
						sink.Cond = func(any) bool { return false }
					}
				}
				registry.manager.Add(coreevent.BucketTouchStart, sink)
			}
			event := scriptEventDispatch{run: func(coroutine.Thread, *eventSink) {}}
			b.ReportAllocs()
			for range b.N {
				registry.dispatchTarget(coreevent.BucketTouchStart, owner, event)
			}
		})
	}
}

func BenchmarkTargetDispatchMissingManaged(b *testing.B) {
	var registry scriptEventRegistry
	for range 300 {
		registry.manager.Add(coreevent.BucketTouchStart, eventSink{Owner: &SpriteImpl{}})
	}
	owner := &SpriteImpl{}
	event := scriptEventDispatch{mode: coroutine.BatchAsync}
	b.ReportAllocs()
	for range b.N {
		registry.dispatchTarget(coreevent.BucketTouchStart, owner, event)
	}
}

func TestTargetDispatchMutation(t *testing.T) {
	previous := gco
	gco = nil
	defer func() { gco = previous }()

	var registry scriptEventRegistry
	owner, other := &SpriteImpl{}, &SpriteImpl{}
	var calls []string
	enabled, nested := true, false
	var event scriptEventDispatch
	event.run = func(_ coroutine.Thread, sink *eventSink) {
		name := sink.Handler.(string)
		calls = append(calls, name)
		if name == "first" && !nested {
			nested = true
			registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "second"})
			registry.dispatchTarget(coreevent.BucketClick, owner, event)
		}
		if name == "delete" {
			registry.manager.DeleteOwner(owner)
		}
	}
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: other, Handler: "other"})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: owner, Handler: "first", Cond: func(any) bool { return enabled },
	})

	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second"}; !slices.Equal(calls, want) {
		t.Fatalf("nested dispatch = %v, want %v", calls, want)
	}
	enabled = false
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second", "second"}; !slices.Equal(calls, want) {
		t.Fatalf("condition change = %v, want %v", calls, want)
	}
	registry.manager.DeleteOwner(owner)
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if len(calls) != 4 {
		t.Fatalf("deleted owner received event: %v", calls)
	}
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "delete"})
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "after"})
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second", "second", "delete", "after"}; !slices.Equal(calls, want) {
		t.Fatalf("deleted during dispatch = %v, want %v", calls, want)
	}
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if len(calls) != 6 {
		t.Fatalf("deleted handlers received another event: %v", calls)
	}
}

func TestTargetOrderPreservesGroupsAndSnapshot(t *testing.T) {
	game := &Game{}
	game.initShapeMgr()
	back, front := &SpriteImpl{g: game}, &SpriteImpl{g: game}
	removed := &SpriteImpl{g: game}
	game.shapeMgr.add(back)
	game.shapeMgr.add(front)
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
	ordered := sinksInTargetOrder(game, sinks)
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
	if got := sinksInTargetOrder(game, sinks)[0].Handler; got != "back-1" {
		t.Fatalf("first handler after layer change = %v", got)
	}
}
