# Drag mode

Drag the blue tile with the left mouse button. Press **Space** to switch between
`setDragMode Draggable` and `setDragMode NotDraggable`. The gray tile keeps
the default non-draggable mode. A blue-tile click prints a count; dragging does
not count as a click.

From the repository root, run with the installed matching native runtime:

```sh
spx runnative --path ./test/DragMode
```

A press captures the topmost draggable sprite, including behind non-draggable
sprites. Dragging starts after moving more than 3 viewport pixels or holding for
400 ms. The target stays fixed for that press; pressing empty space cannot capture
a sprite that appears later. This intentionally differs from Scratch's delayed
target selection.

Hiding, destroying, or disabling dragging on the target cancels the gesture.
Restoring the target does not resume it; release and press again.

Small movement still counts as a click on release. Clicks use the topmost visible
hit target. Dragging with the pen down draws nothing, and the next scripted
movement resumes from the drop position without an extra pen-down dot.

Recordings require the same scene, camera, and viewport during replay. See
[input replay](../../docs/en/dev/engine/input_replay.md#version-1-data) for camera
and HiDPI details. Browser/GPU behavior needs validation with a matching runtime.
