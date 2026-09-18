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

import "math"

const (
	SyncFieldsPerSprite     = 9  // id, x, y, rotation, scaleX, scaleY, renderOffsetX, renderOffsetY, visibility
	DefaultDeleteBufferSize = 16 // initial capacity for sprite deletion buffer
)

// SpriteSyncData represents the data to sync for a single sprite
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

// SpriteSyncBuffer collects sync data for batch processing
type SpriteSyncBuffer struct {
	data       []SpriteSyncData
	deleteIDs  []int64
	serialized []float32
	positions  []float32
}

// Add appends a sprite's sync data to the buffer
func (b *SpriteSyncBuffer) Add(id int64, x, y, rotation, scaleX, scaleY, renderOffsetX, renderOffsetY float64, visible bool) {
	vis := float32(0.0)
	if visible {
		vis = float32(1.0)
	}

	b.data = append(b.data, SpriteSyncData{
		SpriteID:      id,
		X:             float32(x),
		Y:             float32(y),
		Rotation:      float32(rotation),
		ScaleX:        float32(scaleX),
		ScaleY:        float32(scaleY),
		RenderOffsetX: float32(renderOffsetX),
		RenderOffsetY: float32(renderOffsetY),
		Visible:       vis,
	})
}

// AddDelete appends a sprite ID to the deletion list
func (b *SpriteSyncBuffer) AddDelete(id int64) {
	b.deleteIDs = append(b.deleteIDs, id)
}

// Clear resets the buffer for reuse
func (b *SpriteSyncBuffer) Clear() {
	b.data = b.data[:0]
	b.deleteIDs = b.deleteIDs[:0]
}

// UpdateCount returns the number of sprite updates in the buffer
func (b *SpriteSyncBuffer) UpdateCount() int {
	return len(b.data)
}

// DeleteCount returns the number of sprite deletions in the buffer
func (b *SpriteSyncBuffer) DeleteCount() int {
	return len(b.deleteIDs)
}

// GetDeleteIDs returns the list of sprite IDs to be deleted
func (b *SpriteSyncBuffer) GetDeleteIDs() []int64 {
	return b.deleteIDs
}

// Serialize encodes [updateCount, deleteCount, updates..., deleteIDs...] for FFI.
// Each update contains [id, x, y, rotation, scaleX, scaleY, offsetX, offsetY, visible].
// Panics before returning a packet if an ID is negative or cannot be represented
// exactly in the legacy float32 format.
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
		result[idx+1] = sprite.X
		result[idx+2] = sprite.Y
		result[idx+3] = sprite.Rotation
		result[idx+4] = sprite.ScaleX
		result[idx+5] = sprite.ScaleY
		result[idx+6] = sprite.RenderOffsetX
		result[idx+7] = sprite.RenderOffsetY
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

// GetPositions retrieves x/y pairs into storage owned by this sync buffer.
// The result is valid until the next GetPositions call on the same buffer.
func (b *SpriteSyncBuffer) GetPositions(spriteIDs []int64) []float32 {
	if len(spriteIDs) > math.MaxInt32/2 {
		panic("position batch exceeds the native array length limit")
	}
	b.positions = ensureFloat32BufferSize(b.positions, len(spriteIDs)*2)
	if len(spriteIDs) > 0 && !Managers().SpriteMgr.BatchRetrievePositions(spriteIDs, b.positions) {
		return nil
	}
	return b.positions
}

// NewSpriteSyncBuffer creates a new sync buffer
func NewSpriteSyncBuffer(capacity int) *SpriteSyncBuffer {
	serializedCapacity := 2 + capacity*SyncFieldsPerSprite + DefaultDeleteBufferSize
	return &SpriteSyncBuffer{
		data:       make([]SpriteSyncData, 0, capacity),
		deleteIDs:  make([]int64, 0, DefaultDeleteBufferSize),
		serialized: make([]float32, 0, serializedCapacity),
	}
}

// SyncBatchUpdateSprites sends sprite updates to Godot in one FFI call.
func SyncBatchUpdateSprites(buffer []float32) {
	if len(buffer) == 0 {
		return
	}

	Managers().SpriteMgr.BatchUpdateTransforms(buffer)
}
