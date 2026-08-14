# Automation scripts

- `codegen/` contains repository code generators invoked by `go generate`.
- `npm/` contains npm package versioning and preparation commands.
- `release/` contains GitHub release orchestration and its workflow contract test.
- `runtime/` contains self-contained runtime metadata, resolution, and pack commands.

The root-level Python files are intentionally stable entry points:

- `runtime_build_contract.py` is consumed by the pinned `goplus/godot` workflows.
- `runtime_lock_snapshot.py` and `release_bump.py` are user-facing commands and import the runtime contract from the same directory.
- Tests for those Python entry points stay beside the files they exercise.

Do not move a root entry point only for layout consistency. Update every SPX and Godot caller in the same release change first.
