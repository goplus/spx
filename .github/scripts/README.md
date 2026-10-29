# Automation layout

| Directory | Responsibility |
| --- | --- |
| `.github/workflows/` | Workflow triggers, jobs, permissions, and artifact transport |
| `.github/actions/` | Reusable step composition in `action.yml` |
| `.github/cmd/` | Go command packages, each with its own `main.go` and tests |
| `.github/scripts/ci/` | Source checks, test runners, coverage, and shared test helpers |
| `.github/scripts/setup/` | Build tool setup and OS dependencies |
| `.github/scripts/runtime/` | Lock snapshots and runtime pack builds |
| `.github/scripts/release/` | Version bumps, release planning, assembly, and publication |
| `.github/scripts/npm/` | npm package preparation and publication |
| `.github/scripts/standalone/` | Native package preparation and verification |

Python owns release and npm orchestration, CI result policies, and bootstrap
validation. It parses JSON and prepares npm archives with the standard library;
the npm publisher owns package preparation in the same module. Shell scripts
run platform build/test tools and prepare OS environments. Go commands reuse
the repository's Go libraries through the root `go.mod`; run them as packages, for example
`go run ./.github/cmd/runtime-version --json`. There is no command dispatcher or
extra module to maintain.

Script filenames describe an operation and its subject in `snake_case`;
workflow filenames use the same convention. Action and Go command directories
use `kebab-case`. A domain directory supplies context: `release/assemble.py`,
`runtime/lock_snapshot.py`, and `setup/install_linux_dependencies.sh`.
Tests stay beside the implementation they exercise.

The public bootstrap entry point is `runtime/build_contract.py`. Godot workflows
fetch only that file and the runtime lock before checking out the module. Keep it
self-contained; its tests live beside it in `runtime/build_contract_test.py`.
Changes to this path must be coordinated with the Godot consumers.

## Workflow and action responsibilities

| Entry point | Responsibility |
| --- | --- |
| `ci.yml` | Select the runtime path, enforce the CI gate, and upload Go/C++ coverage before requesting combined checks |
| `check_source.yml` | Check generated source, syntax, contracts, and native/Web tests |
| `build_go.yml` | Build Go on each OS and test platform-specific behavior |
| `build_web.yml` / `test_godot_module.yml` | Build with a published Web runtime / test the current module against locked Godot and produce the C++ coverage artifact |
| `release.yml` | Order publication, hold the publication concurrency lock, and provide npm's top-level OIDC identity |
| `build_release.yml` | Plan, build, and assemble release artifacts; expose publication inputs only through the completion gate |
| `build_runtime_pack.yml` / `package_*.yml` | Build the runtime pack / package selected products |
| `publish_npm.yml` | Publish a verified release bundle or build and publish the requested development commit |

`setup-toolchain` installs locked Go/XGo. `setup-buildctl` builds the current
checkout and returns its executable path; callers pass that path explicitly.
`build-web-bundle` and `package-standalone` own their toolchain preparation and
product commands. Platform workflows own runner selection, OS dependencies,
and artifact transport. Actions call scripts in the owning domain; setup logic
lives in `scripts/setup/`, while product logic lives in `scripts/standalone/`
or in Go commands. Actions contain no separate script implementations.
`install-linux-dependencies` accepts only the `web` and `native` profiles used
by those workflows; retries and process timeouts remain in its setup script.

Workflow filenames and job IDs use the operation and its subject. Display names
describe the actual work. Keep `release.yml` stable for npm Trusted Publishing;
artifact names and concurrency keys are interfaces, so update every producer and
consumer together when changing them.

## Tests and coverage

- Install Python test dependencies with `python3 -m pip install -r .github/scripts/ci/requirements.txt`
  (use a virtual environment locally), then run `python3 .github/scripts/ci/test_python.py`.
  It discovers the automation's `*_test.py` scripts and runs the interpreter build
  contract, each in a separate process with a shared import environment. Pass
  individual test paths to run a subset. YAML tests parse structure; release
  behavior tests execute the same scripts as CI with fixture tools.
