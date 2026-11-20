---
name: spx-project
description: Create and edit XGo .spx game projects, configure sprites and assets, and translate Scratch gameplay into SPX.
---

# Author an SPX game

Use the game's declared SPX version. In a v4 project, run
`go list -m -json github.com/goplus/spx/v4` from the game directory to find
the selected module and any replacement. Check unfamiliar calls against a
`.spx` example from that version.

## Write the scripts

- Put shared fields in `main.spx` and per-instance fields in the sprite's
  `.spx` file. Declare fields in an initial `var (...)` block. The compiler
  supplies named sprite references and the game entry point.
- Register handlers at file scope, for example `onMsg "hit", => { hide }`.
  Use `onStart` for startup logic and `onCond` when a condition becomes true.
  `onCond` skips further checks while its handler is running.
- Use `onCloned` to initialize each clone. A clone copies its parent's fields,
  so allocate a fresh map or slice when the clone needs independent storage.
- Call a known sprite directly. Use `broadcast` to notify listeners and
  `broadcastAndWait` when the next step depends on their handlers finishing.
- Use XGo spellings such as `setXYpos` and `waitNextFrame`. Omit the `__N`
  overload suffix when translating an exported Go method into a script call.

For scaffolding, asset indexes, and native or Web checks, follow the
[project guide](references/project.md). Work through keyboard input, contact
events, and a score monitor in the [Dragon example](references/project.md#steer-and-score).
For Scratch conversions that depend on units, sensing, coercion, or indexing, use the
[Scratch guide](references/scratch.md).
