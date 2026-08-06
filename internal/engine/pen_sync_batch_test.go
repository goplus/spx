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

func TestPenSyncBufferFlushesOneOrderedBatch(t *testing.T) {
	buffer := NewPenSyncBuffer(1)
	const firstID = Object(0x1122334455667788)
	const secondID = Object(0x0102030405060708)

	buffer.AddColor(firstID, 0.1, 0.2, 0.3, 0.4)
	buffer.AddMove(secondID, 10, -20)
	buffer.AddDown(firstID, false)
	buffer.AddSetSize(secondID, 8)
	buffer.AddUp(firstID)

	var got []float32
	buffer.Flush(func(data []float32) {
		got = append(got, data...)
		if cap(data) != len(data) {
			t.Fatalf("batch cap = %d, want len %d", cap(data), len(data))
		}
	})

	if want := 1 + 5*PenBatchFields; len(got) != want {
		t.Fatalf("batch len = %d, want %d", len(got), want)
	}
	if got[0] != 5 {
		t.Fatalf("command count = %v, want 5", got[0])
	}

	assertPenCommand(t, got, 0, PenBatchColor, firstID, [4]float32{0.1, 0.2, 0.3, 0.4})
	assertPenCommand(t, got, 1, PenBatchMove, secondID, [4]float32{10, -20})
	assertPenCommand(t, got, 2, PenBatchDown, firstID, [4]float32{})
	assertPenCommand(t, got, 3, PenBatchSetSize, secondID, [4]float32{8})
	assertPenCommand(t, got, 4, PenBatchUp, firstID, [4]float32{})
}

func TestPenSyncBufferPreservesObjectIDBits(t *testing.T) {
	buffer := NewPenSyncBuffer(1)
	const objectID = Object(-0x0edcba9876543211)
	buffer.AddUp(objectID)

	buffer.Flush(func(data []float32) {
		if got, want := math.Float32bits(data[2]), uint32(0x89abcdef); got != want {
			t.Fatalf("object id low bits = %x, want %x", got, want)
		}
		if got, want := math.Float32bits(data[3]), uint32(0xf1234567); got != want {
			t.Fatalf("object id high bits = %x, want %x", got, want)
		}
	})
}

func TestPenSyncBufferReusesStorageAndSkipsEmptyFlush(t *testing.T) {
	buffer := NewPenSyncBuffer(1)
	buffer.AddUp(1)

	var firstPtr *float32
	buffer.Flush(func(data []float32) {
		firstPtr = &data[0]
	})

	called := false
	buffer.Flush(func([]float32) { called = true })
	if called {
		t.Fatal("empty buffer triggered a batch")
	}

	buffer.AddSetSize(1, 8)
	buffer.Flush(func(data []float32) {
		if firstPtr != &data[0] {
			t.Fatal("flush allocated a new backing array instead of reusing storage")
		}
	})
}

func TestPenSyncBufferDiscard(t *testing.T) {
	buffer := NewPenSyncBuffer(1)
	buffer.AddMove(1, 2, 3)
	buffer.Discard()

	called := false
	buffer.Flush(func([]float32) { called = true })
	if called {
		t.Fatal("discarded commands were flushed")
	}
}

func TestPenSyncBufferSignalsBatchLimit(t *testing.T) {
	buffer := NewPenSyncBuffer(1)
	for i := 0; i < maxPenBatchCommands-1; i++ {
		if buffer.AddUp(Object(i + 1)) {
			t.Fatalf("command %d reached the batch limit too early", i+1)
		}
	}
	if !buffer.AddUp(maxPenBatchCommands) {
		t.Fatalf("command %d did not reach the batch limit", maxPenBatchCommands)
	}
}

func TestPenSyncBufferBarrierFlushesBeforeOperation(t *testing.T) {
	buffer := NewPenSyncBuffer(1)
	buffer.AddUp(1)
	var events []string
	buffer.Barrier(func(data []float32) {
		events = append(events, "batch")
		if got, want := int(data[1]), PenBatchUp; got != want {
			t.Fatalf("batched operation = %d, want %d", got, want)
		}
	}, func() {
		events = append(events, "operation")
	})

	if got, want := len(events), 2; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	if events[0] != "batch" || events[1] != "operation" {
		t.Fatalf("events = %v, want [batch operation]", events)
	}
}

func assertPenCommand(t *testing.T, data []float32, index, operation int, objectID Object, args [4]float32) {
	t.Helper()
	record := data[1+index*PenBatchFields : 1+(index+1)*PenBatchFields]
	if got := int(record[0]); got != operation {
		t.Fatalf("command %d operation = %d, want %d", index, got, operation)
	}
	low, high := splitInt64BitsToFloat32(int64(objectID))
	if got, want := math.Float32bits(record[1]), math.Float32bits(low); got != want {
		t.Fatalf("command %d object low bits = %x, want %x", index, got, want)
	}
	if got, want := math.Float32bits(record[2]), math.Float32bits(high); got != want {
		t.Fatalf("command %d object high bits = %x, want %x", index, got, want)
	}
	for i, want := range args {
		if got := record[3+i]; got != want {
			t.Fatalf("command %d arg %d = %v, want %v", index, i, got, want)
		}
	}
	if record[7] != 0 {
		t.Fatalf("command %d reserved lane = %v, want 0", index, record[7])
	}
}
