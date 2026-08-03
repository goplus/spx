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
6. Capture at selected ticks or at replay completion.
7. Compare output with the baseline.

Do not replay into an already-running game because prior state is not part of the input recording.

## Input ticks

Replay uses the SPX input tick, not browser event timestamps or render frames. All input consumed by one logical update belongs to the same tick. This keeps the stream stable when browser scheduling or rendering frequency changes.

## Version 1 data

The v1 stream covers the SPX input snapshot used by game logic, including supported pointer, keyboard, and action state. The exact schema is versioned; readers must reject incompatible future formats rather than guessing.

## EOF and end-of-frame completion

Reaching the final input item does not necessarily mean the visible result is ready. Replay completion is finalized at the appropriate frame boundary so game logic, engine synchronization, and rendering can consume the last tick before capture begins.

## Deterministic environment

Input alone is insufficient if a game depends on uncontrolled time, randomness, username, viewport, assets, network data, or platform behavior. The replay launch path supplies deterministic runtime environment values where supported. Tests should explicitly fix every external value that affects output.

## Go host interface

The Go runtime owns replay state and tick consumption. The Web host bridge starts/stops recording, loads replay data, and receives completion or error events. Keep format validation and gameplay-input substitution in the runtime rather than duplicating them in every host page.

## Capture integration

Replay is designed to work with the fixed-frame capture API. Select capture ticks relative to the replay session and wait for the runtime's frame-complete signal before reading pixels. See [Web capture](web_capture.md).

## Not supported in v1

- arbitrary application or network state recording;
- cross-version compatibility without a format migration;
- nondeterministic external services;
- restoring a complete in-memory game snapshot;
- assuming identical results across different assets or engine builds.
