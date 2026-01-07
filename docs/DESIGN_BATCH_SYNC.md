# Design: Batch Synchronization for Sprite Updates

## Status
- **Status**: Proposed (Not Implemented)
- **Author**: SPX Team
- **Created**: 2026-01-04
- **Updated**: 2026-01-04

## Summary

This design document proposes optimizing sprite synchronization between Go and Godot engine by batching multiple sprite updates into a single FFI call per frame, reducing overhead from O(N) calls to O(1) where N is the number of sprites.

## Background

### Current Implementation

The current sprite synchronization in SPX uses individual FFI calls for each sprite property update:

```go
// Per sprite, per frame:
sprite.syncSprite.SetPosition(pos)      // FFI call 1
sprite.syncSprite.SetRotation(rotation) // FFI call 2
sprite.syncSprite.SetScale(scale)       // FFI call 3
sprite.syncSprite.SetVisible(visible)   // FFI call 4
```

**Cost Analysis:**
- 100 sprites: ~400 FFI calls/frame
- 1000 sprites: ~4000 FFI calls/frame
- At 60 FPS: 24,000 to 240,000 FFI calls/second

### Problem Statement

1. **High FFI Overhead**: Each FFI call incurs context switching between Go and C++
2. **Performance Bottleneck**: FFI overhead becomes significant with many sprites
3. **Cache Inefficiency**: Scattered memory access patterns reduce CPU cache utilization
4. **Scalability Concern**: Performance degrades linearly with sprite count

## Goals

### Primary Goals
1. Reduce FFI calls from O(N) to O(1) per frame
2. Maintain backward compatibility with existing code
3. Enable incremental migration to batch API

### Non-Goals
1. Changing the Godot rendering pipeline
2. Modifying sprite behavior or semantics
3. Breaking existing sprite synchronization logic

## Design

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Go Side (SPX)                        │
├─────────────────────────────────────────────────────────┤
│  Game.syncUpdateProxy()                                 │
│    ├── Create SyncBuffer                                │
│    ├── For each sprite:                                 │
│    │     └── Collect transform data                     │
│    └── SyncBatchUpdateSprites(buffer)                   │
│                         │                                │
│                         ▼                                │
│  ┌─────────────────────────────────────────┐            │
│  │ SpriteSyncBuffer                        │            │
│  │  - Serialize() → []float64              │            │
│  │  Format: [id,x,y,rot,sx,sy,ox,oy,vis]  │            │
│  └─────────────────────────────────────────┘            │
└─────────────────────────────────────────────────────────┘
                         │
                         │ Single FFI Call
                         ▼
┌─────────────────────────────────────────────────────────┐
│                  Godot Side (C++)                       │
├─────────────────────────────────────────────────────────┤
│  SpxSpriteMgr::BatchUpdateTransforms(buffer)            │
│    └── For each sprite in buffer:                       │
│          ├── Parse transform data                       │
│          ├── Apply position, rotation, scale            │
│          └── Update visibility                          │
└─────────────────────────────────────────────────────────┘
```

### Component Design

#### 1. SpriteSyncBuffer (Go)

**Location**: `internal/engine/sync_batch.go`

```go
type SpriteSyncData struct {
    SpriteID int64
    X        float64
    Y        float64
    Rotation float64
    ScaleX   float64
    ScaleY   float64
    OffsetX  float64
    OffsetY  float64
    Visible  float64  // 0.0 or 1.0
}

type SpriteSyncBuffer struct {
    data []SpriteSyncData
}

func (b *SpriteSyncBuffer) Add(id int64, x, y, rotation, scaleX, scaleY, offsetX, offsetY float64, visible bool)
func (b *SpriteSyncBuffer) Serialize() []float64
func (b *SpriteSyncBuffer) Clear()
```

**Responsibilities:**
- Collect sprite transform data
- Serialize to compact float64 array
- Memory management and reuse

#### 2. Buffer Format Specification

**Wire Format:**
```
[sprite1_id, x, y, rot, scaleX, scaleY, offsetX, offsetY, visible,
 sprite2_id, x, y, rot, scaleX, scaleY, offsetX, offsetY, visible,
 ...]
