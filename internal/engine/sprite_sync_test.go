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

package engine

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type typedTransformSpy struct {
	gdx.ISpriteMgr
	id        Object
	transform SpriteTransform
	calls     int
}

type transformBatchSpy struct {
	gdx.ISpriteMgr
	batches [][]float32
}

type positionQuerySpy struct {
	gdx.ISpriteMgr
	query func([]int64, []float32) bool
}

func TestSpriteTransformPreservesDirectValuesAndBatchLayout(t *testing.T) {
	setupSpriteManagerTest(t)
	spy := &typedTransformSpy{}
	gdx.SpriteMgr = spy
	sprite := &Sprite{Sprite: gdx.Sprite{Id: 19}}
	transform := SpriteTransform{
		Position:     mathf.NewVec2(1.2345678901, -2.3456789012),
		Rotation:     0.123456789,
		Scale:        mathf.NewVec2(3.4567890123, -4.5678901234),
		RenderOffset: mathf.NewVec2(5.6789012345, -6.7890123456),
		Visible:      true,
	}
	sprite.ApplyTransform(transform)
	if spy.calls != 1 || spy.id != sprite.Id || spy.transform != transform {
		t.Fatalf("direct transform = %d/%+v, want %d/%+v", spy.id, spy.transform, sprite.Id, transform)
	}

	var buffer SpriteSyncBuffer
	buffer.AddTransform(sprite.Id, transform)
	buffer.AddDelete(23)
	want := []float32{1, 1, 19, float32(transform.Position.X), float32(transform.Position.Y),
		float32(transform.Rotation), float32(transform.Scale.X), float32(transform.Scale.Y),
		float32(transform.RenderOffset.X), float32(transform.RenderOffset.Y), 1, 23}
	if got := buffer.Serialize(); !slices.Equal(got, want) {
		t.Fatalf("typed transform packet = %v, want %v", got, want)
	}
	buffer.Clear()
	transform.Visible = false
	buffer.AddTransform(sprite.Id, transform)
	if got := buffer.Serialize()[2+SyncFieldsPerSprite-1]; got != 0 {
		t.Fatalf("hidden transform visibility = %v, want 0", got)
	}
}

func TestSpriteTransformRejectsOverflowBeforeDirectSubmission(t *testing.T) {
	setupSpriteManagerTest(t)
	spy := &typedTransformSpy{}
	gdx.SpriteMgr = spy
	sprite := &Sprite{Sprite: gdx.Sprite{Id: 73}}
	defer func() {
		if got := recover(); got == nil || spy.calls != 0 {
			t.Fatalf("invalid direct submission: panic=%v calls=%d", got, spy.calls)
		}
	}()
	sprite.ApplyTransform(SpriteTransform{Position: mathf.NewVec2(math.MaxFloat64, 0)})
}

func TestSpriteTransformRejectsFloat32Overflow(t *testing.T) {
	for field, name := range []string{"x", "y", "rotation", "scale x", "scale y", "offset x", "offset y"} {
		t.Run(name, func(t *testing.T) {
			fields := [7]float64{1, 2, 3, 4, 5, 6, 7}
			fields[field] = math.MaxFloat64
			defer func() {
				want := fmt.Sprintf("sprite 73 transform %s=%v must be finite after float32 conversion", name, math.MaxFloat64)
				if got := recover(); got != want {
					t.Fatalf("transform validation panic = %v, want %q", got, want)
				}
			}()
			SpriteTransform{Position: mathf.NewVec2(fields[0], fields[1]), Rotation: fields[2], Scale: mathf.NewVec2(fields[3], fields[4]), RenderOffset: mathf.NewVec2(fields[5], fields[6])}.Validate(73)
		})
	}
}

func TestSpriteSyncBufferSerializeReusesBuffer(t *testing.T) {
	buffer := NewSpriteSyncBuffer(1)
	buffer.AddTransform(1, SpriteTransform{Position: mathf.NewVec2(2, 3), Rotation: 4, Scale: mathf.NewVec2(5, 6), RenderOffset: mathf.NewVec2(7, 8), Visible: true})
	buffer.AddDelete(9)
	buffer.AddDelete(12)
	if buffer.UpdateCount() != 1 || !slices.Equal(buffer.deleteIDs, []int64{9, 12}) {
		t.Fatal("buffer counts or deletion IDs incorrect")
	}

	first := buffer.Serialize()
	if want := []float32{1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 1, 9, 12}; !slices.Equal(first, want) {
		t.Fatalf("Serialize = %v, want %v", first, want)
	}
	firstPtr := &first[0]

	buffer.Clear()
	if buffer.UpdateCount() != 0 || len(buffer.deleteIDs) != 0 || buffer.Serialize() != nil {
		t.Fatal("clear retained buffer entries")
	}
	buffer.AddTransform(10, SpriteTransform{Position: mathf.NewVec2(11, 12), Rotation: 13, Scale: mathf.NewVec2(14, 15), RenderOffset: mathf.NewVec2(16, 17), Visible: false})

	second := buffer.Serialize()
	if len(second) == 0 {
		t.Fatal("Serialize returned an empty buffer after reuse")
	}
	if firstPtr != &second[0] {
		t.Fatal("Serialize allocated a new backing array instead of reusing scratch storage")
	}
	if got, want := len(second), 2+SyncFieldsPerSprite; got != want {
		t.Fatalf("Serialize len = %d, want %d", got, want)
	}
	if got, want := cap(second), len(second); got != want {
		t.Fatalf("Serialize cap = %d, want %d", got, want)
	}
	if got, want := second[0], float32(1); got != want {
		t.Fatalf("header updateCount = %v, want %v", got, want)
	}
	if got, want := second[1], float32(0); got != want {
		t.Fatalf("header deleteCount = %v, want %v", got, want)
	}
	if got, want := second[2], float32(10); got != want {
		t.Fatalf("sprite id = %v, want %v", got, want)
	}
}

