# Web Capture Integration

SPX Web exports provide a host-facing capture workflow for screenshots and visual regression tests. The runtime dispatches screenshot requests after the current frame's visual-state synchronization; the browser host waits for a render fence before reading the canvas.

Selecting the same frame or input tick does not guarantee identical script state. The scheduler uses a wall-clock budget for loop rounds, which input recording and replay do not fix or record. If visible state depends on the number of iterations completed within that budget, screenshots can vary between runs. See [SPX script scheduling](scheduling.md#capture-and-input-replay).

## 1. SPX capability

Game scripts call `spx.Snapshot(name, fn)` at the step they want to capture. It runs `fn` if non-nil, then queues a screenshot if the body succeeds. A body that yields can finish in a later frame; the request records the frame and latest input tick when the body returns. Without an active game, the request is dispatched immediately.

Recording and replay can also use a configured `captureKey` (the Web runner defaults to `P`). Its press edges are part of the recorded input, so replay requests screenshots at the same input ticks without script-side scheduling. The internal engine frame counter is retained as screenshot metadata and includes loading and bootstrap updates.

## 2. Web bridge

The template Web host receives runtime messages from the Go/WASM and engine bridge. It converts a completed capture request into a canvas image, attaches metadata, and returns or stores the result according to the embedding page.

## 3. Request shape

A request includes a capture name, sequence, engine frame, and the latest consumed input tick when available. The host uses `inputTick` to pair recording and replay images; it uses `frame` to share one rendered image between requests from the same engine frame. A host may add session, replay, baseline, or run identifiers. Treat unknown fields as host metadata; do not make game determinism depend on storage-specific values.

## 4. Minimal integration

1. Use the exported SPX canvas and runtime host bridge.
2. Listen for capture requests and runtime completion events.
3. Wait until the requested frame is rendered.
4. read the canvas with `toBlob`, `toDataURL`, or an equivalent API.
5. Report success or failure to the caller.

Handle a tainted canvas as an explicit error. Cross-origin assets must permit capture.

## 5. Baselines and runs

The template lab separates approved baseline images from new run images. Store metadata alongside each image: build/version, Web mode, viewport, device scale, capture name, replay name, and logical tick. Compare only results produced under equivalent conditions.

## 6. External page workflow

- embed or load the exported runner;
- wait for runtime readiness;
- launch a clean game session;
- optionally load an input replay;
- request named captures;
- collect completion and image results;
- terminate or reset the session before the next run.

## 7. Template lab features

The repository template adds a UI for sessions, baseline/run organization, replay selection, capture status, and image inspection. These are conveniences, not requirements of the low-level host API.

## 8. Runner events

Coordinate capture with runner readiness, launch, game end, replay completion, and errors. Do not infer readiness from DOM creation alone. A game-end event and the last rendered frame may occur at different points in the browser event loop.

## 9. Notes

- Fix viewport size and device pixel ratio for regression images.
- Keep Web mode and asset build identical.
- Avoid wall-clock delays as capture synchronization.
- Surface missing canvas, context loss, and encoding failures.
- Release object URLs and large image buffers after use.

## 10. Reference files

The implementation is under `cmd/spx/template/platform/web`, especially the runner, capture module, lab page, and replay integration. Those files define the current event names and payload schema.
