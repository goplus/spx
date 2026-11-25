# Translating Scratch blocks

## Gameplay

- **Size.** SPX uses scale factors. “Set size to 50%” becomes `setSize 0.5`,
  and “change size by 10” becomes `changeSize 0.1`.
- **Touch.** By default, `touching("Shark")` and `onTouchStart` sense overlapping
  costume pixels even with physics enabled. For a larger detection zone,
  enable `touchByShape` in the stage config and give both sprites trigger shapes.
  Body colliders control physical response.
- **Color.** Use `touchingColor(targetColor)` for “touching color?” and
  `touchingColor(spriteColor, targetColor)` for “color is touching color?”.

## Values and selectors

Import `github.com/goplus/spx/v4/pkg/scratchcompat` for blocks that depend
on Scratch coercion or indexing.

- `scratchcompat.CharAt("A😀B", 2)` returns `"😀"`. It counts Unicode code
  points from 1. `scratchcompat.Compare` compares numbers when possible,
  then falls back to lowercase UTF-16 string ordering.
- `List.At` uses zero-based positions. `IndexOf` returns -1 for a missing
  item. Convert numeric Scratch positions separately from selectors like
  `Last` and `Random`.
- Preserve operand types for `scratchcompat.Rand`. A bound of `"1"` with
  integer upper bound `6` selects an integer. `"1.0"` selects a continuous value.
- In a sprite script, use `setCostume scratchcompat.ResolveCostumeIndex(this, selector)`
  for a computed costume selector. Exact names take priority over one-based
  integer strings.
- In `main.spx`, use `setBackdrop scratchcompat.ResolveBackdropIndex(this, selector)`
  for a computed backdrop selector. In a sprite script, pass `this.Game` as
  the first argument so names and indices refer to the stage's backdrops.
  Exact names take priority over actions and one-based numeric selectors.