func TestSpriteSyncBufferRejectsNonFiniteTransforms(t *testing.T) {
	for field, name := range []string{"x", "y", "rotation", "scale x", "scale y", "offset x", "offset y"} {
		t.Run(name, func(t *testing.T) {
			for _, invalid := range []struct {
				name  string
				value float64
			}{
				{"nan", math.NaN()},
				{"positive infinity", math.Inf(1)},
				{"negative infinity", math.Inf(-1)},
				{"positive float32 overflow", math.MaxFloat64},
				{"negative float32 overflow", -math.MaxFloat64},
			} {
				t.Run(invalid.name, func(t *testing.T) {
					fields := [7]float64{1, 2, 3, 4, 5, 6, 7}
					fields[field] = invalid.value
					buffer := NewSpriteSyncBuffer(1)
					buffer.AddTransform(73, SpriteTransform{Position: mathf.NewVec2(fields[0], fields[1]), Rotation: fields[2], Scale: mathf.NewVec2(fields[3], fields[4]), RenderOffset: mathf.NewVec2(fields[5], fields[6]), Visible: true})
					defer func() {
						want := fmt.Sprintf("sprite 73 transform %s=%v must be finite after float32 conversion", name, float64(float32(invalid.value)))
						if got := recover(); got != want {
							t.Fatalf("serialization panic = %v, want %q", got, want)
						}
					}()
					buffer.Serialize()
				})
			}
		})
	}
}

func TestSpriteSyncBufferAcceptsFiniteFloat32Limits(t *testing.T) {
	transform := SpriteTransform{Position: mathf.NewVec2(math.MaxFloat32, -math.MaxFloat32), Rotation: math.SmallestNonzeroFloat32, Scale: mathf.NewVec2(1, 1), Visible: true}
	transform.Validate(1)
	buffer := NewSpriteSyncBuffer(1)
	buffer.AddTransform(1, transform)
	packet := buffer.Serialize()
	if packet[3] != math.MaxFloat32 || packet[4] != -math.MaxFloat32 || packet[5] != math.SmallestNonzeroFloat32 {
		t.Fatalf("finite float32 limits changed during serialization: %v", packet)
	}
}

func TestLegacyBatchObjectIDs(t *testing.T) {
	serializers := []struct {
		name      string
		serialize func(int64) []float32
		idIndex   int
	}{
		{
			name: "transform",
			serialize: func(id int64) []float32 {
				buffer := NewSpriteSyncBuffer(2)
				buffer.AddTransform(1, SpriteTransform{Position: mathf.NewVec2(2, 3), Rotation: 4, Scale: mathf.NewVec2(5, 6), RenderOffset: mathf.NewVec2(7, 8), Visible: true})
				buffer.AddTransform(id, SpriteTransform{Position: mathf.NewVec2(2, 3), Rotation: 4, Scale: mathf.NewVec2(5, 6), RenderOffset: mathf.NewVec2(7, 8), Visible: true})
				buffer.AddDelete(2)
				return buffer.Serialize()
			},
			idIndex: 2 + SyncFieldsPerSprite,
		},
		{
			name: "delete",
			serialize: func(id int64) []float32 {
				buffer := NewSpriteSyncBuffer(1)
				buffer.AddTransform(1, SpriteTransform{Position: mathf.NewVec2(2, 3), Rotation: 4, Scale: mathf.NewVec2(5, 6), RenderOffset: mathf.NewVec2(7, 8), Visible: true})
				buffer.AddDelete(2)
				buffer.AddDelete(id)
				return buffer.Serialize()
			},
			idIndex: 2 + SyncFieldsPerSprite + 1,
		},
	}
	cases := []struct {
		id    int64
		valid bool
	}{
		{0, true},
		{1, true},
		{1<<24 - 1, true},
		{1 << 24, true},
		{1<<24 + 1, false},
		{1<<24 + 2, true},
		{1 << 53, true},
		{1<<53 + 1, false},
		{1 << 60, true},
		{1<<60 + 1, false},
		{math.MaxInt64 - (1 << 39) + 1, true}, // Largest float32 below 2^63.
		{math.MaxInt64, false},
		{-1, false},
		{math.MinInt64, false},
	}
	for _, serializer := range serializers {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%d", serializer.name, tc.id), func(t *testing.T) {
				defer func() {
					recovered := recover()
					if tc.valid {
						if recovered != nil {
							t.Fatalf("exactly representable ID caused panic: %v", recovered)
						}
						return
					}
					if recovered == nil {
						t.Fatal("invalid ID did not cause a panic")
					}
					if !strings.Contains(fmt.Sprint(recovered), fmt.Sprint(tc.id)) {
						t.Fatalf("panic %q does not identify original ID %d", recovered, tc.id)
					}
				}()
				packet := serializer.serialize(tc.id)
				if tc.valid && int64(packet[serializer.idIndex]) != tc.id {
					t.Errorf("serialized ID %v does not round-trip to %d", packet[serializer.idIndex], tc.id)
				}
			})
		}
	}
}