```

**Constraints:**
- Fields per sprite: 9 (constant `SyncFieldsPerSprite`)
- ID field: sprite object ID (int64 as float64)
- Visibility: 0.0 = hidden, 1.0 = visible
- All floats are float64 for precision

#### 3. FFI Layer

**Current Phase (Phase 1)**:
```go
func SyncBatchUpdateSprites(buffer []float64) {
    // Processes buffer on Go side
    // Sends individual updates via existing FFI
    for each sprite in buffer {
        SpriteMgr.SetPosition(id, pos)
        SpriteMgr.SetRotation(id, rot)
        SpriteMgr.SetScale(id, scale)
        SpriteMgr.SetVisible(id, vis)
    }
}
```

**Future Phase (Phase 3)**:
```go
func SyncBatchUpdateSprites(buffer []float64) {
    // Single FFI call passing entire buffer
    SpriteMgr.BatchUpdateTransforms(buffer)
}
```

#### 4. Godot Module (Future)

**Location**: `pkg/gdspx/godot/modules/spx/spx_sprite_mgr.cpp`

```cpp
void SpxSpriteMgr::batch_update_transforms(const PackedFloat64Array& buffer) {
    const int FIELDS_PER_SPRITE = 9;
    int count = buffer.size() / FIELDS_PER_SPRITE;
    
    for (int i = 0; i < count; i++) {
        int base = i * FIELDS_PER_SPRITE;
        GdObj sprite_id = (GdObj)buffer[base];
        
        SpxSprite* sprite = get_sprite(sprite_id);
        if (sprite) {
            Vector2 pos(buffer[base+1], buffer[base+2]);
            float rotation = buffer[base+3];
            Vector2 scale(buffer[base+4], buffer[base+5]);
            Vector2 offset(buffer[base+6], buffer[base+7]);
            bool visible = buffer[base+8] != 0.0;
            
            sprite->set_position(pos);
            sprite->set_rotation(rotation);
            sprite->set_scale(scale);
            sprite->set_pivot(offset);
            sprite->set_visible(visible);
        }
    }
}
```

## Implementation Plan

### Phase 1: Infrastructure (📋 Planned)

**Deliverables:**
- [ ] Create `SpriteSyncBuffer` structure
- [ ] Implement serialization/deserialization
- [ ] Add batch API stubs in `sync_batch.go`
- [ ] Integrate buffer infrastructure in `gdspx.go`

**Testing:**
- Unit tests for buffer operations
- Verify backward compatibility

### Phase 2: Go-Side Batching (📋 Planned)

**Deliverables:**
- [ ] Modify `Game.syncUpdateProxy()` to use buffer
- [ ] Collect all sprite data before sync
- [ ] Process buffer in batch (individual FFI calls for now)

**Testing:**
- Integration tests with multiple sprites
- Performance baseline measurements

### Phase 3: True Batch FFI (📋 Planned)

**Deliverables:**
- [ ] Implement `BatchUpdateTransforms()` in Godot C++
- [ ] Add method to `ISpriteMgr` interface
- [ ] Update code generator templates
- [ ] Switch to single FFI call

**Testing:**
- Performance benchmarks
- Stress tests with 1000+ sprites
- Visual verification tests

## Performance Analysis

### Expected Improvements

| Sprites | Current FFI Calls | Batch FFI Calls | Reduction |
|---------|------------------|-----------------|-----------|
| 10      | 40               | 1               | 97.5%     |
| 100     | 400              | 1               | 99.75%    |
| 1000    | 4000             | 1               | 99.975%   |

### Memory Impact

**Per Frame:**
- Buffer allocation: 9 × N × 8 bytes = 72N bytes
- Example: 1000 sprites = 72KB buffer

**Cache Efficiency:**
- Current: Scattered memory access across sprite objects
- Batch: Sequential memory access in single buffer
- Expected: Better CPU cache utilization

## Backward Compatibility

### Compatibility Guarantees

1. **API Compatibility**: All existing sprite methods continue to work
2. **Behavior Compatibility**: Sprite synchronization semantics unchanged
3. **Performance**: No performance regression for small sprite counts

### Migration Path

**Step 1**: Current code continues working (no changes needed)
```go
sprite.updateProxyTransform(true)  // Works as before
```

**Step 2**: Optional migration to buffer API
```go
// Can gradually migrate when ready
syncBuffer.Add(sprite.id, x, y, rotation, ...)
```

**Step 3**: Automatic optimization (Phase 3)
```go
// Same code, but uses batch FFI internally
// No code changes required
```

## Risks and Mitigations

### Risk 1: Godot Module Complexity
- **Impact**: Medium - C++ implementation may be complex
- **Mitigation**: Reference existing array handling code (GdArray)

### Risk 2: Debugging Difficulty
- **Impact**: Low - Batch processing harder to debug
- **Mitigation**: Add logging and validation in debug builds

### Risk 3: Memory Usage
- **Impact**: Low - Buffer allocation per frame
- **Mitigation**: Reuse buffer, preallocate capacity

## Testing Strategy

### Unit Tests
```go
func TestSpriteSyncBuffer_Serialize(t *testing.T)
func TestSpriteSyncBuffer_Empty(t *testing.T)
func TestSpriteSyncBuffer_SingleSprite(t *testing.t)
func TestSpriteSyncBuffer_MultipleSprites(t *testing.T)
```

### Integration Tests
```go
func TestBatchSync_100Sprites(t *testing.T)
func TestBatchSync_BackwardCompatibility(t *testing.T)
```

### Performance Benchmarks
```go
func BenchmarkIndividualSync_100Sprites(b *testing.B)
func BenchmarkBatchSync_100Sprites(b *testing.B)
func BenchmarkIndividualSync_1000Sprites(b *testing.B)
func BenchmarkBatchSync_1000Sprites(b *testing.B)
```

## Success Metrics

### Performance Metrics
- FFI call reduction: >95% for 100+ sprites
- Frame time improvement: 10-30% for sprite-heavy scenes
- Memory overhead: <100KB for 1000 sprites

### Quality Metrics
- Zero regression in existing tests
- 100% backward compatibility
- No visual glitches or synchronization issues

## Future Work

### Potential Optimizations
1. **Dirty Tracking**: Only sync changed sprites
2. **Delta Compression**: Send only changed fields
3. **Separate Channels**: Split position/rotation/scale updates
4. **Double Buffering**: Prepare next frame while rendering current

### Related Features
1. Batch physics updates
2. Batch animation updates
3. Batch audio commands

## References

### Code References
- `gdspx.go` - Main sync logic
- `internal/engine/sync_batch.go` - Batch implementation
- `pkg/gdspx/internal/ffi/gdextension_interface.go` - FFI layer
- `pkg/gdspx/godot/modules/spx/spx_sprite_mgr.cpp` - Godot sprite manager

### External References
- [Godot GDExtension Docs](https://docs.godotengine.org/en/stable/tutorials/scripting/gdextension/index.html)
- [Go CGO Performance](https://dave.cheney.net/2016/01/18/cgo-is-not-go)
- [Batch Rendering Patterns](https://en.wikipedia.org/wiki/Batch_rendering)

## Appendix

### A. Buffer Layout Example

For 2 sprites with IDs 101 and 102:
```
[
  101.0,  // Sprite 1 ID
  100.0,  // x
  200.0,  // y
  45.0,   // rotation (degrees)
  1.0,    // scaleX
  1.0,    // scaleY
  0.0,    // offsetX
  0.0,    // offsetY
  1.0,    // visible
  
  102.0,  // Sprite 2 ID
  150.0,  // x
  250.0,  // y
  90.0,   // rotation
  2.0,    // scaleX
  2.0,    // scaleY
  5.0,    // offsetX
  5.0,    // offsetY
  1.0     // visible
]
```

### B. Implementation Checklist

**Phase 1** (📋 Planned):
- [ ] Create SpriteSyncBuffer struct
- [ ] Implement Add() method
- [ ] Implement Serialize() method
- [ ] Implement Clear() method
- [ ] Add SyncBatchUpdateSprites() stub
- [ ] Add SyncBatchGetPositions() stub
- [ ] Integrate in Game.syncUpdateProxy()
- [ ] Write documentation

**Phase 2** (📋 Planned):
- [ ] Test buffer serialization
- [ ] Verify no performance regression
- [ ] Add unit tests
- [ ] Add benchmarks

**Phase 3** (📋 Planned):
- [ ] Design Godot C++ API
- [ ] Implement BatchUpdateTransforms in C++
- [ ] Add to ISpriteMgr interface
- [ ] Update code generation
- [ ] Performance testing
- [ ] Documentation update

---

**Last Updated**: 2026-01-04  
**Next Review**: After design approval and Phase 1 planning
