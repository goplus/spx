---
name: spx-project
description: Create or modify SPX game projects using .spx scripts, events, sprites, clones, and asset configuration. Use for Scratch-style gameplay requests, not SPX engine development.
---

# SPX project authoring

Write runnable project code in XGo, using the project's existing conventions.
This guide targets SPX v4; an installed skill is not evidence of the game's
runtime version.

## Establish the target

1. Inspect `go.mod`, any `gox.mod`, `.spx` files, and the resource directory.
   Resolve the actual SPX dependency, including `replace` directives. Do not
   upgrade an existing project just to match this skill.
2. Use that version's source and examples to confirm unfamiliar APIs. The
   current module is `github.com/goplus/spx/v4`; ordinary gameplay APIs are
   available directly in `.spx` files without importing SPX.
3. Identify the owner of the change: shared game state in `main.spx`, per-instance
   state and behavior in the sprite file, or resource configuration in
   `assets/`. Preserve `res/` when the project already uses it.

For a new project, start from a small matching tutorial with its assets and
module context. Read [project guidance](references/project.md) for the file
model, runnable examples, and validation. Read
[Scratch compatibility](references/scratch.md) when translating block semantics
or preserving behavior from a Scratch project.

## Authoring rules

- Put file fields in an initial `var (...)` block before helper functions and
  executable statements. Use `main.spx` fields for shared state and sprite
  fields for each instance's state.
- Register events at top level with XGo lambdas, for example
  `onStart => { ... }` and `onMsg "hit", => { ... }`. Keep ongoing gameplay in
  event handlers rather than blocking file initialization.
- Use `onCloned` for clone-only initialization. Clones copy sprite fields;
  reset any state that must differ from the original.
- Do not add manual sprite reference declarations or a `run` call to a normal
  project. Follow the project loader and existing file naming.
- Prefer local `.spx` spellings such as `setXYpos`, `broadcastAndWait`, and
  `waitNextFrame`. Go declarations may use exported names and `__N` overload
  suffixes; those suffixes are not the project API.
- For one known target, direct sprite access is usually enough. Use broadcasts
  for multiple listeners; `broadcastAndWait` waits for their handlers.
- Confirm resource names, paths, and stage `zorder` entries when adding sprites,
  costumes, sounds, or monitors. Code alone does not supply missing assets.

## Look up only what is needed

Use the target version's checkout or Go module source, not a copied API list.
These paths are relative to the SPX repository:

- File roles and loading: `gox.mod`, `internal/spxproject/`, `game_load.go`,
  `game_apply.go`.
- Events: `game_events.go`, `sprite_events.go`, `runtime_events.go`.
- Movement, size, appearance, sensing, and dragging: `sprite_transform.go`,
  `sprite_render.go`, `sprite_query.go`, `sprite_drag.go`.
- Clones and execution: `sprite_clone.go`, `runtime_control.go`.
- Asset schemas: `internal/core/project/config.go`; concrete configs under
  `tutorial/`.
- Camera and specialized features: `camera.go`, related source/tests, and
  matching projects under `test/`.

Search the declaration and an existing `.spx` call site together. Confirm
overloads and semantics in tests; examples can contain stale comments. If the
matching source is unavailable, state the uncertainty rather than inventing
an API or silently using a different version.

## Finish

Check syntax and types through the project's build or SPX interpreter checks.
Run local gameplay with `spx runnative` and Web gameplay with `spx runweb`,
using the `--path` examples in [project guidance](references/project.md).
Distinguish compilation from asset loading and visual behavior. Report changed
files, checks performed, and any runtime checks that were unavailable.
