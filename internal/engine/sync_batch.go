package engine

import (
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

const (
	// Batch sync constants
	SyncFieldsPerSprite = 9 // id, x, y, rotation, scaleX, scaleY, offsetX, offsetY, visibility
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
	data      []SpriteSyncData
	deleteIDs []int64
}

// NewSpriteSyncBuffer creates a new sync buffer
func NewSpriteSyncBuffer(capacity int) *SpriteSyncBuffer {
	return &SpriteSyncBuffer{
		data:      make([]SpriteSyncData, 0, capacity),
		deleteIDs: make([]int64, 0, 16),
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
	result := make([]float32, totalSize)

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

	return result
}

// SyncBatchUpdateSprites sends batch sprite updates to Godot via a single FFI call
// This significantly reduces FFI overhead from O(N) to O(1) where N is the number of sprites
func SyncBatchUpdateSprites(buffer []float32) {
	if len(buffer) == 0 {
		return
	}

	// Send entire buffer to Godot in a single FFI call
	// The buffer is processed on the C++ side for optimal performance
	gdx.SpriteMgr.BatchUpdateTransforms(buffer)
}

// SyncBatchGetPositions retrieves positions for multiple sprites
// Format: [id1, id2, id3, ...] -> [x1, y1, x2, y2, x3, y3, ...]
func SyncBatchGetPositions(spriteIDs []int64) []float32 {
	if len(spriteIDs) == 0 {
		return nil
	}

	result := make([]float32, len(spriteIDs)*2)
	for i, id := range spriteIDs {
		pos := gdx.SpriteMgr.GetPosition(gdx.Object(id))
		result[i*2] = float32(pos.X)
		result[i*2+1] = float32(pos.Y)
	}
	return result
}
