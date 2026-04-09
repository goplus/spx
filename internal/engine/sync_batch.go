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

const (
	// Batch sync constants
	SyncFieldsPerSprite     = 9  // id, x, y, rotation, scaleX, scaleY, offsetX, offsetY, visibility
	DefaultDeleteBufferSize = 16 // initial capacity for sprite deletion buffer
)

// SpriteSyncData represents the data to sync for a single sprite
type SpriteSyncData struct {
	SpriteID int64
	X        float32
	Y        float32
	Rotation float32
	ScaleX   float32
	ScaleY   float32
	OffsetX  float32
	OffsetY  float32
	Visible  float32 // 0 or 1
}

// SpriteSyncBuffer collects sync data for batch processing
type SpriteSyncBuffer struct {
	data       []SpriteSyncData
	deleteIDs  []int64
	serialized []float32
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

// Add appends a sprite's sync data to the buffer
func (b *SpriteSyncBuffer) Add(id int64, x, y, rotation, scaleX, scaleY, offsetX, offsetY float64, visible bool) {
	vis := float32(0.0)
	if visible {
		vis = float32(1.0)
	}

	b.data = append(b.data, SpriteSyncData{
		SpriteID: id,
		X:        float32(x),
		Y:        float32(y),
		Rotation: float32(rotation),
		ScaleX:   float32(scaleX),
		ScaleY:   float32(scaleY),
		OffsetX:  float32(offsetX),
		OffsetY:  float32(offsetY),
		Visible:  vis,
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

// Serialize converts the buffer to a flat float32 array for FFI
// Format with header: [updateCount, deleteCount, update_data..., delete_ids...]
// - Header: [updateCount, deleteCount]
// - Update section: [id1, x1, y1, rot1, scaleX1, scaleY1, offsetX1, offsetY1, vis1, ...]
// - Delete section: [id1, id2, id3, ...]
func (b *SpriteSyncBuffer) Serialize() []float32 {
	updateCount := len(b.data)
	deleteCount := len(b.deleteIDs)

	if updateCount == 0 && deleteCount == 0 {
		return nil
	}

	// Calculate total size: header(2) + updates(updateCount * 9) + deletes(deleteCount * 1)
	totalSize := 2 + updateCount*SyncFieldsPerSprite + deleteCount
	b.serialized = ensureFloat32BufferSize(b.serialized, totalSize)
	result := b.serialized

	// Write header
	result[0] = float32(updateCount)
	result[1] = float32(deleteCount)

	idx := 2

	// Serialize update data
	for _, sprite := range b.data {
		result[idx] = float32(sprite.SpriteID)
		result[idx+1] = sprite.X
		result[idx+2] = sprite.Y
		result[idx+3] = sprite.Rotation
		result[idx+4] = sprite.ScaleX
		result[idx+5] = sprite.ScaleY
		result[idx+6] = sprite.OffsetX
		result[idx+7] = sprite.OffsetY
		result[idx+8] = sprite.Visible
		idx += SyncFieldsPerSprite
	}

	// Serialize delete IDs (only IDs, no wasted space)
	for _, id := range b.deleteIDs {
		result[idx] = float32(id)
		idx++
	}

	// The returned view is backed by reusable scratch storage and remains valid
	// only until the next buffer mutation.
	return result[:totalSize:totalSize]
}

// SyncBatchUpdateSprites sends batch sprite updates to Godot via a single FFI call
// This significantly reduces FFI overhead from O(N) to O(1) where N is the number of sprites
func SyncBatchUpdateSprites(buffer []float32) {
	if len(buffer) == 0 {
		return
	}

	// Send entire buffer to Godot in a single FFI call
	// The buffer is processed on the C++ side for optimal performance
	Managers().SpriteMgr.BatchUpdateTransforms(buffer)
}

// SyncBatchGetPositions retrieves positions for multiple sprites
// Format: [id1, id2, id3, ...] -> [x1, y1, x2, y2, x3, y3, ...]
func SyncBatchGetPositions(spriteIDs []int64) []float32 {
	if len(spriteIDs) == 0 {
		return nil
	}

	positions := Managers().SpriteMgr.BatchRetrievePositions(spriteIDs)
	f32Pos, _ := positions.([]float32)
	return f32Pos
}

// -------------------------------------------------------------------------------------
// Visual Sync Buffer - Batches SetRenderScale, SetZIndex, and UV remap updates
// -------------------------------------------------------------------------------------

const (
	// VisualFieldsPerSprite is the number of float32 fields per sprite in the visual buffer
	// [spriteId, renderScaleX, renderScaleY, zIndex, flags, uvX, uvY, uvW, uvH]
	VisualFieldsPerSprite = 9

	// Visual flags
	VisualFlagHasZIndex  = 1 // bit 0: apply SetZIndex
	VisualFlagHasUvRemap = 2 // bit 1: apply SetMaterialParamsVec4 for UV remap
)

// VisualSyncData represents the visual data to sync for a single sprite
type VisualSyncData struct {
	SpriteID    int64
	RenderScale float32
	ZIndex      int32
	Flags       int32
	UvRemap     [4]float32 // x, y, w, h (UV remap for atlas textures)
}

// VisualSyncBuffer collects visual sync data for batch processing
type VisualSyncBuffer struct {
	data       []VisualSyncData
	serialized []float32
}

// NewVisualSyncBuffer creates a new visual sync buffer
func NewVisualSyncBuffer(capacity int) *VisualSyncBuffer {
	return &VisualSyncBuffer{
		data:       make([]VisualSyncData, 0, capacity),
		serialized: make([]float32, 0, 1+capacity*VisualFieldsPerSprite),
	}
}

// AddRenderScale adds a render scale update to the buffer
func (b *VisualSyncBuffer) AddRenderScale(id int64, renderScale float64) {
	b.data = append(b.data, VisualSyncData{
		SpriteID:    id,
		RenderScale: float32(renderScale),
	})
}

// AddFull adds a full visual update (render scale + optional zIndex + optional UV remap)
func (b *VisualSyncBuffer) AddFull(id int64, renderScale float64, zIndex int, hasZIndex bool, uvRemap [4]float64, hasUvRemap bool) {
	entry := VisualSyncData{
		SpriteID:    id,
		RenderScale: float32(renderScale),
		ZIndex:      int32(zIndex),
	}
	if hasZIndex {
		entry.Flags |= VisualFlagHasZIndex
	}
	if hasUvRemap {
		entry.Flags |= VisualFlagHasUvRemap
		entry.UvRemap = [4]float32{
			float32(uvRemap[0]), float32(uvRemap[1]),
			float32(uvRemap[2]), float32(uvRemap[3]),
		}
	}
	b.data = append(b.data, entry)
}

// Clear resets the buffer for reuse
func (b *VisualSyncBuffer) Clear() {
	b.data = b.data[:0]
}

// Count returns the number of visual updates in the buffer
func (b *VisualSyncBuffer) Count() int {
	return len(b.data)
}

// Serialize converts the buffer to a flat float32 array for FFI
// Format: [count, entry0..., entry1..., ...]
// Each entry: [spriteId, renderScaleX, renderScaleY, zIndex, flags, uvX, uvY, uvW, uvH]
func (b *VisualSyncBuffer) Serialize() []float32 {
	count := len(b.data)
	if count == 0 {
		return nil
	}

	totalSize := 1 + count*VisualFieldsPerSprite
	b.serialized = ensureFloat32BufferSize(b.serialized, totalSize)
	result := b.serialized

	result[0] = float32(count)
	idx := 1

	for _, entry := range b.data {
		result[idx] = float32(entry.SpriteID)
		result[idx+1] = entry.RenderScale
		result[idx+2] = entry.RenderScale // scaleX == scaleY for render scale
		result[idx+3] = float32(entry.ZIndex)
		result[idx+4] = float32(entry.Flags)
		result[idx+5] = entry.UvRemap[0]
		result[idx+6] = entry.UvRemap[1]
		result[idx+7] = entry.UvRemap[2]
		result[idx+8] = entry.UvRemap[3]
		idx += VisualFieldsPerSprite
	}

	// The returned view is backed by reusable scratch storage and remains valid
	// only until the next buffer mutation.
	return result[:totalSize:totalSize]
}

// SyncBatchUpdateVisuals sends batch visual updates to Godot via a single FFI call
func SyncBatchUpdateVisuals(buffer []float32) {
	if len(buffer) == 0 {
		return
	}
	Managers().SpriteMgr.BatchUpdateVisuals(buffer)
}

func ensureFloat32BufferSize(buffer []float32, size int) []float32 {
	if cap(buffer) < size {
		newCap := size * 2
		if newCap < size || newCap < 16 {
			newCap = size
			if newCap < 16 {
				newCap = 16
			}
		}
		return make([]float32, size, newCap)
	}
	return buffer[:size]
}
