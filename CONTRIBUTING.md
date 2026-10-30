# Contributing to SPX

Thank you for your interest in improving SPX. This guide covers the most
important points for reporting bugs, proposing changes, and contributing pull
requests.

For development details, see the [SPX development documentation](docs/en/dev/index.md),
especially the [architecture](docs/en/dev/engine/architecture.md),
[build commands](docs/en/dev/engine/cmd_make.md), and
[binding code generation](docs/en/dev/engine/code_generator.md) guides.

<!--
Automated contribution policy:

An AI agent that opens an issue or pull request for someone else must identify
its involvement. Prefix the title with 🤖 and add a notice like this to the
description:
>
> `Automation notice: An AI agent prepared this submission at the direction of [person or team] for [purpose].`
>
Maintainers may close automated submissions that omit this notice. Repeated
nondisclosure may result in contribution access being restricted.
-->

## Table of contents

- [Reporting bugs](#reporting-bugs)
- [Proposing features or improvements](#proposing-features-or-improvements)
- [Setting up a development environment](#setting-up-a-development-environment)
- [Contributing pull requests](#contributing-pull-requests)
- [Testing your changes](#testing-your-changes)
- [Working across SPX and Godot](#working-across-spx-and-godot)
- [Communicating with maintainers](#communicating-with-maintainers)

## Reporting bugs

Report bugs in the [SPX issue tracker](https://github.com/goplus/spx/issues).
Search existing issues first to avoid duplicates. Include enough information
for another contributor to reproduce the problem:

- the SPX version or commit;
- the operating system and target platform;
- the Go and XGo versions;
- the runtime path used, such as interpreted, native, editor, or Web;
- the Web mode (`normal`, `worker`, `minigame`, or `miniprogram`), when relevant;
- the expected and actual behavior, including complete error output; and
- a minimal reproduction project or a link to a small repository.

Remove generated directories, caches, binaries, and unrelated assets from a
reproduction. Confirm the issue against the latest SPX release and, when
practical, the current default branch. If the problem is a regression, state
the last known working version and the first version that fails.

Security vulnerabilities should not be filed as public issues. Use the
repository's **Security** tab to report them privately.

## Proposing features or improvements

Open an [issue](https://github.com/goplus/spx/issues/new) before investing in a
large feature or architectural change. Explain the user problem, the intended
behavior, alternatives you considered, and any compatibility impact. A small
example showing how the proposed API would be used is especially helpful.

Early discussion helps establish whether the feature belongs in the SPX Go
API, the SPX Godot module, the Godot fork, or a platform-specific bridge. It
also reduces the risk of implementing an approach that cannot be maintained
across all supported runtimes.

## Setting up a development environment

Install Git, Make, Go, and the XGo CLI. Keep `$GOPATH/bin` on `PATH`. Windows
contributors should use Git Bash and install `mingw-w64`.

Fork the repository, clone your fork, and add the canonical repository as an
upstream remote:

```sh
git clone https://github.com/YOUR_ACCOUNT/spx.git
cd spx
git remote add upstream https://github.com/goplus/spx.git
```

Prepare a development environment with the published host runtime:

```sh
make setup
make doctor
make list-demos
make run DEMO_INDEX=1
```

Run `make help` for common commands and `make help-advanced` for lower-level
commands. The root `Makefile` is the supported entry point for normal local
development.

Changes to the engine, the SPX Godot module, native bindings, or Web bridge may
require the [goplus/godot](https://github.com/goplus/godot) source tree. Point
`GODOT_SRC` at that checkout and build the smallest affected target:

```sh
GODOT_SRC=/absolute/path/to/godot make dev MODE=normal
```

Use a focused target such as `make build-editor`, `make build-desktop`, or
`make build-web MODE=normal` when a complete development build is unnecessary.

## Contributing pull requests

Keep each pull request focused on one bug or feature. Before opening it:

- rebase your branch on the current upstream branch and avoid unrelated merge
  commits;
- add or update tests for behavior changes;
- update user or developer documentation when behavior, APIs, or workflows
  change;
- regenerate checked-in output when its source declarations or templates
  change; and
- run the checks appropriate for the affected code and inspect the complete
  diff, including generated files.

In the pull request description, explain the problem, the chosen approach, and
how the change was tested. Include platform and Web mode details when relevant.
Use a GitHub closing keyword such as `Fixes #1234` when the pull request should
close an issue.

Draft pull requests are welcome for early feedback, but mark a pull request as
ready only when the implementation, tests, and documentation are complete.

### Keep commits reviewable

Each commit should leave the repository in a coherent state. Fold follow-up
typo, formatting, and build-fix commits into the commit that introduced the
problem before review is complete.

SPX uses concise, English commit subjects based on the conventional commit
style already used in the repository. Keep the subject around 72 characters
when possible and use a relevant type and optional scope, for example:

```text
fix(audio): clean up completed playback IDs
feat(runtime): add condition event trigger
docs(engine): explain Web worker synchronization
```

Use the commit body to explain why a non-obvious change is needed and note
important compatibility or design decisions.

### Document changes

Public API changes must include clear API documentation and an example where it
improves understanding. Update developer documentation for changes to the
engine boundary, build system, generated bindings, platform behavior, or
release flow.

Keep corresponding files under `docs/en` and `docs/zh` structurally aligned.
Commands, identifiers, paths, and code examples should remain equivalent in
both languages.

### Do not edit generated files directly

Files marked as generated, including files with `.gen.` in their names, must be
updated through their source declarations, generator code, or templates. Run:

```sh
make generate
```

Review every generated change. For binding changes, compile and exercise each
affected native or Web target; successful generation alone does not establish
runtime compatibility.

## Testing your changes

Add focused tests close to the code they cover. Bug fixes should include a test
that fails before the fix. New features should cover normal behavior and
expected failure cases where applicable.

For Go changes, start with the affected package and then run the repository
checks:

```sh
make format
make generate
git diff --check
go test $(go list ./... | grep -v /internal/webffi)
(cd cmd/ispx && go test ./...)
```

The `internal/webffi` package is excluded from the host test command because it
is built for Web targets. Changes to build orchestration should also run:

```sh
go test ./internal/cmd/buildctl/...
```

Compile and run the smallest representative target for runtime changes:

```sh
make run DEMO_INDEX=1
make runnative DEMO_INDEX=1
make runweb DEMO_INDEX=1
```

Choose only the commands relevant to the change, but test every affected
platform boundary. Web changes must use the same mode during setup, build,
export, and execution. Platform-specific changes that cannot be tested locally
must be called out explicitly in the pull request.

GitHub Actions runs generation checks, Go tests, cross-platform builds, and the
appropriate published-runtime or current-module integration path. All required
checks must pass before merge.

### Go coverage

CI collects atomic Go statement coverage from Linux/Web, macOS, and Windows.
Coverage is published by upstream pull requests, pushes to upstream `dev`, and
merge-group runs. Other branch and fork pushes run tests without publishing
duplicate coverage.

After all checks for the selected runtime path pass, the final CI gate collects
the Go profiles and, when module tests ran, the C++ report. It uploads them to
[Codecov](https://codecov.io/gh/goplus/spx) separately under `go` and `cpp`, then
requests checks for the combined report. Missing artifacts, upload errors, or
notification errors fail the gate. Codecov evaluates coverage asynchronously and
publishes informational `codecov/project` and `codecov/patch` results. Coverage
changes do not fail these checks, and the CI gate does not wait for them.
Job reruns can upload again; Codecov merges uploads for the same commit SHA.

Go coverage artifacts are retained for 7 days. Fixed names with overwrite enabled
allow failed-job reruns to reuse reports from successful jobs while those artifacts
remain available. After they expire, use **Re-run all jobs** to regenerate them;
rerunning only failed jobs does not recreate artifacts from successful producers.

To reproduce host coverage using the same package selection as CI:

```sh
bash .github/scripts/ci/test_go.sh -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out
go tool cover -html=coverage.out
(cd cmd/ispx && go tool cover -func=coverage.out)
```

Keep the profile path relative so the root module and `cmd/ispx` write separate
files; an absolute path would overwrite the first report. CI also records
Node/WASM Go tests and `.github/cmd` Go tools in separate profiles.

Coverage includes Linux host Go tests and
Node/WASM binding, coroutine, interpreter, and process-fallback tests, subject
to Go build tags. The nested `cmd/ispx` and `internal/cmd/codegen` modules have
separate profiles.

The existing Go build matrix also collects targeted native macOS buildctl
environment, Homebrew JDK/Brotli fixture, and CLI permission-repair coverage,
plus Windows command lookup, file replacement, tool setup, and process-lock coverage. These are not full
cross-platform test suites. Reproduce them on their respective native hosts
with `bash .github/scripts/ci/test_go_coverage.sh macos` or
`bash .github/scripts/ci/test_go_coverage.sh windows`.

Godot end-to-end, browser, C++, JavaScript, and other platform-specific paths
remain outside these Go coverage suites. Packages without tests may report
zero coverage or no instrumented statements.

Raw profiles include generated Go files. Codecov applies
[`.github/codecov.yml`](.github/codecov.yml) to exclude examples, fixtures,
generated bindings, metadata, and interpreter exports. Handwritten runtime,
CLI, build tools, and Web bindings remain in scope; Codecov totals can therefore
differ from local coverage totals.

Like [XGo](https://github.com/goplus/xgo), uploads omit a token. Codecov may
require repository or organization setup. The `dev` badge needs a successful
upload from `dev`.

### C++ module coverage

When CI selects the current-module path (`missing` or `incompatible` runtime),
the locked Godot workflow builds the Linux editor with `tests=yes`,
`use_coverage=yes`, and `optimize=none`, then runs the `*SPX*` unit tests.
The coverage build caches SCons signatures, objects, archives, and `.gcno`
metadata together. The cache requires a matching Godot commit, build settings,
toolchain, and runner environment. When only the SPX module changes, it reuses
the engine build and rebuilds the module and tests. Configure probes rerun to
keep capability detection independent of cached probe files. The editor is linked
and tests run every time; `.gcda` execution counts are never cached and are cleared
before testing. Successful compilation saves the cache before tests run, so a
later test failure does not discard the build. The first compatible build is
still a cold build.

Godot uses pinned gcovr to produce a Cobertura report for maintained native
code under `godot_modules/spx`, excluding tests, third-party code, and generated
bindings. It publishes only the report as the `coverage-spx-cpp` artifact.
The final SPX CI gate uploads that report alongside Go coverage with the `cpp`
flag. Missing or empty reports fail CI. Manually dispatching `Test Godot Module`
produces the artifact without publishing a partial Codecov report. These
artifacts are retained for one day; after expiry, rerun all integration jobs
to regenerate the report.

The `cpp` flag carries forward its latest coverage when CI reuses a
published runtime and skips module tests. Codecov labels this as historical
coverage, not a new test run. A successful Go and C++ upload is needed to
establish the initial baseline. The repository report, badge, and project check
include both languages; the patch check measures changed Go and C++ lines.
Both checks use the base commit's combined coverage as their automatic target.
The C++ report does not measure browser execution or Web-only C++ code.

## Working across SPX and Godot

SPX spans two repositories:

- [goplus/spx](https://github.com/goplus/spx) owns the Go runtime, CLI, build
  orchestration, generated bindings, and the external module under
  `godot_modules/spx`;
- [goplus/godot](https://github.com/goplus/godot) owns the coupled Godot engine
  fork and its engine-side integration.

When a feature needs changes in both repositories, open linked pull requests
and state which commit or branch was used for integration testing. Keep
`GODOT_SRC` pointed at the intended Godot checkout and do not copy the external
SPX module into the Godot tree.

Do not change runtime lock snapshots or release metadata merely to make local
testing work. Those files define published cross-repository build inputs and
should change only as part of an intentional runtime or release update.

## Communicating with maintainers

Use the [issue tracker](https://github.com/goplus/spx/issues) for reproducible
bugs, feature proposals, and discussions tied to a specific change. Use the
discussion on an existing issue or pull request when one already covers the
topic, and keep technical decisions recorded there so future contributors can
find them.

Be respectful, patient, and specific. Reviews may request changes for API
stability, cross-platform behavior, generated-code consistency, or maintenance
cost even when an implementation works on one platform.

Thanks for contributing to SPX.
