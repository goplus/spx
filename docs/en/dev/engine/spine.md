# Spine animation

A sprite can render a Spine skeleton instead of frame costumes. Set `spineAnim`
in its `assets/sprites/<name>/index.json`:

```json
{
  "size": 1,
  "visible": true,
  "heading": 90,
  "spineAnim": {
    "atlas": "hero.atlas",
    "skeleton": "hero.skel",
    "defaultMix": 0.1,
    "stepDuration": 0.01,
    "turnToDuration": 1,
    "animMap": { "idle": "idle", "walk": "run" }
  },
  "defaultAnimation": "idle",
  "animBindings": { "step": "walk" }
}
```

Atlas and skeleton paths resolve relative to the sprite configuration, following
the same path rules as other sprite assets. A skeleton may use Spine binary
(`.skel`) or JSON format. Texture page paths in the atlas resolve relative to the
atlas file and must stay within its directory or subdirectories. Absolute paths,
URL schemes, parent (`..`) segments, and symlinks are rejected. Package the
atlas, skeleton, and every texture page together.
The project packer retains local resources and follows referenced external
atlas pages; native launch bundles collect typed Spine references, including
resources elsewhere within the project.

`animMap` maps SPX animation names to names in the skeleton. Aliases and target
names must be nonempty. `fAnimations` can additionally declare animation
metadata such as `isLoop`, `isKeepOnStop`, `stepDuration`, and `turnToDuration`.
Its frame indices and frame FPS do not define skeletal animation playback.
Spine mode does not require costumes and does not render frame costumes when
both are present. Animation duration comes from the skeleton.
Ghost transparency and overall color modulation apply to Spine sprites.
Bitmap shader effects such as color and brightness do not affect Spine meshes.

Zero or omitted `defaultMix`, `stepDuration`, and `turnToDuration` use defaults
of 0.1, 0.01, and 1 seconds respectively. Negative or nonfinite timing values
are rejected. Entries declared in `fAnimations` retain their own timing, with
0.01-second step and 1-second turn defaults; the Spine timing defaults apply to
mapped entries without a corresponding `fAnimations` declaration.

## Building and checking

Spine is **disabled by default**. The public binding remains available, but
`engine.Managers().SpriteMgr.IsSpineSupported()` returns false in a default build. Loading
Spine content requires a locally built editor and runtime with `SPX_SPINE=1`.
The module bundles the Spine runtime and its Godot integration; no separate
Spine module is required.

Use a fresh, dedicated GOPATH so opt-in binaries cannot replace or be confused
with published default runtimes. `SPX_SPINE_GOPATH` explicitly acknowledges
that this path is reserved for Spine. The first use requires an empty `bin`
directory and rejects the default GOPATH. Run the following in a subshell,
pointing `GODOT_SRC` at the checkout matching the repository runtime lock:

```sh
(
  unset GOBIN
  export GOPATH="$HOME/.spx-spine"
  export SPX_SPINE_GOPATH="$GOPATH"
  export SPX_SPINE=1
  export GODOT_SRC=/absolute/path/to/godot
  make dev MODE=normal
  make run DEMO_INDEX=1
)
```

Within that same environment, subsequent focused builds are:

```sh
make build-editor
make build-desktop
make build-web MODE=normal
```

The Makefile exports the switch to buildctl, which passes `spx_spine=yes` to
editor, desktop, Web, and mounted-module SCons argument paths. Off builds pass
`spx_spine=no`; the shared default SCons profile cannot enable Spine. Every
build invokes SCons, so switching features does not reuse an unchecked ready
binary. The dedicated `bin` gets a persistent `.spx-spine` marker and Godot's
`_sc_` self-contained marker. Export templates are installed under
`$GOPATH/bin/editor_data/export_templates/<engine-version>`, not the user's
global Godot directory. Keep the dedicated GOPATH for Spine only. Default
buildctl commands reject it while Spine is disabled.

To build without Spine, leave the subshell (restoring the normal GOPATH) and use:

```sh
GODOT_SRC=/absolute/path/to/godot make dev SPX_SPINE=0 MODE=normal
```

The dedicated workspace does not download default published runtimes: do not
use `make setup`, `make setup-web`, `make download`, or `make download-engine`
there. Build the complete local stack with `make dev` first. This feature does
not introduce published Spine artifacts or change immutable runtime snapshots.
Publishing a Spine distribution needs an intentional separate runtime identity
and release workflow; never publish these binaries under a default runtime tag.
CLI runtime selection also stays within this dedicated directory: embedded and
PATH fallback runtimes are not used, and native launch bundles use its local
engine/PCK pair rather than a published cache. Missing local assets produce a
build instruction instead of downloading a default runtime.
See the [current-module build workflow](cmd_make.md) for prerequisites.

Use the same Web mode for setup, build, export, and execution. Validate both
native and Web rendering with the intended skeleton export version, including
multiple atlas pages, animation transitions, cloning, and collision bounds.

On Linux amd64, the self-authored native smoke fixture exercises the real Go
bridge and animation completion:

```sh
./testdata/spine_native_smoke/run.sh /path/to/enabled/godot enabled
./testdata/spine_native_smoke/run.sh /path/to/enabled/godot enabled export
./testdata/spine_native_smoke/run.sh /path/to/default/godot disabled
SPX_SPINE_GODOT=/path/to/enabled/godot go test ./internal/release -run TestSpineNoticeExport
```

These headless tests do not verify GPU rendering. The Web bindings have Go/WASM
and JavaScript tests, but a full Spine-enabled Web engine and browser rendering
still need platform validation before release.

The bundled Spine runtime is subject to its own license. Check the runtime's
[license terms](https://github.com/EsotericSoftware/spine-runtimes/blob/4.2/LICENSE)
and the license files under `godot_modules/spx/thirdparty/spine_godot` before
using or distributing Spine content.
Enabling this feature does not grant a Spine Editor license. Developers creating
applications containing the runtime must satisfy the applicable Editor license
requirements; a tool author's license does not cover downstream developers.
Retain both bundled notices when distributing an enabled build.
