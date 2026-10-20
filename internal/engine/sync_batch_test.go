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
	"math"
	"testing"
)

func TestSpriteSyncBufferSerializeReusesBuffer(t *testing.T) {
	buffer := NewSpriteSyncBuffer(1)
	buffer.Add(1, 2, 3, 4, 5, 6, 7, 8, true)
	buffer.AddDelete(9)
	if buffer.UpdateCount() != 1 || buffer.DeleteCount() != 1 || len(buffer.GetDeleteIDs()) != 1 || buffer.GetDeleteIDs()[0] != 9 {
		t.Fatal("buffer counts or deletion IDs incorrect")
	}

	first := buffer.Serialize()
	if len(first) == 0 {
		t.Fatal("Serialize returned an empty buffer")
	}
	firstPtr := &first[0]

	buffer.Clear()
	if buffer.UpdateCount() != 0 || buffer.DeleteCount() != 0 || len(buffer.GetDeleteIDs()) != 0 || buffer.Serialize() != nil {
		t.Fatal("clear retained buffer entries")
	}
	buffer.Add(10, 11, 12, 13, 14, 15, 16, 17, false)

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

func TestBatchBufferReuseAndGrowth(t *testing.T) {
	for _, tc := range []struct {
		name   string
		buffer []float32
		size   int
	}{
		{"nil empty", nil, 0},
		{"allocate minimum", nil, 1},
		{"allocate larger", nil, 17},
		{"shrink", make([]float32, 4, 8), 2},
		{"reuse capacity", make([]float32, 2, 8), 8},
		{"grow capacity", make([]float32, 2, 8), 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.buffer) != 0 {
				tc.buffer[0] = 42
			}
			got := ensureFloat32BufferSize(tc.buffer, tc.size)
			if len(got) != tc.size || cap(got) < tc.size {
				t.Fatalf("length/capacity = %d/%d, requested %d", len(got), cap(got), tc.size)
			}
			if tc.size > cap(tc.buffer) && cap(got) <= tc.size {
				t.Fatalf("growth capacity = %d, want headroom beyond %d", cap(got), tc.size)
			}
			if tc.buffer == nil && tc.size == 0 && got != nil {
				t.Fatal("zero-sized nil buffer was allocated")
			}
			if tc.size == 0 || len(tc.buffer) == 0 {
				return
			}
			if tc.size <= cap(tc.buffer) {
				if &got[0] != &tc.buffer[0] || got[0] != 42 {
					t.Fatal("available capacity was not reused with its contents intact")
				}
			} else {
				got[0] = 99
				if tc.buffer[0] != 42 {
					t.Fatal("growing the buffer overwrote the original storage")
				}
			}
		})
	}
	if boolArg(false) != 0 || boolArg(true) != 1 {
		t.Fatal("boolean batch argument encoding changed")
	}
}

func FuzzSplitInt64BatchBits(f *testing.F) {
	// The final two seeds carry infinity and NaN bit patterns in both halves.
	for _, value := range []int64{0, 1, -1, math.MinInt64, math.MaxInt64, 1 << 32, 0x7f8000007f800000, 0x7fc000007fc00000} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value int64) {
		low, high := splitInt64BitsToFloat32(value)
		got := int64(uint64(math.Float32bits(high))<<32 | uint64(math.Float32bits(low)))
		if got != value {
			t.Fatalf("batch ID round trip = %#016x, want %#016x", got, value)
		}
	})
}
