# Spine runtime

Spine 4.2 C++ runtime and Godot integration from Esoteric Software's
[spine-runtimes](https://github.com/EsotericSoftware/spine-runtimes) project.
This directory is licensed under the included **Spine Runtimes License**, not
SPX's license. Preserve the license and copyright notices when redistributing.
See [Spine licensing](https://esotericsoftware.com/spine-editor-license) for the
applicable Spine Editor license requirements.

The SPX module compiles and registers this dependency privately only when
`spx_spine=yes` is set; it is disabled by default. SPX-specific changes support
direct project assets and validate skeleton version headers.

The JSON parser in `spine-cpp/src/spine/Json.cpp` has a separate notice in
`LICENSE-Json`. Retain both notices when distributing binaries.
