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

## Tests and coverage

- `test_go.sh` is the local and CI entry point for native Go tests in both modules; it owns package selection and forwards Go test arguments.
- `test_go_coverage.sh <suite>` runs the `web`, `web-process`, `web-interpreter`, `tools`, `codegen`, `native`, `macos`, or `windows` suite with atomic coverage. Web suites select `js/wasm` and the Go WebAssembly executor automatically. After success, the script reports the suite's profiles as newline-separated paths through the `coverage` step output when `GITHUB_OUTPUT` is set.
- `macos` and `windows` require their native OS and run only targeted buildctl tests, not all Go packages. macOS covers child toolchain environments and local Homebrew JDK/Brotli fixtures; Windows covers command lookup, replacement, tool setup, and process locks. They neither install real tools nor download SDKs.
- `static_checks.yml` owns Linux/Web/tooling suite ordering. `go_builds.yml` runs the targeted native suites in its existing macOS/Windows build jobs. Both save profiles as Actions artifacts using the test runner's outputs. Once both workflows succeed, `runner.yml` downloads all three required artifacts and attempts one complete Codecov upload per execution of its coverage job. Release checks leave artifact collection disabled so coverage files cannot enter release assembly. `.github/codecov.yml` owns reporting exclusions; raw profiles remain unfiltered. See [Go coverage](../../CONTRIBUTING.md#go-coverage) for upload failure and artifact retention behavior.
