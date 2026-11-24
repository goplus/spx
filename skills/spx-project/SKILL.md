---
name: spx-project
description: Create and edit XGo .spx game projects, configure sprites and assets, and translate Scratch gameplay into SPX.
---

# Author an SPX game

Use the game's declared SPX version. In a v4 project, run
`go list -m -json github.com/goplus/spx/v4` from the game directory to find
the selected module and any replacement. Check unfamiliar calls against a
`.spx` example from that version.

## Organize stage and sprite scripts

`main.spx` defines the stage. Put shared fields and helper functions there
so sprites can use them directly. A sprite file such as `Bullet.spx`
defines fields and methods for each instance of that sprite. In these
classfiles, omit `package main` and `func main()`. The compiler supplies
the entry point and named sprite references such as `Bullet`.

1. Group the file's fields in one `var (...)` declaration. Typed fields
   and fields with initial values can share that block.
2. Define helper functions after the fields. A `func` declared here becomes
   a method of the stage or sprite.
3. Register event handlers at file scope. Top-level statements execute in
   order, so a wait or loop above a handler delays its registration. Put
   startup work that waits or loops inside `onStart => { ... }`.

## Coordinate game objects

- Call a named sprite directly for a targeted action, as in `Bullet.clone`.
  Initialize the clone in `onCloned`, where `this` refers to the new
  instance. A clone copies its parent's fields. Allocate a fresh map or
  slice when it needs independent storage.
- Use `broadcast` to notify listeners, or `broadcastAndWait` when the
  sender's next action depends on their handlers finishing. A named
  listener takes no arguments, as in `onMsg "hit", => { hide }`. To read
  a payload from `broadcast "loot", 5`, use `onMsg (msg, data) => { ... }`
  and check `msg` inside the handler.
- Use `onCond` when a condition becomes true. It skips further checks
  while its handler is running.

## References

- To look up a stage or sprite API, its overloads, callback arguments, or
  return values, use the [SPX v4 script API reference](references/apis.md).
- For command calls, overloads, literals, loops, and event callback syntax,
  see [XGo in SPX scripts](references/xgo.md).
- For movement, input, animation, sound, physics, or Builder AI, see
  [common gameplay calls](references/gameplay.md).
- For scaffolding, asset indexes, and native or Web checks, follow the
  [project guide](references/project.md). The [Dragon example](references/project.md#steer-and-score)
  combines keyboard input, contact events, and a score monitor.
- For Scratch conversions involving units, sensing, coercion, or indexing,
  use the [Scratch guide](references/scratch.md).
