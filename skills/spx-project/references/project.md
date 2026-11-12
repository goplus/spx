# Project model and examples

## Files and lifecycle

`main.spx` is the game script. Other `.spx` files define sprites. A typical
project has `assets/index.json` and
`assets/sprites/<SpriteName>/index.json`, plus the images and sounds referenced
by those configs. Preserve the existing resource root and filename casing.

The initial file `var` block declares fields. Top-level functions become
methods, and top-level statements form the file's entry body. Use normal XGo
expressions, control flow, and `=>` lambdas. A helper named `init` or `main` is
not an ordinary Go package initialization hook in a classfile.

Use `onStart` for startup behavior, `onCloned` for clone behavior, and messages
or input events for reactions. `func OnLoaded()` in `main.spx` is a post-load
setup hook; it is not the place for an ongoing gameplay loop. Keep condition
predicates quick and nonblocking. Current `onCond` handlers fire on a rising
edge, not every frame the condition stays true.

The compiler exposes named sprite references to the game. Sprite scripts can
access shared game fields and other sprites without manually declaring those
references. A clone has its own sprite fields, but slices, maps, and pointers
need care: copying a field does not imply deep-copying its contents.

## Working examples

Use these small, complete projects from the matching SPX checkout:

- [`tutorial/01-Weather`](https://github.com/goplus/spx/tree/dev/tutorial/01-Weather):
  `onStart`, `onMsg`, and broadcasts. Run it and check that Kai and Jaime take
  turns speaking.
- [`tutorial/02-Dragon`](https://github.com/goplus/spx/tree/dev/tutorial/02-Dragon):
  sprite fields, collision polling, sounds, and a score monitor. Check that
  collisions with Shark increase Dragon's displayed score.
- [`tutorial/03-Clone`](https://github.com/goplus/spx/tree/dev/tutorial/03-Clone):
  shared `gid`, per-clone `id`, `onCloned`, and `broadcastAndWait`. Click Calf
  to create clones, then Arrow to undo the most recent one.

These links navigate `dev`; use the corresponding files at the project's
resolved version when behavior differs. Copy a complete project, not just its
scripts. Tutorials inside the repository inherit its module context; a copy
outside it needs its own module setup for the intended SPX version.

For a repository checkout, follow its
[README](https://github.com/goplus/spx/blob/dev/README.md) to install the runtime
with `make setup` and check it with `make doctor`. Toolchain versions are in
`internal/release/runtime.lock.json`; `go.mod` and `gox.mod` describe the module
and classfile setup.

For a standalone game, use the matching installed CLI to create a new, empty
destination:

```sh
spx init --path ./my-game
```

Keep the generated `go.mod`, give the game its own module name, and check that
its SPX requirement matches the intended version. The `//xgo:class` marker on
that requirement enables the dependency's classfile configuration; do not
copy the engine's module identity into the game. To base it on a tutorial,
replace the scaffold scripts and assets with the complete tutorial contents.
For local development against a checkout, add a Go module `replace` for
`github.com/goplus/spx/v4` pointing to that checkout. Do not run `spx init` over
an existing game: it writes starter scripts and assets.

With the matching toolchain and runtime installed, run a repository tutorial:

```sh
cd tutorial/03-Clone
xgo run .
```

The repository's `TestProjectTutorials` compiles all three examples through
`pkg/ispx` without starting Godot. It does not validate graphics, audio, or
interactive behavior. For edited games, run their own build and exercise the
changed input, event, clone, and asset paths in the target runtime.
