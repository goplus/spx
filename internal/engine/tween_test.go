/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package engine

import (
	"fmt"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type tweenTestSprite struct {
	gdx.ISpriter
	id        Object
	position  mathf.Vec2
	positions []mathf.Vec2
}

func (s *tweenTestSprite) GetId() Object           { return s.id }
func (s *tweenTestSprite) GetPosition() mathf.Vec2 { return s.position }
func (s *tweenTestSprite) SetPosition(position mathf.Vec2) {
	s.position = position
	s.positions = append(s.positions, position)
}

func setupTweenTest(t *testing.T) *tweenTestSprite {
	t.Helper()
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	sprite := &tweenTestSprite{id: 1}
	state.sprites[sprite.id] = sprite
	return sprite
}

func TestTweenCompletionWritesEndpointBeforeCallback(t *testing.T) {
	for _, test := range []struct {
		name     string
		duration float64
		delta    float64
	}{
		{"exact", 1, 1},
		{"overshoot", 1, 1.25},
		{"zero duration", 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			sprite := setupTweenTest(t)
			want := mathf.NewVec2(10, 20)
			calls := 0
			tweenPos(sprite, want, test.duration, func() {
				calls++
				if sprite.position != want {
					t.Errorf("callback observed position %v, want endpoint %v", sprite.position, want)
				}
			})
			updateTweens(test.delta)
			updateTweens(1)
			if sprite.position != want || calls != 1 || len(tweenInfos) != 0 {
				t.Fatalf("completion = position %v, callbacks %d, active %d; want %v, 1, 0", sprite.position, calls, len(tweenInfos), want)
			}
		})
	}
}

func TestTweenSegmentsStartAtPreviousEndpoint(t *testing.T) {
	sprite := setupTweenTest(t)
	calls := 0
	tweenPos2(sprite, mathf.NewVec2(10, 0), 1, mathf.NewVec2(20, 0), 1, func() { calls++ })
	for _, step := range []struct {
		delta float64
		want  float64
		calls int
	}{{0.5, 5, 0}, {0.75, 12.5, 0}, {0.75, 20, 1}} {
		updateTweens(step.delta)
		if sprite.position != mathf.NewVec2(step.want, 0) || calls != step.calls {
			t.Fatalf("after delta %v: position %v, callbacks %d; want %v, %d", step.delta, sprite.position, calls, step.want, step.calls)
		}
	}
}

func TestTweenCrossedSegmentsCommitInOrder(t *testing.T) {
	for _, durations := range [][2]float64{{1, 1}, {0, 1}, {1, 0}, {0, 0}} {
		t.Run(fmt.Sprint(durations), func(t *testing.T) {
			sprite := setupTweenTest(t)
			first, last := mathf.NewVec2(10, 0), mathf.NewVec2(20, 0)
			tweenPos2(sprite, first, durations[0], last, durations[1], func() {
				if !slices.Equal(sprite.positions, []mathf.Vec2{first, last}) {
					t.Errorf("callback observed writes %v, want [%v %v]", sprite.positions, first, last)
				}
			})
			updateTweens(durations[0] + durations[1])
			if !slices.Equal(sprite.positions, []mathf.Vec2{first, last}) {
				t.Fatalf("writes = %v, want [%v %v]", sprite.positions, first, last)
			}
		})
	}
}

func TestTweenCallbacksFollowAllUpdatesAndDeferNewTweens(t *testing.T) {
	first := setupTweenTest(t)
	second := &tweenTestSprite{id: 2}
	state.sprites[second.id] = second
	var calls []Object
	for _, sprite := range []*tweenTestSprite{first, second} {
		tweenPos(sprite, mathf.NewVec2(float64(sprite.id)*10, 0), 1, func() {
			calls = append(calls, sprite.id)
			if first.position.X != 10 || second.position.X != 20 {
				t.Errorf("callback before all endpoints: first %v, second %v", first.position, second.position)
			}
			if sprite == first {
				tweenPos(first, mathf.NewVec2(30, 0), 1, func() {})
			}
		})
	}
	updateTweens(1)
	if !slices.Equal(calls, []Object{1, 2}) || len(tweenInfos) != 1 || first.position.X != 10 {
		t.Fatalf("callbacks %v, active %d, first %v; want [1 2], 1, (10,0)", calls, len(tweenInfos), first.position)
	}
	updateTweens(0.5)
	if first.position.X != 20 {
		t.Fatalf("new tween position = %v, want (20,0)", first.position)
	}
}