- Source checks run pinned actionlint and check automation shell syntax.
  `ci/check_generated.sh` generates bindings and runtime files, then rejects tracked
  changes and untracked generated outputs. It defaults to the strict release
  check. Regular CI passes `--check-export-types false` to restore the type
  snapshot after generation; existing local or staged snapshot edits are rejected
  before generation. Run it in a clean worktree.
- `ci/test_go.sh` is the local and CI entry point for native Go tests in both modules; it owns package selection and forwards Go test arguments.
- `ci/test_go_coverage.sh <suite>` runs the `web`, `web-process`, `web-interpreter`, `tools`, `codegen`, `native`, `macos`, or `windows` suite with atomic coverage. Web suites select `js/wasm` and the Go WebAssembly executor automatically. After success, the script reports the suite's profiles as newline-separated paths through the `coverage` step output when `GITHUB_OUTPUT` is set.
- `macos` and `windows` require their native OS and run targeted platform tests. macOS covers child toolchain environments, local Homebrew JDK/Brotli fixtures, and CLI permission repair; Windows covers command lookup, replacement, tool setup, and process locks. They neither install real tools nor download SDKs.
- `tools` tests every automation command with `go test ./.github/cmd/...` and
  produces one `coverage-tools.out`. Root `go test ./...` does not discover
  `.github` packages.
- `check_source.yml` owns Linux/Web/tooling suite ordering. `build_go.yml` runs the targeted native suites in its existing macOS/Windows build jobs. Both save profiles as Actions artifacts using the test runner's outputs. Release source checks keep Go coverage collection disabled; release assembly independently downloads only runtime, Web, and standalone artifact namespaces. `.github/codecov.yml` owns Go reporting exclusions; raw Go profiles remain unfiltered.
- `test_godot_module.yml` verifies the locked Godot checks and produces
  `coverage-spx-cpp`. The final `ci.yml` gate validates the selected runtime
  path, collects all required reports, uploads them under separate `go` and
  `cpp` flags, and requests combined Codecov checks. Report delivery must
  succeed. The `cpp` flag carries forward coverage when the published-runtime
  path skips module tests. See [coverage](../../CONTRIBUTING.md#go-coverage) for
  report scope, retention, and baseline requirements.

## Automation boundaries

- Workflows own scheduling, permissions, source identity, and artifact transport.
  Release scripts own input and publication policy. Runtime compatibility and
  exact SPX asset identity remain separate policies; npm must succeed before an
  SPX draft becomes public. The top-level `release.yml` retains its npm OIDC
  identity and publication concurrency group.
- Release planning validates inputs and selects platforms in one invocation,
  before runtime resolution. The npm publisher derives its version and dist-tag
  from `RELEASE_VERSION`: an empty value selects the development version policy.
  CI gates read GitHub's `needs` JSON directly, without separate result aliases.
- `build-web-bundle` uses `buildctl setup web --export-bundle` to prepare assets,
  install tools once, export the template, and export the bundle. Standalone
  `buildctl runtime export-web` still installs its tools. Both paths share the
  same export implementation and keep explicit mode/asset-source inputs.
- `package-standalone` owns prepare/package/verify/upload; its artifact name
  determines the ZIP filename. Platform workflows retain runner selection,
  dependencies, download sources, and their existing matrix failure boundaries.
  Smoke tests use a fresh GOPATH and executable directory while retaining Go
  compiler caches. ZIP metadata is normalized; this does not promise that an
  entire native build is reproducible.
- Assembly validates the complete expected asset set inside the selected release
  namespaces. Diagnostic and coverage artifacts stay outside that input set;
  missing, duplicate, or extra release files still fail validation.
  `runtime-manifest --revision HEAD` derives provenance from one resolved commit;
  Python supplies the asset set without calculating or relaying Git fingerprints.
- Changes to runtime inputs or their fingerprint selectors can invalidate
  published runtime reuse. In particular, export dispatch is part of pack-source
  identity, and moving the buildctl setup script changes build-recipe identity.
  Follow the version migration policy in the release guide rather than
  weakening fingerprints or replacing published assets. Cross-repository export
  smoke and candidate artifact promotion remain separate changes.
