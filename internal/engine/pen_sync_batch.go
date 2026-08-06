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

// PenBatchFields is the number of float32 lanes in one pen command.
// [op, objectIDLowBits, objectIDHighBits, a, b, c, d, reserved]
const PenBatchFields = 8

const maxPenBatchCommands = 1024

const (
	PenBatchMove = 1 + iota
	PenBatchDown
	PenBatchUp
	PenBatchColor
	PenBatchSetSize
)

// PenSyncBuffer preserves the global order of pen commands while reducing
// Web bridge calls to bounded, reusable float32 batches.
type PenSyncBuffer struct {
	count int
	data  []float32
}

func NewPenSyncBuffer(capacity int) *PenSyncBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &PenSyncBuffer{
		data: make([]float32, 1, 1+capacity*PenBatchFields),
	}
}

func (b *PenSyncBuffer) AddMove(obj Object, x, y float64) bool {
	return b.add(PenBatchMove, obj, x, y, 0, 0)
}

func (b *PenSyncBuffer) AddDown(obj Object, moveByMouse bool) bool {
	return b.add(PenBatchDown, obj, boolArg(moveByMouse), 0, 0, 0)
}

func (b *PenSyncBuffer) AddUp(obj Object) bool {
	return b.add(PenBatchUp, obj, 0, 0, 0, 0)
}

func (b *PenSyncBuffer) AddColor(obj Object, r, g, blue, a float64) bool {
	return b.add(PenBatchColor, obj, r, g, blue, a)
}

func (b *PenSyncBuffer) AddSetSize(obj Object, size float64) bool {
	return b.add(PenBatchSetSize, obj, size, 0, 0, 0)
}

func (b *PenSyncBuffer) add(command int, obj Object, a, bval, c, d float64) bool {
	low, high := splitInt64BitsToFloat32(int64(obj))

	b.data = append(b.data,
		float32(command),
		low,
		high,
		float32(a),
		float32(bval),
		float32(c),
		float32(d),
		0,
	)
	b.count++
	return b.count >= maxPenBatchCommands
}

// Flush exposes the reusable command slice synchronously and clears it only
// after send returns.
func (b *PenSyncBuffer) Flush(send func([]float32)) {
	if b == nil || send == nil {
		return
	}

	b.flush(send)
}

// Barrier flushes queued commands before running an immediate engine operation.
func (b *PenSyncBuffer) Barrier(send func([]float32), operation func()) {
	if operation == nil {
		return
	}
	if b == nil || send == nil {
		operation()
		return
	}

	b.flush(send)
	operation()
}

// Discard removes pending commands without sending them to the engine.
func (b *PenSyncBuffer) Discard() {
	if b == nil {
		return
	}
	b.reset()
}

func (b *PenSyncBuffer) reset() {
	b.count = 0
	b.data = b.data[:1]
	b.data[0] = 0
}

func (b *PenSyncBuffer) flush(send func([]float32)) {
	if b.count == 0 {
		return
	}
	b.data[0] = float32(b.count)
	send(b.data[:len(b.data):len(b.data)])
	b.reset()
}
