# Go → native Spine smoke

This self-authored 16×8 rectangle fixture checks real SPX startup, costume-free
Spine resource loading, atlas metadata, alias mapping, and `AnimateAndWait`
completion before requesting a successful engine exit. No third-party artwork
is included. The fixture uses a handwritten Go entrypoint so it does not need
the XGo compiler.

On Linux amd64, with Go 1.26, a C compiler, and a matching SPX-enabled Godot build:

```sh
./testdata/spine_native_smoke/run.sh /absolute/path/to/godot enabled
```

To additionally export with the real Linux template preset and run the PCK from
another directory, with external configuration and the platform library beside
it. The exported check deliberately removes raw atlas/skeleton/texture sidecars,
so the native loader must use the packed resources:

```sh
./testdata/spine_native_smoke/run.sh /absolute/path/to/godot enabled export
```

The fixture removes Godot's already-consumed `--main-pack` argument before SPX
script flag parsing, which does not currently recognize that engine option.
This is a fixture-only adaptation, not a runtime CLI change.

The engine must have been built with `spx_spine=yes`. To check the diagnostic
from an otherwise matching default/off build:

```sh
./testdata/spine_native_smoke/run.sh /absolute/path/to/godot disabled
```

The script creates a temporary Godot project, builds a C-shared Go extension,
imports resources before enabling the extension, runs headlessly with a 30-second
timeout, checks its result, and removes temporary
files. It leaves Go toolchain/cache selection to the caller. This exercises the
native bridge and animation events, not visual rendering or Web execution.

A renderer-disabled development engine may print `No renderers available` even
when this functional smoke passes. It does not establish pixel rendering parity.
