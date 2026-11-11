# Godot custom modules

SPX-owned Godot modules live here instead of in the upstream Godot checkout.
The default engine build loads the `spx` module itself with Godot's
`custom_modules` option, so sibling modules are not selected accidentally.
`SPX_MODULE_SRC` may point at another `spx` module source directory; relative
overrides are resolved from the SPX repository root.

`spx/spx_scons_profile.json` is the shared source of truth for static SCons
arguments used by local engine builds, legacy Docker builds, and Godot release
workflows. The versioned profile contains ordered `key=value` arrays for common,
editor, and template release settings. Keep target, platform, architecture, Web
threading, and the `custom_modules` path in the caller: the module path must
remain one argument even when it contains spaces. The common profile disables
recursive custom-module discovery because `custom_modules` points directly at
`spx`.

The `spx` module, its private third-party dependencies, Web bridge, recorder,
and module tests are stored under `godot_modules/spx`. Keep generated bindings
in that module and run `make generate` after changing manager declarations.

The Spine 4.2 runtime is disabled by default. With `spx_spine=yes`, it is
compiled as a private dependency of `spx` from
`spx/thirdparty/spine_godot`; no sibling custom module or Godot checkout patch
is required. Its separate Spine Runtimes License and redistribution notice are
included in that directory. See the [Spine integration guide](../docs/en/dev/engine/spine.md)
for asset formats and licensing requirements.
