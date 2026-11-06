# Drag mode

Drag the blue tile with the left mouse button. Press **Space** to switch between
`setDragMode Draggable` and `setDragMode NotDraggable`. The gray tile keeps
the default non-draggable mode. A blue-tile click prints a count; dragging does
not count as a click.

From the repository root, run with the installed matching native runtime:

```sh
spx runnative --path ./test/DragMode
```

A draggable sprite starts moving after the pointer travels more than 3 viewport
pixels or stays pressed for 400 ms. Small movement still counts as a click on
release. Clicks use the topmost visible hit target. Dragging picks among draggable
sprites, so a non-draggable foreground sprite does not block dragging through it. Dragging with the
pen down draws nothing, and the next scripted movement resumes from the drop
position without an extra pen-down dot.

Input recording/replay v1 keeps mouse positions in world coordinates. Reproduce
the same scene, camera, and viewport when comparing a recording with its replay;
the format does not record the display transform. The threshold uses viewport
pixels, not browser CSS pixels, so on HiDPI or CSS-scaled canvases its physical
movement tolerance can differ from Scratch's 3 CSS pixels. Browser/GPU behavior
needs validation with a matching rebuilt runtime.
