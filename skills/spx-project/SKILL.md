---
name: spx-project
description: Write or modify user-facing spx projects with `.spx` files, events, shared state, sprite behavior, and asset configs.
---

# spx project

A `.spx` file is ordinary XGo source code. Apply XGo syntax rules first. Then apply the spx-specific file roles,
classfile behavior, project structure, event forms, and builtin-like APIs documented here.
You can think of spx project authoring as a Scratch-style DSL built on XGo classfiles. Scratch concepts often
transfer, but the output must still use current spx spellings and normal XGo syntax.

## Use this skill when

- creating or modifying `main.spx`
- creating or modifying sprite `.spx` files
- wiring stage events, sprite events, clone behavior, and shared state
- using spx builtin-like APIs and their overloads
- editing `assets/index.json` or sprite or sound config files for a spx project
- translating Scratch-style behavior descriptions into current spx project code

## Core model

- `main.spx` is the stage or game script
- every other `.spx` file is a sprite script
- the file state is the field declaration block, which is the first top-level `var` declaration whose preceding
  top-level declarations, if any, are only imports, `const`, or `type`
- top-level `func` declarations without explicit receivers become methods of the current file object
- top-level statements become the file entry body
- the project starts automatically

## Reading order

Read only what the task needs:

1. Read [references/scratch-compat.md](references/scratch-compat.md) first when the request is phrased in Scratch
   blocks or other Scratch vocabulary.
2. Read [references/project-layout.md](references/project-layout.md) first when the main question is "which file should
   own this behavior" or "where should this state live".
3. Read [references/xgo-syntax.md](references/xgo-syntax.md) when syntax is the uncertainty, especially lambdas,
   command-style calls, top-level statements, or the field declaration block rule.
4. Read [references/spx-api.md](references/spx-api.md) before writing any event or API spelling that you are not
   already sure about.

## What to do first

- inspect the existing project layout and keep `assets/` or `res/` consistent with the repo
- identify which file owns the requested behavior before writing code
- preserve the current project's naming and calling style unless the task explicitly asks for cleanup
- check whether the request changes `.spx` code only or also requires asset config changes
- translate Scratch requests into current spx authoring spellings before writing code
- read the matching reference before guessing syntax, event signatures, overloads, or uncommon APIs

## Default coding workflow

1. Decide which file owns the requested behavior.
2. Decide whether the needed state belongs in `main.spx` or a sprite file.
3. Decide whether the behavior starts from top-level entry code, `onStart`, `OnLoaded`, `onCloned`, or another event.
4. Add or update helper functions before event handlers when the logic is reused.
5. Register events at top level.
6. Update asset config files only if the task truly needs config changes.
7. Re-read the edited `.spx` file and confirm the state `var` block is still in the correct place.

## Ownership rules

- global score, level, game flags, scene-wide reloads, and stage-wide messages usually belong in `main.spx`
- player movement, aiming, firing cadence, local cooldowns, and local hp usually belong in the player sprite file
- bullet behavior usually belongs in the bullet sprite file, especially when bullets are clones
- enemy movement, local hp, hit reactions, and death behavior usually belong in the enemy sprite file
- HUD monitor visibility and monitor config usually involve `assets/index.json`
- camera follow often belongs in project config, and camera movement logic often belongs in `main.spx`
- pathfinding, physics bodies, collider setup, and trigger setup usually belong in the sprite that owns that body

## Event selection rules

- use top-level statements for small file-local setup only
- use `onStart` for loops and normal lifecycle startup work
- use `func OnLoaded()` in `main.spx` for post-load setup that should happen after sprites are initialized
- use `onCloned` for clone-only initialization and clone-local loops
- use `onMsg` when the sender should not depend on a concrete receiver
- use direct sprite access when one known sprite should be called immediately
- use `onTouchStart` for collision start reactions
- use `touching(...)` inside a loop when you need continuous polling rather than a one-time touch start

## Non-negotiable rules

- use normal XGo syntax for expressions, statements, imports, control flow, composite literals, slices, maps, and
  lambdas
- use XGo lambdas with `=>` normally in `.spx` files
- keep shared state in `main.spx`
- keep per-sprite state in that sprite file
- place the file state `var` block before all top-level helper `func` declarations
- initialize clone-only state in `onCloned`
- do not manually declare sprite references in `main.spx` just because `Player.spx` or `Enemy.spx` exists
- use `onCloned` and `onTouchStart` only in sprite files
- use direct sprite names when the project already uses that style
- treat `func init()` and `func main()` as ordinary classfile methods, not package hooks
- do not guess user-facing spellings or overloads
- do not move the file state `var` block below helper functions or event registrations
- do not put a top-level `func` before the file state `var` block unless you intentionally want later `var`
  declarations to stop being file fields
- do not rewrite normal project code into unusual alternate spellings
- do not add imports for ordinary gameplay APIs that are already available directly in `.spx` project code

## When to stop guessing and read the references

Open the references instead of guessing when the task involves:

- Scratch block wording or other Scratch-style phrasing
- exact event callback signatures
- `onKey`, `onMsg`, `onBackdrop`, `onCloned`, or `onTouchStart` overload choice
- clone setup patterns
- camera, pathfinding, tilemap, query, or physics APIs
- monitor JSON or asset config edits
- uncommon helpers such as `SetDebug`, `GetWidget`, `List`, or `Value`

## References

Read [references/scratch-compat.md](references/scratch-compat.md) when you need:

- a stable Scratch to spx mental-model mapping
- current project-side spellings for common Scratch blocks and reporters
- translation caveats such as 1-based Scratch positions versus 0-based XGo indexing
- a list of unsupported or not-one-to-one Scratch areas that should not be guessed

Read [references/xgo-syntax.md](references/xgo-syntax.md) when you need:

- XGo syntax rules inside `.spx` files
- lambda forms with `=>`
- command-style call syntax
- top-level file rules and classfile-oriented source structure

Read [references/project-layout.md](references/project-layout.md) when you need:

- project layout and asset directory conventions
- `main.spx` vs sprite file responsibilities
- state placement, startup timing, cross-sprite access, and clone patterns
- resource config patterns and stage monitor JSON

Read [references/spx-api.md](references/spx-api.md) when you need:

- precise event signatures and callback forms
- exact overloads for stage or sprite APIs
- event selection by intent
- implementation templates for common gameplay tasks
- helpers, constants, and uncommon project-side APIs

## Output standard

- write direct `.spx` project code, not abstract pseudocode
- keep examples and explanations focused on project authorship
- use normal project spellings such as `onStart`, `broadcastAndWait`, `setXYpos`, and `animateAndWait`
- if the current project already uses mixed casing such as `Camera.SetZoom`, keep that local style consistent
- use reference files instead of guessing behavior
- when a task is ambiguous, choose the most normal project-side spelling and structure rather than inventing a lower
  level workaround

## Do and do not

Do:

- write `.spx` code like normal spx project code
- follow XGo syntax rules first, then apply spx-specific file and API rules
- keep `main.spx` focused on shared state and stage logic
- keep sprite behavior inside sprite files
- use top-level event registrations
- use `onCloned` for clone setup
- use the reference files instead of guessing overloads

Do not:

- rewrite normal project guidance into unusual alternate spellings
- write explicit receiver boilerplate for normal `.spx` methods
- move the state `var` block below top-level event code
- add framework imports for ordinary gameplay code when direct project forms already cover the need
- prefer alternate lower-level spellings when a direct `.spx` form already exists
