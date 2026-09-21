# Pen GPU regression checks

Godot's ordinary `--test` SceneTree runner uses a mock display and dummy
renderer. This small, explicitly enabled test module also runs pen operations
in a normal SceneTree and checks actual OpenGL readback pixels on macOS.
It covers material snapshots, mixed stamp/line ordering, actual SPX effects,
alpha blending, repeated flushes, clear, resize, transforms, cached readback,
skipping readback for non-overlapping queries, and display of the premultiplied
pen layer. The interactive SPX demo is in `test/PenStampSensing`.

With the project's SCons environment installed, run from the repository root:

```sh
GODOT_SRC=/path/to/godot python3 godot_modules/spx/tests/gpu/run.py
```

The script reads the current SPX SCons profile, uses the shared build lock,
and creates a separate `pen_validation` editor binary. The fixture is excluded
from normal builds. To reuse that test binary, pass `--skip-build`; an existing
fixture binary at another path can be selected with `--binary`.

Logs and PNG readbacks are saved to `.tmp/pen-validation/`. The OpenGL run
requires access to the macOS graphics session; a headless renderer cannot
replace this check.