func TestSyncBatchUpdateSpritesForwardsOnlyNonemptyPackets(t *testing.T) {
	setupSpriteManagerTest(t)
	manager := &transformBatchSpy{}
	gdx.SpriteMgr = manager
	buffer := NewSpriteSyncBuffer(1)
	SyncBatchUpdateSprites(nil)
	SyncBatchUpdateSprites(buffer.Serialize())
	if len(manager.batches) != 0 {
		t.Fatal("empty sprite batch reached the engine")
	}
	buffer.AddTransform(7, SpriteTransform{Position: mathf.NewVec2(1, -2), Rotation: 30, Scale: mathf.NewVec2(4, 5), RenderOffset: mathf.NewVec2(-6, 8), Visible: true})
	buffer.AddDelete(9)
	SyncBatchUpdateSprites(buffer.Serialize())
	want := []float32{1, 1, 7, 1, -2, 30, 4, 5, -6, 8, 1, 9}
	if len(manager.batches) != 1 || !slices.Equal(manager.batches[0], want) {
		t.Fatalf("engine batches = %v, want [%v]", manager.batches, want)
	}
	if got := buffer.Serialize(); !slices.Equal(got, want) {
		t.Fatalf("sending altered the pending packet: %v", got)
	}
}

func TestPositionQueryBufferOwnsReusableStorage(t *testing.T) {
	setupSpriteManagerTest(t)
	ids := []int64{0x112233447fc00001, -1}
	calls, success := 0, true
	gdx.SpriteMgr = &positionQuerySpy{query: func(input []int64, out []float32) bool {
		calls++
		if !slices.Equal(input, ids) || &input[0] != &ids[0] {
			t.Fatal("position query changed the original ID buffer")
		}
		if len(out) != len(ids)*2 {
			t.Fatal("position query has incorrect output length")
		}
		if success {
			copy(out, []float32{float32(calls), -4, 5, -6})
		}
		return success
	}}

	var query PositionQueryBuffer
	if got := query.GetPositions(nil); got != nil || calls != 0 {
		t.Fatal("empty query allocated storage or called the engine")
	}
	first := query.GetPositions(ids)
	if !slices.Equal(first, []float32{1, -4, 5, -6}) {
		t.Fatalf("positions = %v", first)
	}
	success = false
	if got := query.GetPositions(ids); got != nil {
		t.Fatalf("failed query returned stale positions: %v", got)
	}
	success = true
	if got := query.GetPositions(ids); &got[0] != &first[0] || got[0] != 3 {
		t.Fatal("query did not refresh its existing storage")
	}
	if got := query.GetPositions(nil); len(got) != 0 || calls != 3 {
		t.Fatal("empty query reached the engine")
	}
	if got := query.GetPositions(ids); &got[0] != &first[0] || got[0] != 4 {
		t.Fatal("empty query discarded reusable storage")
	}
	var other PositionQueryBuffer
	if got := other.GetPositions(ids); &got[0] == &first[0] || first[0] != 4 {
		t.Fatal("independent query buffers shared position storage")
	}
	ids = append(ids, 2, 3, 4, 5, 6, 7, 8)
	if got := query.GetPositions(ids); len(got) != len(ids)*2 || got[0] != float32(calls) {
		t.Fatal("position storage did not grow with the batch")
	}
}

func (spy *typedTransformSpy) SetTransform(id Object, position mathf.Vec2, rotation float64, scale mathf.Vec2, visible bool, offset mathf.Vec2) {
	spy.id = id
	spy.transform = SpriteTransform{Position: position, Rotation: rotation, Scale: scale, RenderOffset: offset, Visible: visible}
	spy.calls++
}

func (s *transformBatchSpy) BatchUpdateTransforms(data []float32) {
	s.batches = append(s.batches, slices.Clone(data))
}

func (s *positionQuerySpy) BatchRetrievePositions(ids []int64, out []float32) bool {
	return s.query(ids, out)
}
