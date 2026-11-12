# Scratch-style requests

Translate the behavior, then verify the SPX spelling at the target version.
Normal `.spx` authoring and exact Scratch VM emulation are different goals.
Use ordinary typed XGo code unless the task needs Scratch coercion or indexing.

## Gameplay differences to check

- `setSize 1` means original size, not 1 percent. `size()` returns that scale
  factor and `changeSize` adds to it. Convert Scratch percentages by dividing
  by 100; do not invent a script `Scale` API. Runtime size limits still apply.
- `setDragMode Draggable` and `setDragMode NotDraggable` control dragging.
- Color sensing uses `touchingColor(targetColor)` or
  `touchingColor(spriteColor, targetColor)`. Check color construction in
  `color.go`; `HSB` and `HSBA` components use a 0–100 range.
- Conditions are supported, for example `onCond score >= 3, => { ... }`.
  Read `runtime_events.go` before relying on edge and reentry behavior.
- `Warp` exists, but does not promise unlimited atomic execution. The runtime
  can yield at its execution budget. Check `runtime_control.go` and a verified
  call form before translating “run without screen refresh”.

For the above, inspect `sprite_transform.go`, `component_transform.go`,
`sprite_drag.go`, `sprite_query.go`, and their tests. Do not infer units from
percentage-looking literals or comments in old examples.

## Compatibility helpers

When the requested behavior requires compatibility helpers, import
`github.com/goplus/spx/v4/pkg/scratchcompat` explicitly. Read its
`operators.go` and `list.go`, plus `pkg/ispx/scratchcompat_test.go`, before
choosing a helper. The package is not an exact Scratch VM emulator.

- `scratchcompat.CharAt` is 1-based and counts Unicode code points, not UTF-16
  code units. Deprecated `spx.CharAt` is 0-based. Native XGo string indexing
  indexes bytes, so these are not interchangeable for non-ASCII text.
- `scratchcompat.List` is still 0-based. `At` returns `spx.Value`; `IndexOf`
  returns -1 for a missing item. Translate Scratch positions at the boundary,
  including special positions, rather than subtracting 1 indiscriminately.
- Compatibility comparisons include numeric coercion and case-insensitive
  string comparisons. `Equal` allows a small numeric tolerance. Native XGo
  operators are not drop-in replacements for these rules.
- `scratchcompat.Rand` needs the original operands: `"1"` and `"1.0"` select
  different integer/continuous behavior. Do not eagerly convert them.
- `Div` preserves infinities and maps a NaN result to zero. Confirm edge cases
  in `internal/core/scratch/` tests when they matter to the project.

For unsupported or uncertain blocks, explain the gap and choose a small,
testable adaptation. Do not claim complete Scratch compatibility from a
similar API name.
