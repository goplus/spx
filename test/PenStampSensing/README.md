# Pen stamping and color sensing demo

Build the local editor and runtime from the current branch first. Previously
installed engine binaries do not include these changes:

```sh
cd /Users/mac/qiniu/spx
GODOT_SRC=/Users/mac/qiniu/godot make build-editor
GODOT_SRC=/Users/mac/qiniu/godot make build-desktop
```

Then run the demo using the same entry point as `test/CoordinateSystem`:

```sh
cd /Users/mac/qiniu/spx/test/PenStampSensing
spx runnative
```

`make runnative DEMO_INDEX=...` only lists projects under `tutorial/`. Since this
demo is under `test/`, run `spx` directly from its directory.

The demo runs 17 checks at startup. Each check prints `PEN_DEMO_PASS` or
`PEN_DEMO_FAIL`. A successful run ends with:

```text
PEN_DEMO_DONE checks=17 failures=0
```

From left to right, the stage shows a cyan stamp, a half-transparent cyan stamp,
and the live source sprite restored to red and facing left. Red and blue pen
lines appear below them. The small black square is the color probe. Existing
stamps retain their appearance when the source sprite's effects, position, and
direction change.

- **Arrow keys**: Turn the live sprite toward the pressed direction, move 40
  steps in that direction, and leave a stamp at the new position. Arrow input is
  ignored while the automatic checks are running.
- **R**: Run all checks again.
- **E**: Erase stamps and lines.
- **Space**: Redraw the demonstration scene.

The checks cover stamping before creating a pen, color and ghost effects,
preserving stamps after source changes, sensing new lines immediately with
`TouchingColor`, both color-sensing overloads, repeated submissions, persistence
across frames, clearing rendered and pending drawings, and drawing immediately
after a clear.

After stamping, the demo waits one frame for the source sprite's new position
to reach the renderer so it no longer obscures the second stamp. Checks for new
lines and clearing query immediately without waiting for a rendered frame.
Separate checks verify persistence across frames.

Automated test mode exits after the checks, returning 0 on success or 1 on
failure:

```sh
SPX_PEN_DEMO_TEST=1 spx runnative
```

Run with a native graphics context. The dummy renderer used by `--headless`
cannot validate GPU stamping or pixel readback. All artwork is provided as SVG
files in this directory; no external images are required.
