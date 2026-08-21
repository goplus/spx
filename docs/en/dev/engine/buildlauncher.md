# `spx buildlauncher`

`spx buildlauncher` creates a self-contained host launcher without starting
Godot and without building or running an XGo project driver.

```sh
spx buildlauncher --path ./game
spx buildlauncher --path ./game -o ./bin/game
```

The default output is `./game/.builds/game` (with `.exe` on Windows). A
relative `-o` path is resolved from the current directory. The command builds
in a private staging directory beside the requested output and atomically
replaces the old file only after validation. The output cannot overwrite SPX
project sources, project configuration, module metadata, selected bridge
sources, or the pack directory.

The target module must contain `gox.mod` or `gop.mod` with a `.spx` project and
its `pack` declaration. Every top-level `.spx` file and every file under the
declared pack directory is included in the launcher. The bridge is built from
the SPX module selected by the active Go graph, whether it comes from the
module cache, the main module, or a local replacement. No local XGo driver is
required.
The command freezes `GOWORK` and `GOFLAGS` for the invocation and verifies the
module selection and graph metadata before and after both Go builds. The built
bridge and payload are content-digested.

An explicit `SPX_RUNTIME_LOCAL_MANIFEST` or `SPX_RUNTIME_ASSET_DIR` takes
priority and fails closed. Otherwise, the command uses the verified release
cache and download first. When the selected SPX module is a source checkout,
an unpublished runtime or failed acquisition falls back to the exact-version
Engine and PCK in the first `GOPATH/bin`. Both files must be regular,
non-symlink files and are hashed again before packaging. If they are missing,
run `make dev` in the SPX checkout; `buildlauncher` never starts a Godot build.
Set `SPX_RUNTIME_OFFLINE=1` to skip network access and use cache/local fallback.
