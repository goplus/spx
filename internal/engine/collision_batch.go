package engine

import (
	gdx "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

// CollisionPair represents a pair of sprites to check for collision
type CollisionPair struct {
	SpriteA int64
	SpriteB int64
}

// CollisionBatchBuffer collects collision pairs for batch processing
type CollisionBatchBuffer struct {
	pairs []CollisionPair
}

// NewCollisionBatchBuffer creates a new collision batch buffer
func NewCollisionBatchBuffer(capacity int) *CollisionBatchBuffer {
	return &CollisionBatchBuffer{
		pairs: make([]CollisionPair, 0, capacity),
	}
}

// AddPair appends a collision pair to the buffer
func (b *CollisionBatchBuffer) AddPair(spriteA, spriteB int64) {
	b.pairs = append(b.pairs, CollisionPair{
		SpriteA: spriteA,
		SpriteB: spriteB,
	})
}

// Clear resets the buffer for reuse
func (b *CollisionBatchBuffer) Clear() {
	b.pairs = b.pairs[:0]
}

// Count returns the number of collision pairs in the buffer
func (b *CollisionBatchBuffer) Count() int {
	return len(b.pairs)
}

// Serialize converts the buffer to a flat float32 array for FFI
// Format: [count, spriteA1, spriteB1, spriteA2, spriteB2, ...]
func (b *CollisionBatchBuffer) Serialize() []float32 {
	count := len(b.pairs)
	if count == 0 {
		return nil
	}

	// Calculate total size: header(1) + pairs(count * 2)
	totalSize := 1 + count*2
	result := make([]float32, totalSize)

	// Write header
	result[0] = float32(count)

	idx := 1
	// Serialize collision pairs
	for _, pair := range b.pairs {
		result[idx] = float32(pair.SpriteA)
		result[idx+1] = float32(pair.SpriteB)
		idx += 2
	}

	return result
}

// BatchCheckCollisions sends batch collision checks to C++ and returns results
// Returns a slice of booleans indicating collision status for each pair
// TODO: Implement C++ side batch collision checking for better performance
func BatchCheckCollisions(buffer []float32, alphaThreshold float64, usePixelPerfect bool) []bool {
	if len(buffer) == 0 {
		return nil
	}

	// Parse buffer to get collision pairs
	count := int(buffer[0])
	boolResults := make([]bool, count)

	// For now, use individual checks (C++ batch API will be added later)
	// This still helps by organizing the collision checks in one place
	idx := 1
	for i := 0; i < count; i++ {
		spriteA := gdx.Object(buffer[idx])
		spriteB := gdx.Object(buffer[idx+1])
		boolResults[i] = gdx.SpriteMgr.CheckCollisionWithSpriteByAlpha(spriteA, spriteB, alphaThreshold, usePixelPerfect)
		idx += 2
	}

	return boolResults
}
