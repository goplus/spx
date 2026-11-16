# Input Recording and Replay

Input replay records a game session as deterministic, tick-indexed input and replays it for capture and regression testing. Version 1 targets the Web host and intentionally records only the data required by the current SPX input model.

## Web host API

### Record a session

The host starts recording before or as a game session launches. Input snapshots are attached to SPX input ticks and collected until the game ends or recording is stopped. The result includes format/version metadata and the recorded tick stream.

### Replay a session

The host loads a recording before launch and starts the game with replay enabled. While replay is active, recorded snapshots replace live gameplay input. Host controls that manage the run remain outside the replay input stream.

### Defaults and overrides

The template host provides default replay behavior. An embedding page may override launch configuration, storage, naming, capture timing, and result handling without changing the low-level Go replay format.

## Recommended host workflow

1. Launch a clean game session.
2. Start recording and interact with the game.
3. Stop and save the versioned recording.
4. Relaunch from a clean state with deterministic environment settings.
5. Load and replay the recording.
6. Replay the recorded capture-key presses or request screenshots from game scripts.
7. Compare output with the baseline.

Do not replay into an already-running game because prior state is not part of the input recording.

## Input ticks

Replay uses the SPX input tick, not browser event timestamps or render frames. All input consumed by one logical update belongs to the same tick. This keeps the stream stable when browser scheduling or rendering frequency changes.

Live input, recording, and replay freeze their input batch in `OnEngineBeforeUpdate`,
before condition sampling. After clock advancement, `OnEngineUpdate` completes
event routing and handler registration before script scheduling. The same-frame
guarantee does not cover handler completion; see
[Script scheduling](scheduling.md#input-and-timer-frame-boundaries) for the limits.

Sharing an event batch does not freeze every input query. With live input or
recording, `MouseX` / `MouseY` read the sampled position, `KeyPressed` calls the
backend's `GetKey`, and `MousePressed` reads the current atomic button state.
During replay, these queries read the resolved session state. The sampling barrier
only excludes scripts; external input arriving after the snapshot cutoff can still
change live query results.

## Version 1 data

The v1 stream covers the SPX input snapshot used by game logic, including supported pointer, keyboard, and action state. The exact schema is versioned; readers must reject incompatible future formats rather than guessing.

At mouse down, a drag gesture captures the topmost hit among draggable sprites;
a non-draggable foreground sprite does not block that selection. The captured
sprite starts dragging after the pointer moves more than 3 viewport pixels or
the button stays pressed for 400 ms. A press with no draggable hit cannot capture
a sprite that appears later. Moving sprites, layer changes, and camera changes
do not select a different target. This is an intentional SPX difference from
Scratch's delayed drag picking.

Click targets are selected independently using the existing click rules; capturing
or cancelling a pending background drag does not change foreground click routing.

Hiding, destroying, or disabling dragging on the captured sprite cancels the
drag gesture, including before activation. The gesture cannot transfer to another
sprite or resume when visibility or dragging is restored; a new press is required.

Drag gestures detect pointer movement using the world pointer and camera/viewport
projection sampled in the same input frame. Every drag update, including release,
reprojects the sampled viewport point through the current camera. At activation,
the offset uses the sprite's current position and the reprojected press point.
Activation applies the pointer displacement since mouse down to that current
position, without pulling the sprite back after scene or camera updates. Replays
must reproduce the same scene, camera, and viewport evolution.

The stream does not contain browser mousemove events, so same-position events
and movement that returns within one tick cannot be reconstructed. The threshold
uses viewport pixels, which can differ from Scratch's CSS pixels on HiDPI or
CSS-scaled canvases. The v1 format is unchanged, but recordings made with delayed
target selection can produce different results in dynamic scenes under the new
mouse-down capture rule.

## EOF and end-of-frame completion

Reaching the final input item does not necessarily mean the visible result is ready. Replay completion is finalized at the appropriate frame boundary so game logic, engine synchronization, and rendering can consume the last tick before capture begins.

## Deterministic environment

Input alone is insufficient if a game depends on uncontrolled time, randomness, username, viewport, assets, network data, or platform behavior. The replay launch path supplies deterministic runtime environment values where supported. Tests should explicitly fix every external value that affects output.

## Go host interface

The Go runtime owns replay state and tick consumption. The Web host bridge starts/stops recording, loads replay data, and receives completion or error events. Keep format validation and gameplay-input substitution in the runtime rather than duplicating them in every host page.

## Capture integration

Configure `captureKey` to request screenshots on recorded key presses. Requests carry the latest consumed input tick and are dispatched after visual-state synchronization. The Web bridge waits for rendering before reading pixels. See [Web capture](web_capture.md).

## Not supported in v1

- arbitrary application or network state recording;
- cross-version compatibility without a format migration;
- nondeterministic external services;
- restoring a complete in-memory game snapshot;
- assuming identical results across different assets or engine builds.
