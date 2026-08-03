# SPX coordinate-system regression demo

This demo exercises the coordinate ownership introduced by:

- goplus/spx#1702
- goplus/godot#327
- goplus/spx#1703

Expected behavior:

1. The stage center is SPX `(0, 0)`, `+X` points right, and `+Y` points up.
2. The black cross is the sprite's logical root. Pressing `Space` cycles the
   same SVG through three costume centers. The selected colored anchor must
   stay on the black cross while the rendered SVG moves around it.
   The SVG source dimensions are declared explicitly as `120 × 80`, so each
   center is interpreted in the same source-image pixel coordinate space.
3. `R` rotates around the logical root. `F` flips left/right around the same
   root. `N` restores normal rotation.
4. `P` draws a cyan pen segment in SPX `+Y`, so the segment must go upward.
5. `S` stamps the current rendered costume and then moves the live sprite to
   the right. The stamp must remain exactly where the rendered costume was.
6. Arrow keys move both the logical root and its rendering together. `C`
   resets the demo and `E` clears pen drawings and stamps.

Run with the native SPX runtime:

```sh
cd test/CoordinateSystem
spx runnative
```
