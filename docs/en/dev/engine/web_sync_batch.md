# Batched Synchronization for Web

Web mode crosses Go WASM, JavaScript, and the C++ engine boundary frequently. Batched synchronization groups fixed-layout, high-frequency data into shared transfer buffers to reduce calls, allocation, and per-field marshalling.

## Directions

- Go to engine: gameplay state, transforms, visual state, and physics configuration.
- Engine to Go: physics results, input snapshots, collision events, and trigger events.

Each batch has explicit ownership and a synchronization point. Producers finish writing before consumers read, and buffers are reused only after consumption.

## Currently batched data

### Sprite transforms

Position, rotation, scale, and related transform state are packed for all changed sprites and applied together.

`Game.OnEngineRender` submits the final batch after the frame's single
scheduler update, including changes made by eligible collision handlers. Each
submission owns its acknowledgment records; dirty state, pending position writes,
and deletions are acknowledged only after it succeeds. Geometry queries may still
synchronize an individual sprite earlier.

### Sprite visuals

Frequently changing visibility and render properties use compact records instead of independent bridge calls.

### Physics position pull

Physics-driven positions are read back in one batch and converted to SPX coordinates at the engine boundary.

Readback precedes condition sampling at frame start. Explicit, unsubmitted Go
position writes are preserved per axis; visibility changes do not reserve a
coordinate. See [frame phases](scheduling.md#frame-phases).

### Input snapshot

The input state consumed by one logical SPX tick is transferred as a coherent snapshot.

### Collision and trigger queue

Events are appended to a bounded/managed queue and drained by Go in batches. Event order within the queue must remain stable.

### Sprite physics configuration

Changed physics properties are grouped so the engine sees a consistent configuration update.

## Good candidates

- pen and debug-draw primitives;
- numeric UI layout data;
- large tile-map edits;
- other fixed-layout, high-frequency records with clear frame ownership.

## Poor candidates

Do not put irregular strings, long-lived object references, rare control operations, or data with ambiguous lifetime into shared memory merely to avoid a call. Message-based or direct APIs are clearer for low-frequency operations.

## Transfer buffer

The current implementation maintains reusable transfer storage and compact record layouts. Capacity growth, record count, and validity must be checked on both sides. Never retain a pointer/view beyond the documented synchronization window because a later batch may reuse or grow the buffer.

Transform, deletion, and visual batches use a legacy format that stores sprite IDs as numeric `float32` values. Their serializers require non-negative IDs that round-trip exactly; an invalid ID causes a panic containing the original ID before the packet is submitted. The engine callback panic handler reports the failure and requests runtime exit or reset. This guard preserves the existing wire format; physics batches already encode IDs losslessly in two bit lanes.

Query and batch transform submissions share a finite-value check after conversion
to `float32`, including overflow from finite Go values. A failure identifies the
sprite ID, field, and value, and does not acknowledge pending writes. Public heading
setters ignore NaN and infinity, preserving the last valid heading; the submission
check remains a guard against invalid internal state. Godot rejects an entire
invalid batch, so checking before submission prevents false acknowledgments.

## Priorities

Optimize measured hot paths first. Preserve semantics, ordering, and diagnostics before reducing bridge calls. Add representative Web benchmarks and test normal and worker modes when changing synchronization behavior.
