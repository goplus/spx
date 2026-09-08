# List Monitor

Scratch player list monitors use the existing `monitor` stage shape with
`"mode": "list"` (or the numeric SPX mode `4`). Bind `val` to a `List`, a slice,
an array, or a getter returning one of these; both `items` and `getVar:items`
are accepted. Set `target` to a sprite name for a sprite-local list.

The monitor shows one-based row numbers, literal item text, `(empty)`, and the
list length. Long items are clipped with an ellipsis. The wheel, trackpad, touch
drag, and scrollbar scroll the list. Only visible rows are drawn, and updates
preserve the scroll position, clamping it when the list shrinks. Like Scratch's
player, the monitor does not edit the underlying list.

`width` and `height` default to 100 × 200 when absent or zero, with minimums of
100 × 60. `size` is the existing monitor scale (default 1); `x` and `y` retain
SPX's centered stage coordinates. The default item color is Scratch list orange
(`#ff661a`); `color` can override it. Use `showVar` / `hideVar` or the existing
widget `show` / `hide` methods to control visibility.

After building the matching engine and Go bindings, run from the repository root:

```sh
go run ./cmd/spx rune --path ./test/StageListMonitor
```

The example includes 10,007 items and an empty list. Scroll to the middle, then
press **A** to append, **R** to replace the first item, **D** to delete the first
item, **C** to clear, and **H** to hide/show the populated monitor. Clearing while
scrolled should display `(empty)` and `length 0`, with no stale rows.

Reference behavior comes from the local Scratch GUI's `list-monitor.jsx`,
`list-monitor-scroller.jsx`, and `containers/list-monitor.jsx`.
