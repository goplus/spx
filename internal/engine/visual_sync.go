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
	// VisualFieldsPerSprite is the number of float32 fields per sprite in the visual buffer
	// [spriteId, renderScaleX, renderScaleY, zIndex, flags, uvX, uvY, uvW, uvH]
	VisualFieldsPerSprite = 9

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
// Panics before returning a packet if an ID is negative or cannot be represented
// exactly in the legacy float32 format.
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
		result[idx] = encodeLegacyBatchObjectID(entry.SpriteID)
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

// NewVisualSyncBuffer creates a new visual sync buffer
func NewVisualSyncBuffer(capacity int) *VisualSyncBuffer {
	return &VisualSyncBuffer{
		data:       make([]VisualSyncData, 0, capacity),
		serialized: make([]float32, 0, 1+capacity*VisualFieldsPerSprite),
	}
}

// SyncBatchUpdateVisuals sends batch visual updates to Godot via a single FFI call
func SyncBatchUpdateVisuals(buffer []float32) {
	if len(buffer) == 0 {
		return
	}
	Managers().SpriteMgr.BatchUpdateVisuals(buffer)
}
