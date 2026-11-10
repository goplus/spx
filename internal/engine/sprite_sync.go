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

	"github.com/goplus/spbase/mathf"
)

const (
	SyncFieldsPerSprite     = 9 // id, x, y, rotation, scaleX, scaleY, renderOffsetX, renderOffsetY, visibility
	DefaultDeleteBufferSize = 16
)

// SpriteTransform describes the geometry shared by direct and batch updates.
type SpriteTransform struct {
	Position     mathf.Vec2
	Rotation     float64
	Scale        mathf.Vec2
	RenderOffset mathf.Vec2
	Visible      bool
}

type SpriteSyncData struct {
	SpriteID      int64
	X             float32
	Y             float32
	Rotation      float32
	ScaleX        float32
	ScaleY        float32
	RenderOffsetX float32 // local SPX position of RenderRoot
	RenderOffsetY float32 // local SPX position of RenderRoot
	Visible       float32 // 0 or 1
}

type SpriteSyncBuffer struct {
	data       []SpriteSyncData
	deleteIDs  []int64
	serialized []float32
}

// PositionQueryBuffer owns reusable storage for batched engine position reads.
// Its zero value is ready to use. Concurrent or reentrant queries need separate buffers.
type PositionQueryBuffer struct {
	positions []float32
}

// Validate rejects transforms that become non-finite in the float32 protocol.
func (transform SpriteTransform) Validate(id int64) {
	values := [...]float64{transform.Position.X, transform.Position.Y, transform.Rotation,
		transform.Scale.X, transform.Scale.Y, transform.RenderOffset.X, transform.RenderOffset.Y}
	for field, value := range values {
		validateTransformField(id, field, value)
	}
}

// ApplyTransform validates and submits a transform. The caller must be on the engine main thread.
func (sprite *Sprite) ApplyTransform(transform SpriteTransform) {
	transform.Validate(sprite.Id)
	sprite.SetTransform(transform.Position, transform.Rotation, transform.Scale, transform.Visible, transform.RenderOffset)
}

func NewSpriteSyncBuffer(capacity int) *SpriteSyncBuffer {
	serializedCapacity := 2 + capacity*SyncFieldsPerSprite + DefaultDeleteBufferSize
	return &SpriteSyncBuffer{
		data:       make([]SpriteSyncData, 0, capacity),
		deleteIDs:  make([]int64, 0, DefaultDeleteBufferSize),
		serialized: make([]float32, 0, serializedCapacity),
	}
}

// AddTransform collects an update; Serialize validates it before submission.
func (b *SpriteSyncBuffer) AddTransform(id int64, transform SpriteTransform) {
	b.data = append(b.data, SpriteSyncData{
		SpriteID:      id,
		X:             float32(transform.Position.X),
		Y:             float32(transform.Position.Y),
		Rotation:      float32(transform.Rotation),
		ScaleX:        float32(transform.Scale.X),
		ScaleY:        float32(transform.Scale.Y),
		RenderOffsetX: float32(transform.RenderOffset.X),
		RenderOffsetY: float32(transform.RenderOffset.Y),
		Visible:       float32(boolArg(transform.Visible)),
	})
}

func (b *SpriteSyncBuffer) AddDelete(id int64) {
	b.deleteIDs = append(b.deleteIDs, id)
}

func (b *SpriteSyncBuffer) UpdateCount() int {
	return len(b.data)
}

func (b *SpriteSyncBuffer) DeleteCount() int {
	return len(b.deleteIDs)
}

func (b *SpriteSyncBuffer) GetDeleteIDs() []int64 {
	return b.deleteIDs
}

func (b *SpriteSyncBuffer) Clear() {
	b.data = b.data[:0]
	b.deleteIDs = b.deleteIDs[:0]
}

// Serialize encodes [updateCount, deleteCount, updates..., deleteIDs...] for FFI.
// Each update contains [id, x, y, rotation, scaleX, scaleY, offsetX, offsetY, visible].
// Panics on negative or inexact float32 IDs, or non-finite float32 transforms.
func (b *SpriteSyncBuffer) Serialize() []float32 {
	updateCount := len(b.data)
	deleteCount := len(b.deleteIDs)

	if updateCount == 0 && deleteCount == 0 {
		return nil
	}

	totalSize := 2 + updateCount*SyncFieldsPerSprite + deleteCount
	b.serialized = ensureFloat32BufferSize(b.serialized, totalSize)
	result := b.serialized

	result[0] = float32(updateCount)
	result[1] = float32(deleteCount)

	idx := 2

	for _, sprite := range b.data {
		result[idx] = encodeLegacyBatchObjectID(sprite.SpriteID)
		values := [...]float32{sprite.X, sprite.Y, sprite.Rotation, sprite.ScaleX, sprite.ScaleY, sprite.RenderOffsetX, sprite.RenderOffsetY}
		for field, value := range values {
			validateTransformField(sprite.SpriteID, field, float64(value))
			result[idx+1+field] = value
		}
		result[idx+8] = sprite.Visible
		idx += SyncFieldsPerSprite
	}

	for _, id := range b.deleteIDs {
		result[idx] = encodeLegacyBatchObjectID(id)
		idx++
	}

	// The returned view is backed by reusable scratch storage and remains valid
	// only until the next buffer mutation.
	return result[:totalSize:totalSize]
}

// SyncBatchUpdateSprites sends sprite updates to Godot in one FFI call.
func SyncBatchUpdateSprites(buffer []float32) {
	if len(buffer) == 0 {
		return
	}

	Managers().SpriteMgr.BatchUpdateTransforms(buffer)
}

// GetPositions retrieves x/y pairs. The result remains valid until the next query
// on this buffer. A failed query returns nil without discarding reusable storage.
func (b *PositionQueryBuffer) GetPositions(spriteIDs []int64) []float32 {
	if len(spriteIDs) > math.MaxInt32/2 {
		panic("position batch exceeds the native array length limit")
	}
	b.positions = ensureFloat32BufferSize(b.positions, len(spriteIDs)*2)
	if len(spriteIDs) > 0 && !Managers().SpriteMgr.BatchRetrievePositions(spriteIDs, b.positions) {
		return nil
	}
	return b.positions
}

func validateTransformField(id int64, field int, value float64) {
	converted := float64(float32(value))
	if math.IsNaN(converted) || math.IsInf(converted, 0) {
		name := [...]string{"x", "y", "rotation", "scale x", "scale y", "offset x", "offset y"}[field]
		panic(fmt.Sprintf("sprite %d transform %s=%v must be finite after float32 conversion", id, name, value))
	}
}
