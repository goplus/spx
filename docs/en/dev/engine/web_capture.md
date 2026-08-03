# Web Capture and Fixed-Frame Integration

SPX Web exports provide a host-facing capture workflow for deterministic screenshots and visual regression tests. The runtime selects a logical frame; the browser host captures the rendered canvas only after that frame has completed.

## 1. SPX capability

Game code or host configuration can request capture at selected ticks. Runtime events identify session state, the target tick, completion, and errors. Capture scheduling belongs to the logical SPX frame pipeline rather than an arbitrary JavaScript timeout.

Future decorators may provide a shorter game-code syntax, but the host protocol remains the integration boundary.

## 2. Web bridge

The template Web host receives runtime messages from the Go/WASM and engine bridge. It converts a completed capture request into a canvas image, attaches metadata, and returns or stores the result according to the embedding page.

## 3. Request shape

A request should include a stable capture name and target tick/frame information. A host may add session, replay, baseline, or run identifiers. Treat unknown fields as host metadata; do not make game determinism depend on storage-specific values.

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
